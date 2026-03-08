package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

func RequireUser(next http.Handler) http.Handler {
	// TODO: replace with real authentication
	const hardcodedUser = "00000000-0000-0000-0000-000000000001"
	id := uuid.MustParse(hardcodedUser)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &StatusResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
		next.ServeHTTP(writer, r)
		slog.Info(
			"completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", writer.StatusCode,
		)
	})
}

type StatusResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func (w *StatusResponseWriter) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
