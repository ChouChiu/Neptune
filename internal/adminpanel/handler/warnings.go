package handler

import (
	"net/http"

	"github.com/ChouChiu/neptune/internal/adminpanel/components"
	"github.com/ChouChiu/neptune/internal/db"
)

// ListWarnings returns an HTMX handler that renders the warnings list as HTML.
func ListWarnings(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		warnings, err := database.GetAllWarnings(&userID)
		if err != nil {
			http.Error(w, "Failed to load warnings", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		components.WarningRows(warnings).Render(r.Context(), w)
	}
}
