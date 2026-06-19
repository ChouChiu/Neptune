package handler

import (
	"context"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/util"
)

func commandArgs(text, command string) string {
	text = trimSpace(text)
	if text == "" {
		return ""
	}

	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}

	name := fields[0]
	if name != command && !strings.HasPrefix(name, command+"@") {
		return ""
	}
	return trimSpace(strings.TrimPrefix(text, name))
}

// requireGroupReply checks group context and sends an error if in private chat.
func requireGroupReply(ctx context.Context, b *tgbot.Bot, update *models.Update) (bool, int64) {
	if update.Message == nil {
		return false, 0
	}
	if update.Message.Chat.Type == "private" {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            util.EscapeMd("此命令只能在群组中使用。"),
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
		return false, 0
	}
	return true, update.Message.Chat.ID
}

// replyTarget returns the user being replied to, or nil if not a reply.
func replyTarget(update *models.Update) *models.User {
	if update.Message == nil || update.Message.ReplyToMessage == nil {
		return nil
	}
	return update.Message.ReplyToMessage.From
}

// requireReplyTarget checks that the message is a reply to another user.
func requireReplyTarget(ctx context.Context, b *tgbot.Bot, update *models.Update) (bool, *models.User) {
	target := replyTarget(update)
	if target == nil {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            util.EscapeMd("请回复目标用户的消息。"),
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
		return false, nil
	}
	return true, target
}

// requireNonBot checks that the target user is not a bot.
func requireNonBot(ctx context.Context, b *tgbot.Bot, update *models.Update, target *models.User) bool {
	if target.IsBot {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            util.EscapeMd("涅普不能对机器人执行此操作哦～"),
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
		return false
	}
	return true
}

// requireAdmin checks admin permission and returns (allowed, groupId).
// In groups: checks Telegram admin status. In private chat: checks admin_connections.
func requireAdmin(ctx context.Context, b *tgbot.Bot, database *db.DB, update *models.Update) (bool, int64) {
	if update.Message == nil {
		return false, 0
	}
	allowed, groupID := util.CheckAdminPermission(ctx, b, database, update.Message)
	if !allowed {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            util.EscapeMd("你没有权限执行此操作。"),
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
	return allowed, groupID
}

// getBotUsername retrieves the bot's username, caching it after the first call.
var cachedBotUsername string

func getBotUsername(ctx context.Context, b *tgbot.Bot) string {
	if cachedBotUsername != "" {
		return cachedBotUsername
	}
	me, err := b.GetMe(ctx)
	if err != nil {
		slog.Error("Failed to get bot username", "error", err)
		return ""
	}
	cachedBotUsername = me.Username
	return cachedBotUsername
}

// chatMemberUser extracts the User from a ChatMember discriminated union.
func chatMemberUser(member *models.ChatMember) *models.User {
	if member == nil {
		return nil
	}
	switch member.Type {
	case models.ChatMemberTypeOwner:
		if member.Owner != nil {
			return member.Owner.User
		}
	case models.ChatMemberTypeAdministrator:
		if member.Administrator != nil {
			return &member.Administrator.User
		}
	case models.ChatMemberTypeMember:
		if member.Member != nil {
			return member.Member.User
		}
	case models.ChatMemberTypeRestricted:
		if member.Restricted != nil {
			return member.Restricted.User
		}
	case models.ChatMemberTypeBanned:
		if member.Banned != nil {
			return member.Banned.User
		}
	}
	return nil
}
