package adminpanel

import (
	"context"
	"net/http"
	"strings"
)

// Context key used to store admin user ID. Must be a plain string
// (not a custom type) so that handler package can read it with the same key.
const userIDContextKey = "admin_user_id"

// SessionAuthMiddleware reads the nep_session cookie, Authorization header,
// or token query parameter, verifies it, and injects the user ID into the request context.
func SessionAuthMiddleware(botToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Authorization header first, then query param, then cookie
			var sessionValue string
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				sessionValue = strings.TrimPrefix(auth, "Bearer ")
			}
			if sessionValue == "" {
				sessionValue = r.URL.Query().Get("token")
			}
			if sessionValue == "" {
				if cookie, err := r.Cookie("nep_session"); err == nil && cookie.Value != "" {
					sessionValue = cookie.Value
				}
			}

			if sessionValue == "" {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			userID := VerifySession(botToken, sessionValue)
			if userID == 0 {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the admin user ID from the request context.
func GetUserID(r *http.Request) int64 {
	if v, ok := r.Context().Value(userIDContextKey).(int64); ok {
		return v
	}
	return 0
}
