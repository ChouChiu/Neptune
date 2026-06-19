package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/model"
)

// getUserID extracts the admin user ID from the request context.
// Context key "admin_user_id" is set by adminpanel.SessionAuthMiddleware.
func getUserID(r *http.Request) int64 {
	if v, ok := r.Context().Value("admin_user_id").(int64); ok {
		return v
	}
	return 0
}

// handleApproved processes an approved report: deletes the message, adds a warning,
// and sends Telegram notifications.
func handleApproved(database *db.DB, botToken string, report *model.Report) {
	ctx := context.Background()

	if report.ReportedMessageID != nil {
		tgAPICall(ctx, botToken, "deleteMessage", map[string]any{
			"chat_id":    report.GroupID,
			"message_id": *report.ReportedMessageID,
		})
	}

	_ = database.AddWarning(report.GroupID, report.ReportedUserID, 0, fmt.Sprintf("举报通过: %s", report.Content))

	preview := ""
	if report.ReportedMessageText != "" {
		preview = fmt.Sprintf("\n📄 被举报内容：「%s」", truncate(report.ReportedMessageText, 100))
	}

	groupMsg := fmt.Sprintf(
		"⚠️ 用户 [%d](tg://user?id=%d) 被举报处理\n\n📝 举报原因：%s%s\n\n🚫 违规消息已删除，用户已被警告。请遵守群规。",
		report.ReportedUserID, report.ReportedUserID, report.Content, preview,
	)
	tgAPICall(ctx, botToken, "sendMessage", map[string]any{
		"chat_id":    report.GroupID,
		"text":       groupMsg,
		"parse_mode": "Markdown",
	})

	reporterMsg := fmt.Sprintf(
		"✅ 你的举报已通过处理\n\n📋 举报编号：#%d\n📝 举报原因：%s\n🏠 群组：%d\n\n违规消息已删除，用户已被警告。感谢你维护群组秩序！",
		report.ReportedUserID, report.Content, report.GroupID,
	)
	tgAPICall(ctx, botToken, "sendMessage", map[string]any{
		"chat_id": report.ReporterID,
		"text":    reporterMsg,
	})
}

// handleDismissed sends a dismissal notification to the reporter.
func handleDismissed(botToken string, report *model.Report) {
	ctx := context.Background()

	reporterMsg := fmt.Sprintf(
		"❌ 你的举报未通过审核\n\n📋 举报编号：#%d\n📝 举报原因：%s\n🏠 群组：%d\n\n经管理员审核，该举报不符合处理条件。如有疑问请联系群管理员。",
		report.ReportedUserID, report.Content, report.GroupID,
	)
	tgAPICall(ctx, botToken, "sendMessage", map[string]any{
		"chat_id": report.ReporterID,
		"text":    reporterMsg,
	})
}

// tgAPICall calls the Telegram Bot API. Errors are logged but not returned.
func tgAPICall(ctx context.Context, botToken, method string, body map[string]any) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", botToken, method)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		slog.Error("tgAPICall marshal error", "method", method, "error", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonBody)))
	if err != nil {
		slog.Error("tgAPICall request error", "method", method, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("tgAPICall error", "method", method, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("tgAPICall failed", "method", method, "status", resp.StatusCode)
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
