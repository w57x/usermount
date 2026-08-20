package main

import (
	"context"
	"net/http"
	"time"
)

type contextKey string

const userContextKey = contextKey("user")

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accessTokenCookie, err := r.Cookie("access_token")
		if err == nil {
			claims, err := ValidateToken(accessTokenCookie.Value)
			if err == nil {
				// Verify user still exists in the database and their role hasn't changed
				user, dbErr := getUser(claims.Username)
				if dbErr == nil && user != nil && user.Role == claims.Role {
					ctx := context.WithValue(r.Context(), userContextKey, claims)
					next(w, r.WithContext(ctx))
					return
				}
			}
		}

		// Try refresh token
		refreshTokenCookie, err := r.Cookie("refresh_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		claims, err := ValidateToken(refreshTokenCookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Verify user still exists and hasn't changed roles before issuing new tokens
		user, dbErr := getUser(claims.Username)
		if dbErr != nil || user == nil || user.Role != claims.Role {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Generate new tokens
		newAccessToken, newRefreshToken, err := GenerateTokens(claims.Username, claims.Role)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		SetAuthCookies(w, newAccessToken, newRefreshToken)

		ctx := context.WithValue(r.Context(), userContextKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(userContextKey).(*Claims)
		if !ok || claims.Role != role {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func SetAuthCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	secure := AppConfig.IsCookieSecure()
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Expires:  time.Now().Add(15 * time.Minute),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func ClearAuthCookies(w http.ResponseWriter) {
	secure := AppConfig.IsCookieSecure()
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}
