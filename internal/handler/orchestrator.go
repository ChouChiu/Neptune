package handler

import (
	"context"

	"github.com/ChouChiu/neptune/internal/db"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Orchestrator returns a default handler that dispatches messages:
// - Private chat: captcha reply
// - Group chat: keyword match
func Orchestrator(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		// Private chat: check for captcha reply
		if update.Message.Chat.Type == "private" {
			HandleCaptchaReply(ctx, b, database, update)
			return
		}

		// Group chat: keyword match only
		if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
			HandleKeywordMatch(ctx, b, database, update)
			return
		}
	}
}
