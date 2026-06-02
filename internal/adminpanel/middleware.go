package adminpanel

import (
	"context"
	"net/http"
)

type contextKey string

const userIDKey contextKey = "admin_user_id"

// SessionAuthMiddleware reads the nep_session cookie, verifies it, and injects
// the user ID into the request context. If the cookie is missing or invalid,
// it responds with 401 Unauthorized.
func SessionAuthMiddleware(botToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("nep_session")
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			userID := VerifySession(botToken, cookie.Value)
			if userID == 0 {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "nep_session",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				})
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
