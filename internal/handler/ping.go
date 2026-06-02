package handler

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/util"
)

// Ping returns a handler that responds with "Pong!".
func Ping() tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "\U0001f3d3 Pong!",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}
