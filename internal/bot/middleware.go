package bot

import (
	"context"
	"log/slog"
	"runtime/debug"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
)

// loggingMiddleware logs each incoming update.
func loggingMiddleware(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message != nil {
			var userID int64
			if update.Message.From != nil {
				userID = update.Message.From.ID
			}
			slog.Info("Message",
				"chat_id", update.Message.Chat.ID,
				"chat_type", update.Message.Chat.Type,
				"user_id", userID,
				"text", update.Message.Text,
			)
		} else if update.CallbackQuery != nil {
			slog.Info("CallbackQuery",
				"data", update.CallbackQuery.Data,
				"user_id", update.CallbackQuery.From.ID,
			)
		} else if update.ChatMember != nil {
			slog.Info("ChatMember update",
				"chat_id", update.ChatMember.Chat.ID,
				"chat_type", update.ChatMember.Chat.Type,
				"user_id", update.ChatMember.From.ID,
				"old_status", update.ChatMember.OldChatMember.Type,
				"new_status", update.ChatMember.NewChatMember.Type,
			)
		} else if update.MyChatMember != nil {
			slog.Info("MyChatMember update",
				"chat_id", update.MyChatMember.Chat.ID,
				"chat_type", update.MyChatMember.Chat.Type,
				"user_id", update.MyChatMember.From.ID,
				"old_status", update.MyChatMember.OldChatMember.Type,
				"new_status", update.MyChatMember.NewChatMember.Type,
			)
		}
		next(ctx, b, update)
	}
}

// recoveryMiddleware recovers from panics in handlers.
func recoveryMiddleware(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Handler panic recovered", "error", r, "stack", string(debug.Stack()))
			}
		}()
		next(ctx, b, update)
	}
}

// groupInitMiddleware ensures the group record exists in the database for group messages.
func groupInitMiddleware(database *db.DB) tgbot.Middleware {
	return func(next tgbot.HandlerFunc) tgbot.HandlerFunc {
		return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			if update.Message != nil && (update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup") {
				if err := database.InitGroup(update.Message.Chat.ID); err != nil {
					slog.Error("Failed to init group", "chat_id", update.Message.Chat.ID, "error", err)
				}
			} else if update.ChatMember != nil && (update.ChatMember.Chat.Type == "group" || update.ChatMember.Chat.Type == "supergroup") {
				if err := database.InitGroup(update.ChatMember.Chat.ID); err != nil {
					slog.Error("Failed to init group", "chat_id", update.ChatMember.Chat.ID, "error", err)
				}
			}
			next(ctx, b, update)
		}
	}
}
