package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"strings"
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
		if r.URL.Path == "/setup-admin" || r.URL.Path == "/api/setup-admin" || strings.HasPrefix(r.URL.Path, "/css/") {
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

	// First Launch Route
	mux.HandleFunc("GET /setup-admin", func(w http.ResponseWriter, r *http.Request) {
		adminExists, err := hasAdmin()
		if err == nil && adminExists {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		views.SetupAdmin("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/setup-admin", func(w http.ResponseWriter, r *http.Request) {
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

		err = createUserDb(username, hash, "admin")
		if err != nil {
			views.SetupAdmin("Failed to create admin").Render(r.Context(), w)
			return
		}

		http.Redirect(w, r, "/login", http.StatusFound)
	})

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
				views.Home(claims.Username).Render(r.Context(), w)
				return
			}
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		views.Login("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
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
	})

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

	// Admin routes
	mux.HandleFunc("GET /admin", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(userContextKey).(*Claims)
		rows := getUsersViewData()
		views.AdminDashboard(claims.Username, claims.Role, rows).Render(r.Context(), w)
	}))

	mux.HandleFunc("GET /api/admin/users-list", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		rows := getUsersViewData()
		views.UserList(rows).Render(r.Context(), w)
	}))

	mux.HandleFunc("POST /api/admin/invite", RequireRole("admin", func(w http.ResponseWriter, r *http.Request) {
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
		views.InviteSuccess(code).Render(r.Context(), w)
	}))

	// Activation routes
	mux.HandleFunc("GET /activate", func(w http.ResponseWriter, r *http.Request) {
		views.Activate("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/activate", func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		invite, err := getInviteByCode(code)
		if err != nil || invite == nil || invite.Used {
			views.Activate("Invalid, expired, or already used activation code").Render(r.Context(), w)
			return
		}
		http.Redirect(w, r, "/setup?code="+code, http.StatusFound)
	})

	mux.HandleFunc("GET /setup", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		invite, err := getInviteByCode(code)
		if err != nil || invite == nil || invite.Used {
			http.Redirect(w, r, "/activate", http.StatusFound)
			return
		}
		views.Setup(code, "").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /api/setup", func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		username := r.FormValue("username")
		password := r.FormValue("password")

		invite, err := getInviteByCode(code)
		if err != nil || invite == nil || invite.Used {
			views.Setup(code, "Invalid or expired code").Render(r.Context(), w)
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
		}

		accessToken, refreshToken, _ := GenerateTokens(normalizedUser, "user")
		SetAuthCookies(w, accessToken, refreshToken)

		http.Redirect(w, r, "/", http.StatusFound)
	})

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

	slog.Info("Server is starting", "addr", server.Addr)

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed to start", "error", err)
	}
}
