package adminpanel

import (
	"net/http"

	"github.com/ChouChiu/neptune/internal/adminpanel/sessionctx"
)

func extractSessionToken(r *http.Request) string {
	if cookie, err := r.Cookie("nep_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

// SessionAuthMiddleware reads the nep_session cookie,
// verifies it, and injects the user ID into the request context.
func SessionAuthMiddleware(botToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionValue := extractSessionToken(r)
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
