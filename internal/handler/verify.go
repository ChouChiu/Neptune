package handler

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/util"
)

const ruleAckWaitSeconds = 10

// buildRuleText builds the rule acknowledgment text with countdown.
func buildRuleText(rule string, remaining int) string {
	countdown := ""
	if remaining > 0 {
		countdown = fmt.Sprintf("\n\n⏱️ %d 秒后可点击下方按钮", remaining)
	} else {
		countdown = "\n\n✅ 阅读时间已到，请点击下方按钮"
	}
	return fmt.Sprintf("📋 *群规*\n\n%s%s", util.EscapeMd(rule), countdown)
}

// StartVerify returns a handler for the /start command.
// In private chat, it handles verify payloads; otherwise shows welcome.
func StartVerify(database *db.DB, cfg *interface{ IsReuseCaptcha() bool }) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.Chat.Type != "private" {
			return
		}

		args := strings.Fields(update.Message.Text)
		payload := ""
		if len(args) > 1 {
			payload = args[1]
		}

		if !strings.HasPrefix(payload, "verify") {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "欢迎使用 Neptune！发送 /help 查看命令列表。",
			})
			return
		}

		parts := strings.SplitN(payload[6:], "_", 2)
		if len(parts) < 2 {
			return
		}

		groupID := strToInt64(parts[0])
		targetUserID := strToInt64(parts[1])
		if groupID == 0 || targetUserID == 0 {
			return
		}

		if update.Message.From == nil || update.Message.From.ID != targetUserID {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "这不是你的验证链接。",
			})
			return
		}

		userID := update.Message.From.ID

		config, err := database.GetGroupConfig(groupID)
		if err != nil || config == nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "群组配置错误。",
			})
			return
		}

		existing, _ := database.GetPendingVerification(userID, groupID)
		var welcomeMsgID *int64
		if existing != nil {
			welcomeMsgID = existing.WelcomeMessageID
		}

		if config.Rule != "" {
			showTime := util.CurrentTimestamp()
			_ = database.AddPendingVerification(
				userID, groupID, "",
				util.CurrentTimestamp()+int64(config.VerifyTimeout),
				welcomeMsgID, false,
			)

			b.SendMessage(ctx, &tgbot.SendMessageParams{
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

		sent := sendCaptcha(ctx, b, database, userID, groupID, config.VerifyTimeout, welcomeMsgID)
		if !sent {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
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

		data := update.CallbackQuery.Data
		// Format: rule_ack:<groupId>:<showTime>
		parts := strings.SplitN(data, ":", 3)
		if len(parts) < 3 {
			return
		}

		groupID := strToInt64(parts[1])
		showTime := strToInt64(parts[2])
		if groupID == 0 || showTime == 0 {
			return
		}

		userID := update.CallbackQuery.From.ID

		config, err := database.GetGroupConfig(groupID)
		if err != nil || config == nil || config.Rule == "" {
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
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
				b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
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

			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            fmt.Sprintf("还需等待 %d 秒", remaining),
			})
			return
		}

		existing, _ := database.GetPendingVerification(userID, groupID)
		if existing == nil {
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "验证已过期，请重新加入群组。",
			})
			return
		}

		if existing.RuleAckDone != 0 {
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "你已经点过啦~",
			})
			return
		}

		_ = database.SetRuleAckDone(userID, groupID)

		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "正在生成验证码...",
		})

		// Remove the button
		msg := update.CallbackQuery.Message
		if msg.Message != nil {
			b.EditMessageReplyMarkup(ctx, &tgbot.EditMessageReplyMarkupParams{
				ChatID:    msg.Message.Chat.ID,
				MessageID: msg.Message.ID,
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{},
				},
			})
		}

		var welcomeMsgID *int64
		if existing.WelcomeMessageID != nil {
			welcomeMsgID = existing.WelcomeMessageID
		}

		sent := sendCaptcha(ctx, b, database, userID, groupID, config.VerifyTimeout, welcomeMsgID)
		if !sent {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: update.CallbackQuery.From.ID,
				Text:   "无法发送验证码，请先私聊机器人并点击「开始」，然后重新加入群组。",
			})
		}
	}
}

// sendCaptcha generates and sends a captcha to the user in private chat.
func sendCaptcha(ctx context.Context, b *tgbot.Bot, database *db.DB, userID, groupID int64, timeout int, welcomeMsgID *int64) bool {
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

	if err := database.AddPendingVerification(
		userID, groupID, captcha.Text, expiresAt, welcomeMsgID, false,
	); err != nil {
		slog.Error("Failed to add pending verification for captcha", "error", err)
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

		text := ""
		if update.Message != nil && update.Message.Text != "" {
			text = commandArgs(update.Message.Text, "/setverifybutton")
		}
		text = trimSpace(text)

		if text == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
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

		b.SendMessage(ctx, &tgbot.SendMessageParams{
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

		timeoutStr := ""
		if update.Message != nil && update.Message.Text != "" {
			timeoutStr = commandArgs(update.Message.Text, "/setverifytimeout")
		}
		timeoutStr = trimSpace(timeoutStr)

		if timeoutStr == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /setverifytimeout <秒数>",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		timeout := strToInt(timeoutStr)
		if timeout <= 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "请输入有效的秒数。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.UpdateVerifyTimeout(groupID, timeout); err != nil {
			slog.Error("Failed to update verify timeout", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("✅ 认证超时时间已更新为: %d 秒", timeout),
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// TestVerify returns a handler for the /testverify command.
func TestVerify(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.Chat.Type == "private" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
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
			b.SendMessage(ctx, &tgbot.SendMessageParams{
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

		botUsername := getBotUsername(ctx, b)
		verifyURL := "https://t.me/" + botUsername + "?start=verify" + intToStr(groupID) + "_" + intToStr(userID)

		welcomeText := util.ReplacePlaceholders(
			config.WelcomeMessage,
			util.EscapeMd(nickname),
			userID,
			escapeGroupTitle(groupTitle),
			true,
		)
		plainWelcomeText := util.ReplacePlaceholders(
			config.WelcomeMessage,
			nickname,
			userID,
			groupTitle,
			false,
		)

		replyMarkup := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: config.VerifyButtonText, URL: verifyURL},
				},
			},
		}

		_, err = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      welcomeText,
			ParseMode: models.ParseModeMarkdown,
			ReplyMarkup: replyMarkup,
		})
		if err != nil {
			slog.Warn("TestVerify: MarkdownV2 failed, retrying as plain text", "error", err)
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				Text:      plainWelcomeText,
				ReplyMarkup: replyMarkup,
			})
		}
	}
}

// strToInt64 converts a string to int64, returning 0 on error.
func strToInt64(s string) int64 {
	if len(s) == 0 {
		return 0
	}
	negative := false
	start := 0
	if s[0] == '-' {
		negative = true
		start = 1
	}
	var n int64
	for i := start; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	if negative {
		return -n
	}
	return n
}

// strToInt converts a string to int, returning 0 on error.
func strToInt(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
