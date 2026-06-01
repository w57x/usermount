package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
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

func main() {
	mux := http.NewServeMux()

	publicFS, err := fs.Sub(staticAssets, "public")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(publicFS))

	mux.Handle("GET /css/", fileServer)

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		component := views.Home("Usermount")
		err := component.Render(r.Context(), w)
		if err != nil {
			slog.Error("Failed to render component", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	muxWrapped := loggingMiddleware(mux)

	server := &http.Server{
		Addr:         ":3000",
		Handler:      muxWrapped,
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
