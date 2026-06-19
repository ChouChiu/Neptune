package handler

import (
	"context"
	"fmt"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/util"
)

// Warn returns a handler for the /warn command.
func Warn(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		groupOK, _ := requireGroupReply(ctx, b, update)
		if !groupOK {
			return
		}

		allowed, adminGroupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		replyAllowed, target := requireReplyTarget(ctx, b, update)
		if !replyAllowed || target == nil {
			return
		}

		if !requireNonBot(ctx, b, update, target) {
			return
		}

		// Check if target is admin
		member, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
			ChatID: update.Message.Chat.ID,
			UserID: target.ID,
		})
		if err == nil && (member.Type == models.ChatMemberTypeAdministrator || member.Type == models.ChatMemberTypeOwner) {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            util.EscapeMd("涅普不能警告管理员哦～"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if update.Message.From == nil {
			return
		}

		reason := util.CommandArgs(update.Message.Text, "/warn")
		reason = trimSpace(reason)

		if err := database.AddWarning(adminGroupID, target.ID, update.Message.From.ID, reason); err != nil {
			slog.Error("Failed to add warning", "error", err)
			return
		}

		count, _ := database.GetWarningCount(adminGroupID, target.ID)
		nickname := util.GetNickname(target)

		var text string
		if reason != "" {
			text = fmt.Sprintf(
				"⚠️ 涅普警告了 %s！\n\n📝 原因：%s\n📊 这是该用户的第 %d 次警告～",
				util.EscapeMd(nickname), util.EscapeMd(reason), count,
			)
		} else {
			text = fmt.Sprintf(
				"⚠️ 涅普警告了 %s！\n📊 这是该用户的第 %d 次警告～",
				util.EscapeMd(nickname), count,
			)
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            text,
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}
