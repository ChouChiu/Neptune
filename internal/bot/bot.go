package bot

import (
	"context"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/handler"
	"github.com/kazumi-group/neptune/internal/model"
)

// New creates a new Bot instance with all handlers registered.
func New(cfg *model.Config, database *db.DB) (*tgbot.Bot, error) {
	b, err := tgbot.New(cfg.BotToken,
		tgbot.WithMiddlewares(
			loggingMiddleware,
			recoveryMiddleware,
			groupInitMiddleware(database),
		),
		tgbot.WithDefaultHandler(handler.Orchestrator(database)),
		tgbot.WithSkipGetMe(),
	)
	if err != nil {
		return nil, err
	}

	// Register command handlers
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "ping", tgbot.MatchTypeCommand, handler.Ping())
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "help", tgbot.MatchTypeCommand, handler.Help())
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "id", tgbot.MatchTypeCommand, handler.ID(database))
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "connect", tgbot.MatchTypeCommand, handler.Connect(database))
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "switch", tgbot.MatchTypeCommand, handler.Switch(database))

	// Register callback handlers
	b.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "switch:", tgbot.MatchTypePrefix, handler.SwitchCallback(database))

	slog.Info("Bot initialized")
	return b, nil
}

// SetCommands registers the bot command list with Telegram.
func SetCommands(ctx context.Context, b *tgbot.Bot) error {
	_, err := b.SetMyCommands(ctx, &tgbot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "help", Description: "显示帮助信息"},
			{Command: "ping", Description: "检查机器人是否在线"},
			{Command: "id", Description: "获取当前群组 ID"},
			{Command: "connect", Description: "绑定私聊与群组"},
			{Command: "switch", Description: "切换管理的群组（私聊）"},
		},
	})
	return err
}
