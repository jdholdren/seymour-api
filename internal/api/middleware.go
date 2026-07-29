package api

import (
	"log/slog"
	"net/http"
	"time"
)

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request received", "method", r.Method, "path", r.URL.Path)
		start := time.Now()

		writer := &respCodeWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)

		slog.Info("request completed",
			"method", r.Method,
			"url", r.URL.String(),
			"duration", time.Since(start),
			"status_code", writer.code,
		)
	})
}

// To trap the response status code for logging later.
type respCodeWriter struct {
	http.ResponseWriter
	code int
}

func (w *respCodeWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}
