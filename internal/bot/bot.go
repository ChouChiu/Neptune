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
		tgbot.WithDefaultHandler(handler.Orchestrator(database, cfg)),
		tgbot.WithSkipGetMe(),
	)
	if err != nil {
		return nil, err
	}

	// Helper to register command with optional bot username suffix
	registerCommand := func(cmd string, h tgbot.HandlerFunc) {
		b.RegisterHandler(tgbot.HandlerTypeMessageText, cmd, tgbot.MatchTypeCommand, h)
		if cfg.BotUsername != "" {
			b.RegisterHandler(tgbot.HandlerTypeMessageText, cmd+"@"+cfg.BotUsername, tgbot.MatchTypeCommand, h)
		}
	}

	// Phase 2: Core commands
	registerCommand("ping", handler.Ping())
	registerCommand("help", handler.Help())
	registerCommand("id", handler.ID(database))
	registerCommand("connect", handler.Connect(database))
	registerCommand("switch", handler.Switch(database))

	// Phase 3: Group management commands
	registerCommand("start", handler.StartVerify(database, nil))
	registerCommand("setwelcome", handler.SetWelcome(database))
	registerCommand("enablewelcome", handler.EnableWelcome(database))
	registerCommand("disablewelcome", handler.DisableWelcome(database))
	registerCommand("setverifybutton", handler.SetVerifyButton(database))
	registerCommand("setverifytimeout", handler.SetVerifyTimeout(database))
	registerCommand("testverify", handler.TestVerify(database))
	registerCommand("rule", handler.Rule(database))
	registerCommand("addkeyword", handler.AddKeyword(database))
	registerCommand("addregex", handler.AddRegex(database))
	registerCommand("listkeywords", handler.ListKeywords(database))
	registerCommand("removekeyword", handler.RemoveKeywordCmd(database))
	registerCommand("enablevotekick", handler.EnableVotekick(database))
	registerCommand("disablevotekick", handler.DisableVotekick(database))
	registerCommand("kick", handler.Kick(database))
	registerCommand("warn", handler.Warn(database))
	registerCommand("report", handler.Report(database))

	// Callback handlers
	b.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "switch:", tgbot.MatchTypePrefix, handler.SwitchCallback(database))
	b.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "rule_ack:", tgbot.MatchTypePrefix, handler.RuleAckCallback(database))
	b.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "vk:", tgbot.MatchTypePrefix, handler.VotekickCallback(database))

	// Welcome new members handler (registered via RegisterHandlerMatchFunc for new_chat_members)
	b.RegisterHandlerMatchFunc(newChatMembersMatch, handler.WelcomeNewMembers(database))

	slog.Info("Bot initialized")
	return b, nil
}

// newChatMembersMatch matches updates with new_chat_members.
func newChatMembersMatch(update *models.Update) bool {
	return update.Message != nil && len(update.Message.NewChatMembers) > 0
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
			{Command: "setwelcome", Description: "设置欢迎消息"},
			{Command: "enablewelcome", Description: "启用入群欢迎"},
			{Command: "disablewelcome", Description: "禁用入群欢迎"},
			{Command: "setverifybutton", Description: "设置认证按钮文案"},
			{Command: "setverifytimeout", Description: "设置验证超时时间"},
			{Command: "testverify", Description: "测试验证消息"},
			{Command: "rule", Description: "设置群规"},
			{Command: "addkeyword", Description: "添加关键词规则"},
			{Command: "addregex", Description: "添加正则规则"},
			{Command: "listkeywords", Description: "列出所有规则"},
			{Command: "removekeyword", Description: "删除关键词规则"},
			{Command: "enablevotekick", Description: "启用投票踢人"},
			{Command: "disablevotekick", Description: "禁用投票踢人"},
			{Command: "kick", Description: "发起踢人投票"},
			{Command: "warn", Description: "警告用户"},
			{Command: "report", Description: "举报用户"},
		},
	})
	return err
}
