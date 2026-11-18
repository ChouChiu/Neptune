package handler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/model"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const maxCaptchaAttempts = 5

// HandleCaptchaReply processes DM replies for captcha verification.
// Returns true if the message was handled (was a captcha reply).
func HandleCaptchaReply(ctx context.Context, b *tgbot.Bot, database *db.DB, update *models.Update) bool {
	if update.Message == nil || update.Message.Chat.Type != "private" || update.Message.From == nil {
		return false
	}

	text := strings.TrimSpace(update.Message.Text)
	if text == "" || strings.HasPrefix(text, "/") {
		return false
	}

	userID := update.Message.From.ID

	verifications, err := database.GetPendingVerificationsByUser(userID)
	if err != nil {
		slog.Error("Failed to get pending verifications", "error", err)
		return false
	}

	if len(verifications) == 0 {
		return false
	}

	var active []model.PendingVerification
	for _, v := range verifications {
		groupID := v.GroupID

		if v.Attempts >= maxCaptchaAttempts {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "验证失败次数过多，请重新加入群组。",
			})
			continue
		}

		if strings.EqualFold(text, v.CaptchaText) {
			if v.WelcomeMessageID != nil {
				deleteMessage(ctx, b, &tgbot.DeleteMessageParams{
					ChatID:    groupID,
					MessageID: int(*v.WelcomeMessageID),
				})
			}

			verifyAndUnrestrictMember(ctx, b, database, userID, groupID, update.Message.Chat.ID, v.PromptedOnMessage != 0)
			return true
		}

		active = append(active, v)
	}

	if len(active) == 0 {
		return true
	}

	if err := database.IncrementVerificationAttempts(userID); err != nil {
		slog.Error("Failed to increment verification attempts", "error", err)
	}

	remaining := maxCaptchaAttempts - active[0].Attempts - 1
	if remaining > 0 {
		sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ 验证码错误，还剩 " + strconv.Itoa(remaining) + " 次机会。",
		})
	} else {
		sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ 验证失败次数过多，请重新加入群组。",
		})
	}
	return true
}

func verifyAndUnrestrictMember(ctx context.Context, b *tgbot.Bot, database *db.DB, userID, groupID, dmChatID int64, restricted bool) {
	if restricted {
		if err := unrestrictUser(ctx, b, groupID, userID); err != nil {
			slog.Error("Failed to unrestrict user after captcha", "group_id", groupID, "user_id", userID, "error", err)
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: dmChatID,
				Text:   "验证成功，但解除限制失败。请联系管理员。",
			})
			return
		}
	}

	_ = database.RemovePendingVerification(userID, groupID)
	sendMessage(ctx, b, &tgbot.SendMessageParams{
		ChatID: dmChatID,
		Text:   "✅ 验证成功！你现在可以在群组中发言了。",
	})
}
