package handler

import (
	"context"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/model"
	"github.com/kazumi-group/neptune/internal/util"
)

const (
	welcomeSourceMessageNewChatMembers = "message_new_chat_members"
	welcomeSourceChatMember            = "chat_member"
)

type welcomeMemberRequest struct {
	groupID         int64
	groupTitle      string
	messageThreadID int
	source          string
	user            *models.User
}

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

// WelcomeNewMembers handles visible Telegram join service messages.
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

		b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
			ChatID:    groupID,
			MessageID: update.Message.ID,
		})

		for _, newMember := range update.Message.NewChatMembers {
			member := newMember
			processWelcomeMember(ctx, b, database, config, configuredBotUsername, welcomeMemberRequest{
				groupID:         groupID,
				groupTitle:      update.Message.Chat.Title,
				messageThreadID: update.Message.MessageThreadID,
				source:          welcomeSourceMessageNewChatMembers,
				user:            &member,
			})
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

func sendVerificationWelcome(ctx context.Context, b *tgbot.Bot, groupID int64, messageThreadID int, markdownText, plainText, buttonText, verifyURL string) (*models.Message, error) {
	replyMarkup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: buttonText, URL: verifyURL},
			},
		},
	}

	params := &tgbot.SendMessageParams{
		ChatID:          groupID,
		MessageThreadID: messageThreadID,
		Text:            markdownText,
		ParseMode:       models.ParseModeMarkdown,
		ReplyMarkup:     replyMarkup,
	}

	groupMsg, err := b.SendMessage(ctx, params)
	if err == nil {
		return groupMsg, nil
	}

	slog.Warn("Failed to send MarkdownV2 welcome verification message, retrying as plain text",
		"group_id", groupID,
		"message_thread_id", messageThreadID,
		"error", err,
	)

	return b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          groupID,
		MessageThreadID: messageThreadID,
		Text:            plainText,
		ReplyMarkup:     replyMarkup,
	})
}

// WelcomeNewMembersFromChatMember handles join events that arrive only as chat_member updates.
// Some large groups hide visible join service messages, so this path must stay active.
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

		if !IsChatMemberJoin(cm.OldChatMember.Type, cm.NewChatMember.Type) {
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

		newMember := chatMemberUser(&cm.NewChatMember)
		if newMember == nil {
			slog.Warn("Could not get user from NewChatMember",
				"chat_id", groupID,
				"new_member_type", cm.NewChatMember.Type,
			)
			return
		}

		processWelcomeMember(ctx, b, database, config, configuredBotUsername, welcomeMemberRequest{
			groupID:         groupID,
			groupTitle:      cm.Chat.Title,
			messageThreadID: 0,
			source:          welcomeSourceChatMember,
			user:            newMember,
		})
	}
}

func processWelcomeMember(ctx context.Context, b *tgbot.Bot, database *db.DB, config *model.GroupConfig, configuredBotUsername string, req welcomeMemberRequest) {
	if req.user == nil || req.user.IsBot {
		return
	}

	userID := req.user.ID
	nickname := util.GetNickname(req.user)

	slog.Info("Processing new member verification",
		"group_id", req.groupID,
		"user_id", userID,
		"nickname", nickname,
		"is_bot", req.user.IsBot,
		"update_source", req.source,
	)

	welcomeText := util.ReplacePlaceholders(
		config.WelcomeMessage,
		util.EscapeMd(nickname),
		userID,
		escapeGroupTitle(req.groupTitle),
	)
	plainWelcomeText := util.ReplacePlaceholders(
		config.WelcomeMessage,
		nickname,
		userID,
		req.groupTitle,
	)

	botUsername := configuredBotUsername
	if botUsername == "" {
		botUsername = getBotUsername(ctx, b)
	}
	if botUsername == "" {
		slog.Error("Failed to build verify URL: bot username is empty", "group_id", req.groupID, "user_id", userID)
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: req.groupID,
			Text:   "验证配置错误：无法获取机器人用户名，请管理员检查 BOT_USERNAME。",
		})
		return
	}

	verifyURL := "https://t.me/" + botUsername + "?start=verify" + intToStr(req.groupID) + "_" + intToStr(userID)
	groupMsg, err := sendVerificationWelcome(
		ctx, b, req.groupID, req.messageThreadID,
		welcomeText, plainWelcomeText, config.VerifyButtonText, verifyURL,
	)

	var welcomeMsgID *int64
	if err == nil && groupMsg != nil {
		mid := int64(groupMsg.ID)
		welcomeMsgID = &mid
		slog.Info("Welcome verification message sent",
			"group_id", req.groupID,
			"user_id", userID,
			"message_id", groupMsg.ID,
			"message_thread_id", req.messageThreadID,
			"update_source", req.source,
		)
	} else if err != nil {
		slog.Error("Failed to send welcome verification message", "group_id", req.groupID, "user_id", userID, "error", err)
	}

	expiresAt := util.CurrentTimestamp() + int64(config.VerifyTimeout) + 300
	if err := database.AddPendingVerification(
		userID, req.groupID, "", expiresAt, welcomeMsgID, false,
	); err != nil {
		slog.Error("Failed to add pending verification", "error", err)
	}

	if err := restrictUser(ctx, b, req.groupID, userID); err != nil {
		slog.Error("Failed to restrict user", "user_id", userID, "error", err)
	}
}

// IsChatMemberJoin reports whether a chat_member update represents a user joining the group.
func IsChatMemberJoin(oldStatus, newStatus models.ChatMemberType) bool {
	wasNotMember := oldStatus != models.ChatMemberTypeMember && oldStatus != models.ChatMemberTypeRestricted
	isNowMember := newStatus == models.ChatMemberTypeMember || newStatus == models.ChatMemberTypeRestricted
	return wasNotMember && isNowMember
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
