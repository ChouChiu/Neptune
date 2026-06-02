package adminpanel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/kazumi-group/neptune/internal/adminpanel/components"
	"github.com/kazumi-group/neptune/internal/adminpanel/handler"
	"github.com/kazumi-group/neptune/internal/db"
)

// NewServer creates a Chi router with all admin panel routes registered.
func NewServer(database *db.DB, botToken, botUsername string) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))

	// Serve the admin SPA page
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		components.Layout(botUsername).Render(r.Context(), w)
	})

	// Auth endpoints (no session required)
	r.Post("/auth/login", handleLogin(database, botToken))
	r.Get("/auth/me", handleMe(botToken))

	// API endpoints (session required)
	r.Group(func(r chi.Router) {
		r.Use(SessionAuthMiddleware(botToken))
		r.Get("/api/reports", handler.ListReports(database))
		r.Post("/api/reports/{reportID}/resolve", handler.ResolveReport(database, botToken))
		r.Get("/api/warnings", handler.ListWarnings(database))
	})

	return r
}

// handleLogin processes Telegram Login Widget authentication.
func handleLogin(database *db.DB, botToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid request body"})
			return
		}

		// Normalize all values to strings (Telegram sends id as number)
		data := make(map[string]string, len(raw))
		for k, v := range raw {
			switch val := v.(type) {
			case json.Number:
				data[k] = val.String()
			case string:
				data[k] = val
			default:
				data[k] = fmt.Sprintf("%v", val)
			}
		}

		if !VerifyTelegramAuth(botToken, data) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "Invalid auth data"})
			return
		}

		idStr, ok := data["id"]
		if !ok {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "Missing user ID"})
			return
		}

		userID, err := parseID(idStr)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "Invalid user ID"})
			return
		}

		// Check if user is admin of any group
		groups, err := database.GetAdminGroups(userID)
		if err != nil || len(groups) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "You are not an admin"})
			return
		}

		expiresAt := time.Now().Unix() + sessionTTL
		session := SignSession(botToken, userID, expiresAt)

		user := map[string]any{
			"id":         userID,
			"first_name": data["first_name"],
			"last_name":  data["last_name"],
			"username":   data["username"],
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "nep_session",
			Value:    session,
			Path:     "/",
			MaxAge:   sessionTTL,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": user, "token": session})
	}
}

// handleMe returns the current authenticated user info.
func handleMe(botToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("nep_session")
		if err != nil || cookie.Value == "" {
			writeJSON(w, http.StatusOK, map[string]any{"user": nil})
			return
		}

		userID := VerifySession(botToken, cookie.Value)
		if userID == 0 {
		http.SetCookie(w, &http.Cookie{
			Name:     "nep_session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
			writeJSON(w, http.StatusOK, map[string]any{"user": nil})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"id":         userID,
				"first_name": "Admin",
			},
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to write JSON response", "error", err)
	}
}

func parseID(s string) (int64, error) {
	var id int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, ErrInvalidID
		}
		id = id*10 + int64(c-'0')
	}
	return id, nil
}

// ErrInvalidID is returned when a user ID string is not a valid number.
var ErrInvalidID = errInvalidID{}

type errInvalidID struct{}

func (errInvalidID) Error() string { return "invalid ID" }
