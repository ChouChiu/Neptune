package handler

import (
	"context"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/util"
)

// SetWelcome returns a handler for the /setwelcome command.
func SetWelcome(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		text := ""
		if update.Message != nil && update.Message.Text != "" {
			// Remove "/setwelcome " prefix
			cmd := "/setwelcome"
			if len(update.Message.Text) > len(cmd)+1 {
				text = update.Message.Text[len(cmd)+1:]
			}
		}
		text = trimSpace(text)

		if text == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /setwelcome <消息>\n支持占位符: {nickname} {userid} {groupname}",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if len(text) > 4096 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "欢迎消息过长（最大 4096 字符）。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.UpdateWelcomeMessage(groupID, text); err != nil {
			slog.Error("Failed to update welcome message", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "✅ 欢迎消息已更新。",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// EnableWelcome returns a handler for the /enablewelcome command.
func EnableWelcome(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		if err := database.SetWelcomeEnabled(groupID, true); err != nil {
			slog.Error("Failed to enable welcome", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "✅ 入群欢迎已启用。",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// DisableWelcome returns a handler for the /disablewelcome command.
func DisableWelcome(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		if err := database.SetWelcomeEnabled(groupID, false); err != nil {
			slog.Error("Failed to disable welcome", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "✅ 入群欢迎已禁用。",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// WelcomeNewMembers returns a handler for new_chat_members events.
// It processes new members joining a group: deletes join message, sends welcome,
// creates pending verification, and restricts the user.
func WelcomeNewMembers(database *db.DB, configuredBotUsername string) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil || len(update.Message.NewChatMembers) == 0 {
			return
		}

		groupID := update.Message.Chat.ID

		config, err := database.GetGroupConfig(groupID)
		if err != nil {
			slog.Error("Failed to get group config", "error", err)
			return
		}
		if config == nil || config.WelcomeEnabled == 0 {
			return
		}

		_ = database.CleanExpiredVerifications()

		// Delete the join message
		b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
			ChatID:    groupID,
			MessageID: update.Message.ID,
		})

		for _, newMember := range update.Message.NewChatMembers {
			if newMember.IsBot {
				continue
			}

			userID := newMember.ID
			nickname := util.GetNickname(&newMember)

			welcomeText := util.ReplacePlaceholders(
				config.WelcomeMessage,
				util.EscapeMd(nickname),
				userID,
				escapeGroupTitle(update.Message.Chat.Title),
			)

			botUsername := configuredBotUsername
			if botUsername == "" {
				botUsername = getBotUsername(ctx, b)
			}
			if botUsername == "" {
				slog.Error("Failed to build verify URL: bot username is empty", "group_id", groupID, "user_id", userID)
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: groupID,
					Text:   "验证配置错误：无法获取机器人用户名，请管理员检查 BOT_USERNAME。",
				})
				continue
			}
			verifyURL := "https://t.me/" + botUsername + "?start=verify" + intToStr(groupID) + "_" + intToStr(userID)

			// Send welcome to the group
			groupMsg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:    groupID,
				Text:      welcomeText,
				ParseMode: models.ParseModeMarkdown,
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{
						{
							{Text: config.VerifyButtonText, URL: verifyURL},
						},
					},
				},
			})

			var welcomeMsgID *int64
			if err == nil && groupMsg != nil {
				mid := int64(groupMsg.ID)
				welcomeMsgID = &mid
			} else if err != nil {
				slog.Error("Failed to send welcome verification message", "group_id", groupID, "user_id", userID, "error", err)
			}

			timeout := config.VerifyTimeout
			expiresAt := util.CurrentTimestamp() + int64(timeout) + 300

			if err := database.AddPendingVerification(
				userID, groupID, "", expiresAt, welcomeMsgID, false,
			); err != nil {
				slog.Error("Failed to add pending verification", "error", err)
			}

			// Restrict user (mute)
			if err := restrictUser(ctx, b, groupID, userID); err != nil {
				slog.Error("Failed to restrict user", "user_id", userID, "error", err)
			}
		}
	}
}

// restrictUser restricts a user in a group (all permissions set to false).
func restrictUser(ctx context.Context, b *tgbot.Bot, groupID, userID int64) error {
	_, err := b.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
		ChatID:                        groupID,
		UserID:                        userID,
		Permissions:                   restrictedChatPermissions(),
		UseIndependentChatPermissions: true,
	})
	return err
}

func unrestrictUser(ctx context.Context, b *tgbot.Bot, groupID, userID int64) error {
	_, err := b.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
		ChatID:                        groupID,
		UserID:                        userID,
		Permissions:                   unrestrictedChatPermissions(),
		UseIndependentChatPermissions: true,
	})
	return err
}

func restrictedChatPermissions() *models.ChatPermissions {
	return &models.ChatPermissions{
		CanSendMessages:       false,
		CanSendAudios:         false,
		CanSendDocuments:      false,
		CanSendPhotos:         false,
		CanSendVideos:         false,
		CanSendVideoNotes:     false,
		CanSendVoiceNotes:     false,
		CanSendPolls:          false,
		CanSendOtherMessages:  false,
		CanAddWebPagePreviews: false,
		CanChangeInfo:         false,
		CanInviteUsers:        false,
		CanPinMessages:        false,
		CanManageTopics:       false,
		CanEditTag:            false,
		CanReactToMessages:    false,
	}
}

func unrestrictedChatPermissions() *models.ChatPermissions {
	return &models.ChatPermissions{
		CanSendMessages:       true,
		CanSendAudios:         true,
		CanSendDocuments:      true,
		CanSendPhotos:         true,
		CanSendVideos:         true,
		CanSendVideoNotes:     true,
		CanSendVoiceNotes:     true,
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
		CanAddWebPagePreviews: true,
		CanChangeInfo:         true,
		CanInviteUsers:        true,
		CanPinMessages:        true,
		CanManageTopics:       true,
		CanEditTag:            true,
		CanReactToMessages:    true,
	}
}

// trimSpace trims whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// escapeGroupTitle escapes a group title for MarkdownV2.
func escapeGroupTitle(title string) string {
	if title == "" {
		return ""
	}
	return util.EscapeMd(title)
}

// WelcomeNewMembersFromChatMember returns a handler for chat_member updates.
// This handles new member joins via the newer ChatMemberUpdated API.
func WelcomeNewMembersFromChatMember(database *db.DB, configuredBotUsername string) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.ChatMember == nil {
			return
		}

		cm := update.ChatMember
		slog.Info("WelcomeNewMembersFromChatMember triggered",
			"chat_id", cm.Chat.ID,
			"user_id", cm.From.ID,
			"old_status", cm.OldChatMember.Type,
			"new_status", cm.NewChatMember.Type,
		)

		// Check if this is a new member join:
		// Old status is NOT member/restricted (i.e., user was not in the group before)
		// New status is member or restricted (i.e., user is now in the group)
		oldStatus := cm.OldChatMember.Type
		newStatus := cm.NewChatMember.Type

		// User was not in the group before (left, banned, or unknown)
		wasNotMember := oldStatus != models.ChatMemberTypeMember && oldStatus != models.ChatMemberTypeRestricted

		// User is now in the group (member or restricted)
		isNowMember := newStatus == models.ChatMemberTypeMember || newStatus == models.ChatMemberTypeRestricted

		if !(wasNotMember && isNowMember) {
			slog.Info("Not a new join, skipping",
				"was_not_member", wasNotMember,
				"is_now_member", isNowMember,
			)
			return
		}

		groupID := cm.Chat.ID

		config, err := database.GetGroupConfig(groupID)
		if err != nil {
			slog.Error("Failed to get group config", "error", err)
			return
		}
		if config == nil || config.WelcomeEnabled == 0 {
			return
		}

		_ = database.CleanExpiredVerifications()

		// Get user from NewChatMember (could be Member or Restricted)
		var newMember *models.User
		switch cm.NewChatMember.Type {
		case models.ChatMemberTypeMember:
			if cm.NewChatMember.Member != nil {
				newMember = cm.NewChatMember.Member.User
			}
		case models.ChatMemberTypeRestricted:
			if cm.NewChatMember.Restricted != nil {
				newMember = cm.NewChatMember.Restricted.User
			}
		}
		if newMember == nil {
			slog.Warn("Could not get user from NewChatMember",
				"chat_id", groupID,
				"new_member_type", cm.NewChatMember.Type,
			)
			return
		}

		slog.Info("Processing new member from chat_member update",
			"user_id", newMember.ID,
			"nickname", newMember.FirstName,
			"is_bot", newMember.IsBot,
		)

		if newMember.IsBot {
			return
		}

		userID := newMember.ID
		nickname := util.GetNickname(newMember)

		welcomeText := util.ReplacePlaceholders(
			config.WelcomeMessage,
			util.EscapeMd(nickname),
			userID,
			escapeGroupTitle(cm.Chat.Title),
		)

		botUsername := configuredBotUsername
		if botUsername == "" {
			botUsername = getBotUsername(ctx, b)
		}
		if botUsername == "" {
			slog.Error("Failed to build verify URL: bot username is empty", "group_id", groupID, "user_id", userID)
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: groupID,
				Text:   "验证配置错误：无法获取机器人用户名，请管理员检查 BOT_USERNAME。",
			})
			return
		}
		verifyURL := "https://t.me/" + botUsername + "?start=verify" + intToStr(groupID) + "_" + intToStr(userID)

		// Send welcome to the group
		groupMsg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    groupID,
			Text:      welcomeText,
			ParseMode: models.ParseModeMarkdown,
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{
						{Text: config.VerifyButtonText, URL: verifyURL},
					},
				},
			},
		})

		var welcomeMsgID *int64
		if err == nil && groupMsg != nil {
			mid := int64(groupMsg.ID)
			welcomeMsgID = &mid
		} else if err != nil {
			slog.Error("Failed to send welcome verification message", "group_id", groupID, "user_id", userID, "error", err)
		}

		timeout := config.VerifyTimeout
		expiresAt := util.CurrentTimestamp() + int64(timeout) + 300

		if err := database.AddPendingVerification(
			userID, groupID, "", expiresAt, welcomeMsgID, false,
		); err != nil {
			slog.Error("Failed to add pending verification", "error", err)
		}

		// Restrict user (mute)
		if err := restrictUser(ctx, b, groupID, userID); err != nil {
			slog.Error("Failed to restrict user", "user_id", userID, "error", err)
		}
	}
}

// intToStr converts an int64 to string.
func intToStr(n int64) string {
	if n < 0 {
		return "-" + uintToStr(uint64(-n))
	}
	return uintToStr(uint64(n))
}

// uintToStr converts a uint64 to string.
func uintToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 20)
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
