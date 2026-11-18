package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/model"
	"github.com/ChouChiu/neptune/internal/util"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	welcomeSourceMessageNewChatMembers = "message_new_chat_members"
	welcomeSourceChatMember            = "chat_member"

	welcomeMessageDeleteDelay = 5 * time.Minute
	welcomeLockTTLSeconds     = 10
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

		text := util.CommandArgs(update.Message.Text, "/setwelcome")
		if text == "" {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /setwelcome <消息>\n支持占位符: {nickname} {userid} {groupname}",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if len(text) > 4096 {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
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

		sendMessage(ctx, b, &tgbot.SendMessageParams{
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

		sendMessage(ctx, b, &tgbot.SendMessageParams{
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

		sendMessage(ctx, b, &tgbot.SendMessageParams{
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

		deleteMessage(ctx, b, &tgbot.DeleteMessageParams{
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
		Permissions:                   defaultUnrestrictedPermissions(),
		UseIndependentChatPermissions: true,
	})
	return err
}

func defaultUnrestrictedPermissions() *models.ChatPermissions {
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
		CanInviteUsers:        true,
		CanReactToMessages:    true,
	}
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

func sendVerificationWelcome(ctx context.Context, b *tgbot.Bot, groupID int64, messageThreadID int, text, buttonText, verifyURL string) (*models.Message, error) {
	return b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          groupID,
		MessageThreadID: messageThreadID,
		Text:            text,
		ParseMode:       models.ParseModeMarkdown,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: buttonText, URL: verifyURL},
				},
			},
		},
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

func sendGroupVerifyPrompt(
	ctx context.Context,
	b *tgbot.Bot,
	config *model.GroupConfig,
	configuredBotUsername string,
	groupID int64,
	groupTitle string,
	messageThreadID int,
	user *models.User,
) (*int64, bool) {
	userID := user.ID
	nickname := util.GetNickname(user)

	welcomeText := util.ReplacePlaceholders(
		util.EscapeMd(config.WelcomeMessage),
		util.EscapeMd(nickname),
		userID,
		util.EscapeMd(groupTitle),
		true,
	)

	botUsername := strings.TrimPrefix(configuredBotUsername, "@")
	if botUsername == "" {
		slog.Error("Failed to build verify URL: bot username is empty", "group_id", groupID, "user_id", userID)
		sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID: groupID,
			Text:   "验证配置错误：无法获取机器人用户名，请管理员检查 BOT_USERNAME。",
		})
		return nil, false
	}

	verifyURL := fmt.Sprintf("https://t.me/%s?start=verify%d_%d", botUsername, groupID, userID)
	groupMsg, err := sendVerificationWelcome(
		ctx, b, groupID, messageThreadID,
		welcomeText, config.VerifyButtonText, verifyURL,
	)
	if err != nil {
		slog.Error("Failed to send welcome verification message", "group_id", groupID, "user_id", userID, "error", err)
		return nil, true
	}
	if groupMsg == nil {
		return nil, true
	}

	mid := int64(groupMsg.ID)
	return &mid, true
}

func processWelcomeMember(ctx context.Context, b *tgbot.Bot, database *db.DB, config *model.GroupConfig, configuredBotUsername string, req welcomeMemberRequest) {
	if req.user == nil || req.user.IsBot {
		return
	}

	userID := req.user.ID

	// Deduplicate concurrent new_chat_members and chat_member join events.
	lockName := fmt.Sprintf("welcome:%d:%d", req.groupID, userID)
	acquired, lockErr := database.AcquireLock(lockName, welcomeLockTTLSeconds)
	if lockErr != nil {
		slog.Error("Failed to acquire welcome lock", "group_id", req.groupID, "user_id", userID, "error", lockErr)
	} else if !acquired {
		slog.Debug("Skipping duplicate member join event", "group_id", req.groupID, "user_id", userID, "source", req.source)
		return
	}

	nickname := util.GetNickname(req.user)

	slog.Info("Processing new member verification",
		"group_id", req.groupID,
		"user_id", userID,
		"nickname", nickname,
		"is_bot", req.user.IsBot,
		"update_source", req.source,
	)

	expiresAt := util.CurrentTimestamp() + int64(config.VerifyTimeout) + 300
	if err := database.AddPendingVerification(
		userID, req.groupID, "", expiresAt, nil, false,
	); err != nil {
		slog.Error("Failed to add pending verification", "error", err)
	}

	welcomeMsgID, ok := sendGroupVerifyPrompt(
		ctx, b, config, configuredBotUsername,
		req.groupID, req.groupTitle, req.messageThreadID, req.user,
	)
	if !ok || welcomeMsgID == nil {
		return
	}

	updated, err := database.UpdateJoinWelcomeMessageID(userID, req.groupID, *welcomeMsgID)
	if err != nil {
		slog.Error("Failed to update join welcome message ID", "group_id", req.groupID, "user_id", userID, "error", err)
		return
	}
	if !updated {
		deleteMessage(ctx, b, &tgbot.DeleteMessageParams{
			ChatID:    req.groupID,
			MessageID: int(*welcomeMsgID),
		})
		return
	}

	slog.Info("Welcome verification message sent",
		"group_id", req.groupID,
		"user_id", userID,
		"message_id", *welcomeMsgID,
		"message_thread_id", req.messageThreadID,
		"update_source", req.source,
	)
	scheduleWelcomeMessageDeletion(ctx, b, database, userID, req.groupID, *welcomeMsgID)
}

// HandleUnverifiedGroupMessage intercepts group messages from unverified members,
// deletes the message, restricts the member, and sends a second verification prompt once.
func HandleUnverifiedGroupMessage(ctx context.Context, b *tgbot.Bot, database *db.DB, configuredBotUsername string, update *models.Update) bool {
	if update.Message == nil || update.Message.From == nil || update.Message.From.IsBot {
		return false
	}
	if update.Message.Chat.Type != "group" && update.Message.Chat.Type != "supergroup" {
		return false
	}
	if len(update.Message.NewChatMembers) > 0 || update.Message.LeftChatMember != nil {
		return false
	}

	groupID := update.Message.Chat.ID
	userID := update.Message.From.ID

	pv, err := database.GetPendingVerification(userID, groupID)
	if err != nil {
		slog.Error("Failed to check pending verification on group message", "group_id", groupID, "user_id", userID, "error", err)
		return false
	}
	if pv == nil {
		return false
	}

	deleteMessage(ctx, b, &tgbot.DeleteMessageParams{
		ChatID:    groupID,
		MessageID: update.Message.ID,
	})

	if pv.PromptedOnMessage != 0 {
		return true
	}

	if err := restrictUser(ctx, b, groupID, userID); err != nil {
		slog.Error("Failed to restrict unverified user on first message", "group_id", groupID, "user_id", userID, "error", err)
	}

	config, err := database.GetGroupConfig(groupID)
	if err != nil {
		slog.Error("Failed to get group config on first unverified message", "group_id", groupID, "user_id", userID, "error", err)
		return true
	}

	timeout := 300
	if config != nil && config.VerifyTimeout > 0 {
		timeout = config.VerifyTimeout
	}
	expiresAt := util.CurrentTimestamp() + int64(timeout) + 300

	firstTime, err := database.MarkFirstMessagePrompted(userID, groupID, expiresAt)
	if err != nil {
		slog.Error("Failed to mark first message prompted", "group_id", groupID, "user_id", userID, "error", err)
		return true
	}
	if !firstTime {
		return true
	}

	if pv.WelcomeMessageID != nil {
		deleteMessage(ctx, b, &tgbot.DeleteMessageParams{
			ChatID:    groupID,
			MessageID: int(*pv.WelcomeMessageID),
		})
	}

	if config == nil {
		return true
	}

	welcomeMsgID, ok := sendGroupVerifyPrompt(
		ctx, b, config, configuredBotUsername,
		groupID, update.Message.Chat.Title, update.Message.MessageThreadID, update.Message.From,
	)
	if !ok {
		return true
	}

	if err := database.UpdatePendingWelcomeMessageID(userID, groupID, welcomeMsgID); err != nil {
		slog.Error("Failed to update welcome message ID on first message", "group_id", groupID, "user_id", userID, "error", err)
	}

	if welcomeMsgID != nil {
		slog.Info("First-message verification prompt sent",
			"group_id", groupID,
			"user_id", userID,
			"message_id", *welcomeMsgID,
			"message_thread_id", update.Message.MessageThreadID,
		)
		scheduleWelcomeMessageDeletion(ctx, b, database, userID, groupID, *welcomeMsgID)
	}
	return true
}

// scheduleWelcomeMessageDeletion deletes the welcome message after a delay
// if the user hasn't completed verification by then.
func scheduleWelcomeMessageDeletion(ctx context.Context, b *tgbot.Bot, database *db.DB, userID, groupID int64, welcomeMsgID int64) {
	deleteCtx := context.WithoutCancel(ctx)
	go func() {
		select {
		case <-time.After(welcomeMessageDeleteDelay):
		case <-deleteCtx.Done():
			return
		}

		// Check if verification still exists with the same welcome message ID.
		v, err := database.GetPendingVerification(userID, groupID)
		if err != nil {
			slog.Error("Failed to check pending verification for delayed welcome deletion", "error", err)
			return
		}
		if v == nil || v.WelcomeMessageID == nil || *v.WelcomeMessageID != welcomeMsgID {
			return
		}

		// Delete the welcome message from the group
		if _, err := b.DeleteMessage(deleteCtx, &tgbot.DeleteMessageParams{
			ChatID:    groupID,
			MessageID: int(welcomeMsgID),
		}); err != nil {
			slog.Warn("Failed to delete expired welcome message",
				"group_id", groupID,
				"user_id", userID,
				"message_id", welcomeMsgID,
				"error", err,
			)
		} else {
			slog.Info("Deleted expired welcome message",
				"group_id", groupID,
				"user_id", userID,
				"message_id", welcomeMsgID,
			)
		}
	}()
}

// IsChatMemberJoin reports whether a chat_member update represents a user joining the group.
func IsChatMemberJoin(oldStatus, newStatus models.ChatMemberType) bool {
	wasNotMember := oldStatus != models.ChatMemberTypeMember && oldStatus != models.ChatMemberTypeRestricted
	isNowMember := newStatus == models.ChatMemberTypeMember || newStatus == models.ChatMemberTypeRestricted
	return wasNotMember && isNowMember
}
