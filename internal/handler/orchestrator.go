package handler

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
)

// Orchestrator returns a default handler that dispatches messages:
// - Private chat: captcha reply
// - Group chat: AI chat → keyword match
func Orchestrator(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		// Private chat: check for captcha reply
		if update.Message.Chat.Type == "private" {
			// TODO: handleCaptchaReply (Phase 3)
			return
		}

		// Group chat: AI chat → keyword match
		if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
			// TODO: handleAiChat (Phase 4)
			// TODO: handleKeywordMatch (Phase 3)
			return
		}
	}
}
