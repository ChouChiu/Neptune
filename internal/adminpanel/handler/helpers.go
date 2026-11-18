package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ChouChiu/neptune/internal/adminpanel/sessionctx"
	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/model"
	"github.com/ChouChiu/neptune/internal/util"
)

var telegramHTTPClient = &http.Client{Timeout: 10 * time.Second}

// getUserID extracts the admin user ID from the request context.
func getUserID(r *http.Request) int64 {
	return sessionctx.UserID(r)
}

// handleApproved processes an approved report: deletes the message, adds a warning,
// and sends Telegram notifications.
func handleApproved(database *db.DB, botToken string, report *model.Report, adminID int64) {
	ctx := context.Background()

	if report.ReportedMessageID != nil {
		tgAPICall(ctx, botToken, "deleteMessage", map[string]any{
			"chat_id":    report.GroupID,
			"message_id": *report.ReportedMessageID,
		})
	}

	if err := database.AddWarning(report.GroupID, report.ReportedUserID, adminID, fmt.Sprintf("举报通过: %s", report.Content)); err != nil {
		slog.Error("Failed to record warning for approved report", "report_id", report.ID, "error", err)
	}

	preview := ""
	if report.ReportedMessageText != "" {
		preview = fmt.Sprintf("\n📄 被举报内容：「%s」", util.EscapeMd(truncate(report.ReportedMessageText, 100)))
	}

	groupMsg := fmt.Sprintf(
		"⚠️ 用户 [%d](tg://user?id=%d) 被举报处理\n\n📝 举报原因：%s%s\n\n🚫 违规消息已删除，用户已被警告。请遵守群规。",
		report.ReportedUserID, report.ReportedUserID, util.EscapeMd(report.Content), preview,
	)
	tgAPICall(ctx, botToken, "sendMessage", map[string]any{
		"chat_id":    report.GroupID,
		"text":       groupMsg,
		"parse_mode": "MarkdownV2",
	})

	reporterMsg := fmt.Sprintf(
		"✅ 你的举报已通过处理\n\n📋 举报编号：#%d\n📝 举报原因：%s\n🏠 群组：%d\n\n违规消息已删除，用户已被警告。感谢你维护群组秩序！",
		report.ID, report.Content, report.GroupID,
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
		report.ID, report.Content, report.GroupID,
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		slog.Error("tgAPICall request error", "method", method, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := telegramHTTPClient.Do(req)
	if err != nil {
		slog.Error("tgAPICall error", "method", method, "error", err)
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("tgAPICall response close error", "method", method, "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		slog.Error("tgAPICall failed", "method", method, "status", resp.StatusCode)
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
