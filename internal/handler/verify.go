package handler

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/util"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	ruleAckWaitSeconds = 10
	maxVerifyTimeout   = 86400
)

// buildRuleText builds the rule acknowledgment text with countdown.
func buildRuleText(rule string, remaining int) string {
	countdown := "\n\n✅ 阅读时间已到，请点击下方按钮"
	if remaining > 0 {
		countdown = fmt.Sprintf("\n\n⏱️ %d 秒后可点击下方按钮", remaining)
	}
	return fmt.Sprintf("📋 *群规*\n\n%s%s", util.EscapeMd(rule), countdown)
}

// StartVerify returns a handler for the /start command.
// In private chat, it handles verify payloads; otherwise shows welcome.
func StartVerify(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.Chat.Type != "private" || update.Message.From == nil {
			return
		}

		args := strings.Fields(update.Message.Text)
		payload := ""
		if len(args) > 1 {
			payload = args[1]
		}

		if !strings.HasPrefix(payload, "verify") {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "欢迎使用 Neptune！发送 /help 查看命令列表。",
			})
			return
		}

		parts := strings.SplitN(payload[len("verify"):], "_", 2)
		if len(parts) < 2 {
			return
		}

		groupID, err1 := strconv.ParseInt(parts[0], 10, 64)
		targetUserID, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 != nil || err2 != nil || groupID == 0 || targetUserID == 0 {
			return
		}

		userID := update.Message.From.ID
		if userID != targetUserID {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "这不是你的验证链接。",
			})
			return
		}

		config, err := database.GetGroupConfig(groupID)
		if err != nil || config == nil {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "群组配置错误。",
			})
			return
		}

		existing, err := database.GetPendingVerification(userID, groupID)
		if err != nil {
			slog.Error("Failed to get pending verification", "user_id", userID, "group_id", groupID, "error", err)
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "验证状态读取失败，请稍后重试。",
			})
			return
		}
		if existing != nil && existing.Attempts >= maxCaptchaAttempts {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "验证失败次数过多，请重新加入群组。",
			})
			return
		}
		if existing != nil && existing.ExpiresAt <= util.CurrentTimestamp() {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "验证已过期，请重新加入群组。",
			})
			return
		}

		var welcomeMsgID *int64
		if existing != nil {
			welcomeMsgID = existing.WelcomeMessageID
		}

		if config.Rule != "" && (existing == nil || existing.RuleAckDone == 0) {
			showTime := util.CurrentTimestamp()
			if err := database.SetPendingCaptcha(
				userID, groupID, "",
				showTime+int64(config.VerifyTimeout),
				welcomeMsgID, false,
			); err != nil {
				slog.Error("Failed to store pending rule acknowledgment", "error", err)
			}

			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				Text:      buildRuleText(config.Rule, ruleAckWaitSeconds),
				ParseMode: models.ParseModeMarkdown,
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{
						{
							{Text: "我已知晓", CallbackData: fmt.Sprintf("rule_ack:%d:%d", groupID, showTime)},
						},
					},
				},
			})
			return
		}

		ruleAckDone := config.Rule != "" && existing != nil && existing.RuleAckDone != 0
		sent := sendCaptcha(ctx, b, database, userID, groupID, config.VerifyTimeout, welcomeMsgID, ruleAckDone)
		if !sent {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "无法发送验证码，请先私聊机器人并点击「开始」，然后重新加入群组。",
			})
		}
	}
}

// RuleAckCallback returns a handler for the "rule_ack:" callback prefix.
func RuleAckCallback(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.CallbackQuery == nil || update.CallbackQuery.Data == "" {
			return
		}

		// Format: rule_ack:<groupId>:<showTime>
		parts := strings.SplitN(update.CallbackQuery.Data, ":", 3)
		if len(parts) < 3 {
			return
		}

		groupID, err1 := strconv.ParseInt(parts[1], 10, 64)
		showTime, err2 := strconv.ParseInt(parts[2], 10, 64)
		if err1 != nil || err2 != nil || groupID == 0 || showTime == 0 {
			return
		}

		userID := update.CallbackQuery.From.ID

		config, err := database.GetGroupConfig(groupID)
		if err != nil || config == nil || config.Rule == "" {
			answerCallbackQuery(ctx, b, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "配置错误。",
			})
			return
		}

		now := util.CurrentTimestamp()
		elapsed := now - showTime

		if elapsed < int64(ruleAckWaitSeconds) {
			remaining := int(ruleAckWaitSeconds - elapsed)

			msg := update.CallbackQuery.Message
			if msg.Message != nil {
				editMessageText(ctx, b, &tgbot.EditMessageTextParams{
					ChatID:    msg.Message.Chat.ID,
					MessageID: msg.Message.ID,
					Text:      buildRuleText(config.Rule, remaining),
					ParseMode: models.ParseModeMarkdown,
					ReplyMarkup: &models.InlineKeyboardMarkup{
						InlineKeyboard: [][]models.InlineKeyboardButton{
							{
								{Text: "我已知晓", CallbackData: fmt.Sprintf("rule_ack:%d:%d", groupID, showTime)},
							},
						},
					},
				})
			}

			answerCallbackQuery(ctx, b, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            fmt.Sprintf("还需等待 %d 秒", remaining),
			})
			return
		}

		existing, err := database.GetPendingVerification(userID, groupID)
		if err != nil {
			slog.Error("Failed to get pending verification in rule_ack", "error", err)
			return
		}
		if existing == nil || existing.ExpiresAt <= now {
			answerCallbackQuery(ctx, b, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "验证已过期，请重新加入群组。",
			})
			return
		}
		if existing.Attempts >= maxCaptchaAttempts {
			answerCallbackQuery(ctx, b, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "验证失败次数过多，请重新加入群组。",
			})
			return
		}

		if existing.RuleAckDone != 0 {
			answerCallbackQuery(ctx, b, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "你已经点过啦~",
			})
			return
		}

		if err := database.SetRuleAckDone(userID, groupID); err != nil {
			slog.Error("Failed to set rule_ack_done", "error", err)
		}

		answerCallbackQuery(ctx, b, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "正在生成验证码...",
		})

		// Remove the button
		msg := update.CallbackQuery.Message
		if msg.Message != nil {
			editMessageReplyMarkup(ctx, b, &tgbot.EditMessageReplyMarkupParams{
				ChatID:    msg.Message.Chat.ID,
				MessageID: msg.Message.ID,
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{},
				},
			})
		}

		sent := sendCaptcha(ctx, b, database, userID, groupID, config.VerifyTimeout, existing.WelcomeMessageID, true)
		if !sent {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.CallbackQuery.From.ID,
				Text:   "无法发送验证码，请先私聊机器人并点击「开始」，然后重新加入群组。",
			})
		}
	}
}

// sendCaptcha generates and sends a captcha to the user in private chat.
func sendCaptcha(ctx context.Context, b *tgbot.Bot, database *db.DB, userID, groupID int64, timeout int, welcomeMsgID *int64, ruleAckDone bool) bool {
	captcha := util.GenerateCaptcha(5)

	expiresAt := util.CurrentTimestamp() + int64(timeout)
	caption := fmt.Sprintf("请回复图片中的文字完成验证（%d秒内有效）", timeout)

	_, err := b.SendPhoto(ctx, &tgbot.SendPhotoParams{
		ChatID:  userID,
		Photo:   &models.InputFileUpload{Filename: "captcha.bmp", Data: bytes.NewReader(captcha.BMP)},
		Caption: caption,
	})
	if err != nil {
		slog.Error("Failed to send captcha photo", "user_id", userID, "error", err)
		return false
	}

	if err := database.SetPendingCaptcha(
		userID, groupID, captcha.Text, expiresAt, welcomeMsgID, ruleAckDone,
	); err != nil {
		slog.Error("Failed to store pending captcha", "error", err)
		return false
	}

	return true
}

// SetVerifyButton returns a handler for the /setverifybutton command.
func SetVerifyButton(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		text := util.CommandArgs(update.Message.Text, "/setverifybutton")
		if text == "" {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /setverifybutton <按钮文案>",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.UpdateVerifyButtonText(groupID, text); err != nil {
			slog.Error("Failed to update verify button text", "error", err)
			return
		}

		sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("✅ 认证按钮文案已更新为: %s", text),
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// SetVerifyTimeout returns a handler for the /setverifytimeout command.
func SetVerifyTimeout(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		timeoutStr := util.CommandArgs(update.Message.Text, "/setverifytimeout")
		if timeoutStr == "" {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /setverifytimeout <秒数>",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil || timeout <= 0 || timeout > maxVerifyTimeout {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            fmt.Sprintf("请输入有效的秒数（1-%d）。", maxVerifyTimeout),
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.UpdateVerifyTimeout(groupID, timeout); err != nil {
			slog.Error("Failed to update verify timeout", "error", err)
			return
		}

		sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("✅ 认证超时时间已更新为: %d 秒", timeout),
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// TestVerify returns a handler for the /testverify command.
func TestVerify(database *db.DB, configuredBotUsername string) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.Chat.Type == "private" {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            util.EscapeMd("请在群组中使用此命令。"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		allowed, _ := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		groupID := update.Message.Chat.ID
		config, err := database.GetGroupConfig(groupID)
		if err != nil || config == nil || config.WelcomeEnabled == 0 {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            util.EscapeMd("请先使用 /enablewelcome 启用入群欢迎。"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if update.Message.From == nil {
			return
		}

		userID := update.Message.From.ID
		nickname := util.GetNickname(update.Message.From)
		groupTitle := update.Message.Chat.Title

		botUsername := strings.TrimPrefix(configuredBotUsername, "@")
		if botUsername == "" {
			sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "验证配置错误：无法获取机器人用户名，请管理员检查 BOT_USERNAME。",
			})
			return
		}

		verifyURL := fmt.Sprintf("https://t.me/%s?start=verify%d_%d", botUsername, groupID, userID)

		welcomeText := util.ReplacePlaceholders(
			util.EscapeMd(config.WelcomeMessage),
			util.EscapeMd(nickname),
			userID,
			util.EscapeMd(groupTitle),
			true,
		)

		if _, err := sendVerificationWelcome(
			ctx, b, update.Message.Chat.ID, update.Message.MessageThreadID,
			welcomeText, config.VerifyButtonText, verifyURL,
		); err != nil {
			slog.Warn("TestVerify failed to send verification message", "error", err)
		}
	}
}
