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

// Report returns a handler for the /report command.
func Report(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		groupOK, groupID := requireGroupReply(ctx, b, update)
		if !groupOK {
			return
		}

		replyAllowed, target := requireReplyTarget(ctx, b, update)
		if !replyAllowed || target == nil {
			return
		}

		if !requireNonBot(ctx, b, update, target) {
			return
		}

		if update.Message.From == nil {
			return
		}

		content := util.CommandArgs(update.Message.Text, "/report")
		content = trimSpace(content)

		if content == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            util.EscapeMd("请填写举报内容。用法: /report <举报原因>"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		reportedText := ""
		if update.Message.ReplyToMessage != nil {
			reportedText = update.Message.ReplyToMessage.Text
			if reportedText == "" {
				reportedText = update.Message.ReplyToMessage.Caption
			}
		}

		var reportedMsgID *int64
		if update.Message.ReplyToMessage != nil {
			mid := int64(update.Message.ReplyToMessage.ID)
			reportedMsgID = &mid
		}

		if err := database.AddReport(
			groupID,
			update.Message.From.ID,
			target.ID,
			reportedMsgID,
			reportedText,
			content,
		); err != nil {
			slog.Error("Failed to add report", "error", err)
			return
		}

		text := fmt.Sprintf(
			"✉️ 涅普已经收到举报啦，会尽快处理～\n\n📝 举报内容：%s",
			util.EscapeMd(content),
		)

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            text,
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}
