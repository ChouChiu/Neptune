package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ChouChiu/neptune/internal/db"
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

	text := update.Message.Text
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

	for _, v := range verifications {
		groupID := v.GroupID

		if v.Attempts >= maxCaptchaAttempts {
			_ = database.RemovePendingVerification(userID, groupID)
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "验证失败次数过多，请重新加入群组。",
			})
			continue
		}

		if strings.EqualFold(text, v.CaptchaText) {
			// Delete welcome message in the group
			if v.WelcomeMessageID != nil {
				deleteMessage(ctx, b, &tgbot.DeleteMessageParams{
					ChatID:    groupID,
					MessageID: int(*v.WelcomeMessageID),
				})
			}

			// Unrestrict user
			if err := unrestrictUser(ctx, b, groupID, userID); err != nil {
				slog.Error("Failed to unrestrict user after captcha", "error", err)
				sendMessage(ctx, b, &tgbot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "验证成功，但解除限制失败。请联系管理员。 (error: -10001)",
				})
			} else {
				member, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
					ChatID: groupID,
					UserID: userID,
				})
				if err != nil {
					slog.Error("Failed to verify member permissions after captcha", "error", err)
					sendMessage(ctx, b, &tgbot.SendMessageParams{
						ChatID: update.Message.Chat.ID,
						Text:   "验证成功，但解除限制失败。请联系管理员。 (error: -10002)",
					})
					return true
				}
				if !canMemberSendMessages(member) {
					slog.Error("Member still cannot send messages after captcha", "group_id", groupID, "user_id", userID, "status", member.Type)
					sendMessage(ctx, b, &tgbot.SendMessageParams{
						ChatID: update.Message.Chat.ID,
						Text:   "验证成功，但解除限制失败。请联系管理员。 (error: -10003)",
					})
					return true
				}

				_ = database.RemovePendingVerification(userID, groupID)
				sendMessage(ctx, b, &tgbot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "✅ 验证成功！你现在可以在群组中发言了。",
				})
			}
			return true
		}
	}

	// Increment attempts for all pending verifications
	_ = database.IncrementVerificationAttempts(userID)

	if len(verifications) > 0 {
		first := verifications[0]
		remaining := maxCaptchaAttempts - first.Attempts - 1
		if remaining > 0 {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "❌ 验证码错误，还剩 " + intToStr(int64(remaining)) + " 次机会。",
			})
		} else {
			// Remove all pending verifications for this user
			for _, v := range verifications {
				_ = database.RemovePendingVerification(userID, v.GroupID)
			}
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "❌ 验证失败次数过多，请重新加入群组。",
			})
		}
		return true
	}

	return false
}

func canMemberSendMessages(member *models.ChatMember) bool {
	if member == nil {
		return false
	}

	switch member.Type {
	case models.ChatMemberTypeOwner, models.ChatMemberTypeAdministrator, models.ChatMemberTypeMember:
		return true
	case models.ChatMemberTypeRestricted:
		return member.Restricted != nil && member.Restricted.CanSendMessages
	default:
		return false
	}
}
