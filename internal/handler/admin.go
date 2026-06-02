package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/util"
)

// ID returns a handler for the /id command.
// In groups: shows the group ID and auto-connects admin.
// In private chat: tells the user to use it in a group.
func ID(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		chat := update.Message.Chat
		if chat.Type == "private" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          chat.ID,
				Text:            util.EscapeMd("请在群组中使用此命令获取群组 ID。"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		// Auto-connect admin if user is a Telegram admin
		if update.Message.From != nil {
			member, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
				ChatID: chat.ID,
				UserID: update.Message.From.ID,
			})
			if err == nil && (member.Type == models.ChatMemberTypeAdministrator || member.Type == models.ChatMemberTypeOwner) {
				_ = database.ConnectAdmin(update.Message.From.ID, chat.ID)
			}
		}

		text := fmt.Sprintf("当前群组 ID: `%d`", chat.ID)
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          chat.ID,
			Text:            text,
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// Connect returns a handler for the /connect command.
// Verifies the user is a Telegram admin of the target group, then binds.
func Connect(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		chat := update.Message.Chat
		if chat.Type != "private" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          chat.ID,
				Text:            util.EscapeMd("请在私聊中使用此命令。"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		args := strings.TrimSpace(update.Message.Text)
		// Remove "/connect" prefix
		if len(args) > 8 {
			args = strings.TrimSpace(args[8:])
		} else {
			args = ""
		}

		if args == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          chat.ID,
				Text:            util.EscapeMd("用法: /connect <群组ID>"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		groupID, err := strconv.ParseInt(args, 10, 64)
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          chat.ID,
				Text:            util.EscapeMd("无效的群组 ID。"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if update.Message.From == nil {
			return
		}

		// Verify the user is a Telegram admin of the target group
		member, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
			ChatID: groupID,
			UserID: update.Message.From.ID,
		})
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          chat.ID,
				Text:            util.EscapeMd(fmt.Sprintf("无法验证群组权限: %v", err)),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if member.Type != models.ChatMemberTypeAdministrator && member.Type != models.ChatMemberTypeOwner {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          chat.ID,
				Text:            util.EscapeMd("你不是该群组的管理员，无法绑定。"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.ConnectAdmin(update.Message.From.ID, groupID); err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          chat.ID,
				Text:            util.EscapeMd("绑定失败，请稍后重试。"),
				ParseMode:       models.ParseModeMarkdown,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          chat.ID,
			Text:            util.EscapeMd(fmt.Sprintf("已绑定到群组 %d。现在可以在私聊中管理该群组。", groupID)),
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// Switch returns a handler for the /switch command.
// Shows inline keyboard to switch the currently managed group.
func Switch(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message.Chat.Type != "private" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "请在私聊中使用此命令。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if update.Message.From == nil {
			return
		}

		groupIDs, err := database.GetAdminGroups(update.Message.From.ID)
		if err != nil || len(groupIDs) == 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "你还没有绑定任何群组。使用 /connect <群组ID> 绑定。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		currentGroupID, _ := database.GetAdminGroupID(update.Message.From.ID)

		buttons := buildSwitchButtons(groupIDs, currentGroupID)
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "选择当前管理的群组：",
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: buttons,
			},
		})
	}
}

// SwitchCallback returns a handler for the "switch:" callback prefix.
func SwitchCallback(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.CallbackQuery == nil || update.CallbackQuery.Data == "" {
			return
		}

		groupIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "switch:")
		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			return
		}

		userID := update.CallbackQuery.From.ID
		if err := database.SetCurrentGroup(userID, groupID); err != nil {
			return
		}

		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            fmt.Sprintf("已切换到群组 %d", groupID),
		})

		// Update the message with new button states
		groupIDs, err := database.GetAdminGroups(userID)
		if err != nil {
			return
		}

		// Access the underlying message (MaybeInaccessibleMessage wraps it)
		msg := update.CallbackQuery.Message.Message
		if msg == nil {
			return
		}

		buttons := buildSwitchButtons(groupIDs, &groupID)
		b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
			ChatID:      msg.Chat.ID,
			MessageID:   msg.ID,
			Text:        "选择当前管理的群组：",
			ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
		})
	}
}

// buildSwitchButtons builds the inline keyboard for group switching.
func buildSwitchButtons(groupIDs []int64, currentGroupID *int64) [][]models.InlineKeyboardButton {
	buttons := make([][]models.InlineKeyboardButton, 0, len(groupIDs))
	for _, gid := range groupIDs {
		label := fmt.Sprintf("%d", gid)
		if currentGroupID != nil && gid == *currentGroupID {
			label = fmt.Sprintf("✅ %d", gid)
		}
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: label, CallbackData: fmt.Sprintf("switch:%d", gid)},
		})
	}
	return buttons
}
