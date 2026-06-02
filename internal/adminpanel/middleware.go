package adminpanel

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const userIDKey contextKey = "admin_user_id"

// SessionAuthMiddleware reads the nep_session cookie or Authorization header,
// verifies it, and injects the user ID into the request context.
func SessionAuthMiddleware(botToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Authorization header first, then cookie
			var sessionValue string
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				sessionValue = strings.TrimPrefix(auth, "Bearer ")
			}
			if sessionValue == "" {
				if cookie, err := r.Cookie("nep_session"); err == nil && cookie.Value != "" {
					sessionValue = cookie.Value
				}
			}

			if sessionValue == "" {
				slog.Warn("SessionAuth: no session", "path", r.URL.Path)
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			userID := VerifySession(botToken, sessionValue)
			if userID == 0 {
				slog.Warn("SessionAuth: invalid session", "path", r.URL.Path)
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the admin user ID from the request context.
func GetUserID(r *http.Request) int64 {
	if v, ok := r.Context().Value(userIDKey).(int64); ok {
		return v
	}
	return 0
}
