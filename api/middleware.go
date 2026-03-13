package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jochemloedeman/misty/users"
)

const (
	wwwAuthMissingHeader = `Bearer error="invalid_request", error_description="missing authorization header"`
	wwwAuthInvalidScheme = `Bearer error="invalid_request", error_description="authorization header must use Bearer scheme"`
	wwwAuthExpiredToken  = `Bearer error="invalid_token", error_description="token has expired"`
	wwwAuthInvalidToken  = `Bearer error="invalid_token", error_description="token is invalid"`
)

func RequireUser(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized,
					withHeader("WWW-Authenticate", wwwAuthMissingHeader),
					withMessage("missing authorization header"),
				)
				return
			}
			token, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok {
				writeError(w, http.StatusUnauthorized,
					withHeader("WWW-Authenticate", wwwAuthInvalidScheme),
					withMessage("authorization header must use Bearer scheme"),
				)
				return
			}

			claims, err := verifier.Verify(token)
			if err != nil {
				if errors.Is(err, users.ErrExpiredToken) {
					writeError(w, http.StatusUnauthorized,
						withHeader("WWW-Authenticate", wwwAuthExpiredToken),
						withMessage("token has expired"),
					)
				} else {
					writeError(w, http.StatusUnauthorized,
						withHeader("WWW-Authenticate", wwwAuthInvalidToken),
						withMessage("invalid token"),
					)
				}
				return
			}
			newRequest := r.WithContext(context.WithValue(r.Context(), userIDKey, claims.UserID))
			next.ServeHTTP(w, newRequest)
		})
	}
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &StatusResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
		next.ServeHTTP(writer, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", writer.StatusCode,
			"duration", time.Since(start),
		}
		switch {
		case writer.StatusCode >= 500:
			slog.Error("completed", attrs...)
		case writer.StatusCode >= 400:
			slog.Warn("completed", attrs...)
		default:
			slog.Info("completed", attrs...)
		}
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
