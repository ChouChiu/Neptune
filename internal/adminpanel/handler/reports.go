package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/kazumi-group/neptune/internal/adminpanel/components"
	"github.com/kazumi-group/neptune/internal/db"
)

// ListReports returns an HTMX handler that renders the reports list as HTML.
func ListReports(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		statusStr := r.URL.Query().Get("status")
		var status *string
		if statusStr != "" {
			status = &statusStr
		}

		reports, err := database.GetReports(status, &userID)
		if err != nil {
			http.Error(w, "Failed to load reports", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		components.ReportCards(reports).Render(r.Context(), w)
	}
}

// ResolveReport handles approving or dismissing a report.
func ResolveReport(database *db.DB, botToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		reportIDStr := strings.TrimSpace(r.PathValue("reportID"))
		reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
		if err != nil {
			writeErrJSON(w, http.StatusBadRequest, "Invalid report ID")
			return
		}

		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErrJSON(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if body.Status != "approved" && body.Status != "dismissed" {
			writeErrJSON(w, http.StatusBadRequest, "Status must be 'approved' or 'dismissed'")
			return
		}

		report, err := database.GetReport(reportID, &userID)
		if err != nil || report == nil {
			writeErrJSON(w, http.StatusNotFound, "Report not found")
			return
		}

		if err := database.UpdateReportStatus(reportID, body.Status, userID); err != nil {
			writeErrJSON(w, http.StatusInternalServerError, "Failed to update report")
			return
		}

		// Side effects: delete message, add warning, send notifications
		if body.Status == "approved" {
			handleApproved(database, botToken, report)
		} else {
			handleDismissed(botToken, report)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

func writeErrJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
