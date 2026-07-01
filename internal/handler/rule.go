package handler

import (
	"context"
	"log/slog"

	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/util"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Rule returns a handler for the /rule command.
func Rule(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		rule := ""
		if update.Message != nil && update.Message.Text != "" {
			rule = commandArgs(update.Message.Text, "/rule")
		}
		rule = trimSpace(rule)

		if rule == "" {
			config, err := database.GetGroupConfig(groupID)
			if err != nil {
				slog.Error("Failed to get group config for rule", "error", err)
				return
			}

			if config != nil && config.Rule != "" {
				sendMessage(ctx, b, &tgbot.SendMessageParams{
					ChatID:          update.Message.Chat.ID,
					Text:            "当前群规:\n\n" + config.Rule + "\n\n使用 /rule <内容> 修改群规\n使用 /rule off 清除群规",
					ReplyParameters: util.ReplyOptions(update.Message),
				})
			} else {
				sendMessage(ctx, b, &tgbot.SendMessageParams{
					ChatID:          update.Message.Chat.ID,
					Text:            "当前未设置群规。\n\n使用 /rule <内容> 设置群规",
					ReplyParameters: util.ReplyOptions(update.Message),
				})
			}
			return
		}

		if rule == "off" {
			if err := database.UpdateGroupRule(groupID, ""); err != nil {
				slog.Error("Failed to clear rule", "error", err)
				return
			}
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "✅ 群规已清除。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if len(rule) > 2048 {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "群规过长（最大 2048 字符）。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.UpdateGroupRule(groupID, rule); err != nil {
			slog.Error("Failed to update rule", "error", err)
			return
		}

		sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "✅ 群规已设置。入群认证时将强制阅读群规。",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}
