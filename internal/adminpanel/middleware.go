package adminpanel

import (
	"net/http"
	"strings"

	"github.com/ChouChiu/neptune/internal/adminpanel/sessionctx"
)

// SessionAuthMiddleware reads the nep_session cookie, Authorization header,
// verifies it, and injects the user ID into the request context.
func SessionAuthMiddleware(botToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Authorization header first, then cookie.
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
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			userID := VerifySession(botToken, sessionValue)
			if userID == 0 {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(sessionctx.WithUserID(r.Context(), userID)))
		})
	}
}

// GetUserID extracts the admin user ID from the request context.
func GetUserID(r *http.Request) int64 {
	return sessionctx.UserID(r)
}
