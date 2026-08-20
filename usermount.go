package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"usermount/views"
)

//go:embed public
var staticAssets embed.FS

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		slog.Info("HTTP Request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", time.Since(start),
		)
	})
}

func checkAdminExists(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/livez" || r.URL.Path == "/setup-admin" || r.URL.Path == "/api/setup-admin" || strings.HasPrefix(r.URL.Path, "/css/") || strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/favicon.ico" {
			next.ServeHTTP(w, r)
			return
		}

		adminExists, err := hasAdmin()
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if !adminExists {
			http.Redirect(w, r, "/setup-admin", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func normalizeUsername(s string) string {
	s = strings.ToLower(s)
	var res []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			res = append(res, r)
		}
	}
	return string(res)
}

func main() {
	configPath := flag.String("config-path", "", "Path to the configuration YAML file (required)")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("CRITICAL: --config-path parameter is required. Please specify the path to your config file.")
	}

	loadConfig(*configPath)

	if err := initDB(); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()

	publicFS, err := fs.Sub(staticAssets, "public")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /css/", http.FileServer(http.FS(publicFS)))
	mux.Handle("GET /assets/", http.FileServer(http.FS(publicFS)))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/assets/favicon.ico"
		http.FileServer(http.FS(publicFS)).ServeHTTP(w, r)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	authLimiter := NewIPRateLimiter(0.2, 5)    // 5 burst, 1 token every 5s
	inviteLimiter := NewIPRateLimiter(0.5, 10) // 10 burst, 1 token every 2s

	// First Launch Route
	mux.HandleFunc("GET /setup-admin", func(w http.ResponseWriter, r *http.Request) {
		adminExists, err := hasAdmin()
		if err == nil && adminExists {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		views.SetupAdmin("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/setup-admin", RateLimitMiddleware(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		adminExists, err := hasAdmin()
		if err == nil && adminExists {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if len(password) < 8 {
			views.SetupAdmin("Password must be at least 8 characters").Render(r.Context(), w)
			return
		}

		hash, err := HashPassword(password)
		if err != nil {
			views.SetupAdmin("Server error").Render(r.Context(), w)
			return
		}

		created, err := createInitialAdmin(username, hash)
		if err != nil || !created {
			views.SetupAdmin("Failed to create admin").Render(r.Context(), w)
			return
		}

		http.Redirect(w, r, "/login", http.StatusFound)
	}))

	getServicesViewData := func() []views.ServiceItem {
		var list []views.ServiceItem
		for key, s := range AppConfig.Services {
			icon := s.Icon
			if icon == "" {
				switch strings.ToLower(key) {
				case "mail", "mailbox", "email", "webmail":
					icon = "mail"
				case "git", "forgejo", "gitea", "github", "gitlab":
					icon = "git-branch"
				case "server", "vps", "ssh":
					icon = "server"
				default:
					icon = "globe"
				}
			}
			name := s.Name
			if name == "" {
				if len(key) > 0 {
					name = strings.ToUpper(key[:1]) + key[1:]
				} else {
					name = "Service"
				}
			}
			list = append(list, views.ServiceItem{
				Key:  key,
				Name: name,
				Goto: s.Goto,
				Icon: icon,
			})
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].Name < list[j].Name
		})
		return list
	}

	// Routes
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		accessTokenCookie, err := r.Cookie("access_token")
		if err == nil {
			claims, err := ValidateToken(accessTokenCookie.Value)
			if err == nil {
				if claims.Role == "admin" {
					http.Redirect(w, r, "/admin", http.StatusFound)
					return
				}
				services := getServicesViewData()
				views.Home(claims.Username, services).Render(r.Context(), w)
				return
			}
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		views.Login("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/login", RateLimitMiddleware(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := getUser(username)
		if err != nil || user == nil {
			views.Login("Invalid credentials").Render(r.Context(), w)
			return
		}

		ok, err := VerifyPassword(password, user.PasswordHash)
		if err != nil || !ok {
			views.Login("Invalid credentials").Render(r.Context(), w)
			return
		}

		accessToken, refreshToken, err := GenerateTokens(user.Username, user.Role)
		if err != nil {
			views.Login("Server error").Render(r.Context(), w)
			return
		}

		SetAuthCookies(w, accessToken, refreshToken)
		if user.Role == "admin" {
			http.Redirect(w, r, "/admin", http.StatusFound)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
	}))

	mux.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		ClearAuthCookies(w)
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	// Helper function to build the user rows view data
	getUsersViewData := func() []views.UserRow {
		dbUsers, err := listUsers()
		if err != nil {
			return nil
		}
		var rows []views.UserRow
		for _, u := range dbUsers {
			rows = append(rows, views.UserRow{
				Username:  u.Username,
				Role:      u.Role,
				CreatedAt: u.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		return rows
	}

	getInvitesViewData := func() []views.InviteRow {
		dbInvites, err := listInvites()
		if err != nil {
			return nil
		}
		var rows []views.InviteRow
		for _, inv := range dbInvites {
			isExpired := !inv.Used && time.Since(inv.CreatedAt) > 10*time.Minute
			rows = append(rows, views.InviteRow{
				ID:        inv.ID,
				Code:      inv.Code,
				Email:     inv.Email,
				Used:      inv.Used,
				CreatedAt: inv.CreatedAt.Format("2006-01-02 15:04"),
				IsExpired: isExpired,
			})
		}
		return rows
	}

	// Admin routes
	mux.HandleFunc("GET /admin", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(userContextKey).(*Claims)
		userRows := getUsersViewData()
		inviteRows := getInvitesViewData()
		views.AdminDashboard(claims.Username, claims.Role, userRows, inviteRows).Render(r.Context(), w)
	}))

	mux.HandleFunc("GET /api/admin/users-list", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(userContextKey).(*Claims)
		rows := getUsersViewData()
		views.UserList(rows, claims.Username).Render(r.Context(), w)
	}))

	mux.HandleFunc("POST /api/admin/users/delete", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(userContextKey).(*Claims)
		targetUsername := r.URL.Query().Get("username")
		if targetUsername == "" || targetUsername == claims.Username {
			http.Error(w, "Invalid user deletion request", http.StatusBadRequest)
			return
		}

		targetUser, err := getUser(targetUsername)
		if err != nil || targetUser == nil || targetUser.Role == "admin" {
			http.Error(w, "Cannot delete admin or nonexistent user", http.StatusBadRequest)
			return
		}

		if err := deleteUser(targetUsername); err != nil {
			slog.Error("Failed to delete user in DB", "err", err)
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}

		if err := deleteUserSystem(targetUsername); err != nil {
			slog.Error("Failed to execute user teardown script", "err", err)
		}

		w.Header().Set("HX-Trigger", "user-deleted")
		rows := getUsersViewData()
		views.UserList(rows, claims.Username).Render(r.Context(), w)
	}))

	mux.HandleFunc("GET /api/admin/invites-list", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		rows := getInvitesViewData()
		views.InviteList(rows).Render(r.Context(), w)
	}))

	mux.HandleFunc("POST /api/admin/invite/revoke", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			_ = revokeInvite(code)
		}
		rows := getInvitesViewData()
		views.InviteList(rows).Render(r.Context(), w)
	}))

	mux.HandleFunc("POST /api/admin/invite", RateLimitMiddleware(inviteLimiter, RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		email := r.FormValue("email")
		code, err := createInvite(email)
		if err != nil {
			views.InviteError("Failed to generate invite").Render(r.Context(), w)
			return
		}

		err = sendActivationEmail(email, code)
		if err != nil {
			slog.Error("Email failed", "err", err)
			views.InviteError("Invite created, but email failed. Code: "+code).Render(r.Context(), w)
			return
		}

		w.Header().Set("HX-Trigger", "invite-sent")
		views.InviteSuccess().Render(r.Context(), w)
	})))

	// User self-service routes
	mux.HandleFunc("POST /api/user/change-password", RateLimitMiddleware(authLimiter, AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(userContextKey).(*Claims)
		currentPassword := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")

		if len(newPassword) < 8 {
			views.PasswordChangeError("New password must be at least 8 characters").Render(r.Context(), w)
			return
		}

		user, err := getUser(claims.Username)
		if err != nil || user == nil {
			views.PasswordChangeError("User not found").Render(r.Context(), w)
			return
		}

		ok, err := VerifyPassword(currentPassword, user.PasswordHash)
		if err != nil || !ok {
			views.PasswordChangeError("Current password is incorrect").Render(r.Context(), w)
			return
		}

		newHash, err := HashPassword(newPassword)
		if err != nil {
			views.PasswordChangeError("Server error hashing password").Render(r.Context(), w)
			return
		}

		if err := updateUserPassword(claims.Username, newHash); err != nil {
			views.PasswordChangeError("Failed to update password in database").Render(r.Context(), w)
			return
		}

		// Execute update password script
		if err := updateUserPasswordSystem(claims.Username, newPassword); err != nil {
			slog.Error("Password update script failed", "err", err)
			if rbErr := updateUserPassword(claims.Username, user.PasswordHash); rbErr != nil {
				slog.Error("Failed to rollback password in DB", "err", rbErr)
			}
			views.PasswordChangeError("Failed to update system password: "+err.Error()).Render(r.Context(), w)
			return
		}

		views.PasswordChangeSuccess().Render(r.Context(), w)
	})))

	// Activation routes
	mux.HandleFunc("GET /activate", func(w http.ResponseWriter, r *http.Request) {
		views.Activate("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/activate", RateLimitMiddleware(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		invite, err := getInviteByCode(code)
		if err != nil || invite == nil || invite.Used {
			views.Activate("Invalid, expired, or already used activation code").Render(r.Context(), w)
			return
		}
		http.Redirect(w, r, "/setup?code="+code, http.StatusFound)
	}))

	mux.HandleFunc("GET /setup", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		invite, err := getInviteByCode(code)
		if err != nil || invite == nil || invite.Used {
			http.Redirect(w, r, "/activate", http.StatusFound)
			return
		}
		views.Setup(code, "").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/setup", RateLimitMiddleware(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		username := r.FormValue("username")
		password := r.FormValue("password")

		invite, err := getInviteByCode(code)
		if err != nil || invite == nil || invite.Used {
			views.Setup(code, "Invalid or expired code").Render(r.Context(), w)
			return
		}

		if len(password) < 8 {
			views.Setup(code, "Password must be at least 8 characters").Render(r.Context(), w)
			return
		}

		normalizedUser := normalizeUsername(username)
		if normalizedUser == "" {
			views.Setup(code, "Invalid username characters").Render(r.Context(), w)
			return
		}

		// Check if username is already taken
		existingUser, err := getUser(normalizedUser)
		if err != nil {
			views.Setup(code, "Server error").Render(r.Context(), w)
			return
		}
		if existingUser != nil {
			views.Setup(code, "Username already taken").Render(r.Context(), w)
			return
		}

		// Atomically mark invite as used first to prevent race conditions (TOCTOU)
		consumed, err := markInviteAsUsed(code)
		if err != nil || !consumed {
			views.Setup(code, "Invalid or already used code").Render(r.Context(), w)
			return
		}

		// Hash password
		hash, err := HashPassword(password)
		if err != nil {
			views.Setup(code, "Server error").Render(r.Context(), w)
			return
		}

		// Create user in DB with normalized username
		err = createUserDb(normalizedUser, hash, "user")
		if err != nil {
			views.Setup(code, "Failed to register user").Render(r.Context(), w)
			return
		}

		// Execute bash script with normalized username
		err = createUser(normalizedUser, password)
		if err != nil {
			slog.Error("Script error", "err", err)
			if delErr := deleteUser(normalizedUser); delErr != nil {
				slog.Error("Failed to rollback user creation in DB", "err", delErr)
			}
			if invErr := markInviteAsUnused(code); invErr != nil {
				slog.Error("Failed to rollback invite status in DB", "err", invErr)
			}
			views.Setup(code, "Failed to create system user: "+err.Error()).Render(r.Context(), w)
			return
		}

		accessToken, refreshToken, _ := GenerateTokens(normalizedUser, "user")
		SetAuthCookies(w, accessToken, refreshToken)

		http.Redirect(w, r, "/", http.StatusFound)
	}))

	handler := checkAdminExists(mux)
	handler = loggingMiddleware(handler)

	addr := AppConfig.Port
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("Server is starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Server is shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	} else {
		slog.Info("Server exited cleanly")
	}
}
