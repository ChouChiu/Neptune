package handler

import (
	"context"

	"github.com/ChouChiu/neptune/internal/util"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Ping returns a handler that responds with "Pong!".
func Ping() tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "\U0001f3d3 Pong!",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}
