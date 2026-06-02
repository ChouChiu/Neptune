package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/util"
)

const (
	voteThreshold       = 5
	voteDuration        = 300 // 5 minutes
	initiatorCooldown   = 60  // 1 minute
)

// buildVoteText builds the vote display text.
func buildVoteText(targetName, initiatorName string, yesCount, noCount int, expiresAt int64) string {
	yesBar := strings.Repeat("🟢", min(yesCount, 10))
	noBar := strings.Repeat("🔴", min(noCount, 10))
	remaining := max(0, expiresAt-util.CurrentTimestamp())
	minutes := remaining / 60
	seconds := remaining % 60
	timeStr := fmt.Sprintf("%d秒", seconds)
	if minutes > 0 {
		timeStr = fmt.Sprintf("分%d秒", seconds)
		timeStr = fmt.Sprintf("%d", minutes) + timeStr
	}
	return fmt.Sprintf(
		"🗳️ 投票踢人\n\n目标: %s\n发起人: %s\n\n赞成: %s %d/%d\n反对: %s %d\n\n剩余时间: %s",
		targetName, initiatorName, yesBar, yesCount, voteThreshold, noBar, noCount, timeStr,
	)
}

// EnableVotekick returns a handler for the /enablevotekick command.
func EnableVotekick(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		config, _ := database.GetGroupConfig(groupID)
		if config != nil && config.VotekickEnabled != 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "投票踢人已经处于启用状态。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.SetVotekickEnabled(groupID, true); err != nil {
			slog.Error("Failed to enable votekick", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "✅ 投票踢人已启用。使用 /kick（回复目标消息）发起投票。",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// DisableVotekick returns a handler for the /disablevotekick command.
func DisableVotekick(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		config, _ := database.GetGroupConfig(groupID)
		if config == nil || config.VotekickEnabled == 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "投票踢人已经处于禁用状态。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.SetVotekickEnabled(groupID, false); err != nil {
			slog.Error("Failed to disable votekick", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            "✅ 投票踢人已禁用。",
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// Kick returns a handler for the /kick command.
func Kick(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		groupOK, groupID := requireGroupReply(ctx, b, update)
		if !groupOK {
			return
		}

		config, _ := database.GetGroupConfig(groupID)
		if config == nil || config.VotekickEnabled == 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "投票踢人未启用，请让管理员使用 /enablevotekick 启用。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if update.Message.From == nil {
			return
		}

		replyAllowed, target := requireReplyTarget(ctx, b, update)
		if !replyAllowed || target == nil {
			return
		}

		if !requireNonBot(ctx, b, update, target) {
			return
		}

		targetID := target.ID
		targetName := util.GetNickname(target)
		initiatorID := update.Message.From.ID

		if targetID == initiatorID {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "❌ 不能对自己发起投票。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		// Check if target is admin
		member, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
			ChatID: groupID,
			UserID: targetID,
		})
		if err == nil && (member.Type == models.ChatMemberTypeAdministrator || member.Type == models.ChatMemberTypeOwner) {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "❌ 不能对管理员发起投票。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		existing, _ := database.GetActiveVoteForTarget(groupID, targetID)
		if existing != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "❌ 该用户已有进行中的投票。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		now := util.CurrentTimestamp()
		lastVote, _ := database.GetLastVoteByInitiator(groupID, initiatorID)
		if lastVote != nil {
			elapsed := now - lastVote.CreatedAt
			if elapsed < initiatorCooldown {
				remaining := initiatorCooldown - elapsed
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:          update.Message.Chat.ID,
					Text:            fmt.Sprintf("❌ 冷却中，请等待 %d 秒后再试。", remaining),
					ReplyParameters: util.ReplyOptions(update.Message),
				})
				return
			}
		}

		voteID := util.RandomString(16)
		expiresAt := now + voteDuration

		if err := database.CreateActiveVote(voteID, groupID, targetID, initiatorID, now, expiresAt); err != nil {
			slog.Error("Failed to create vote", "error", err)
			return
		}

		initiatorName := util.GetNickname(update.Message.From)
		text := buildVoteText(targetName, initiatorName, 0, 0, expiresAt)

		sent, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   text,
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{
						{Text: "赞成 (0)", CallbackData: fmt.Sprintf("vk:%s:1", voteID)},
						{Text: "反对 (0)", CallbackData: fmt.Sprintf("vk:%s:0", voteID)},
					},
				},
			},
		})
		if err != nil {
			slog.Error("Failed to send vote message", "error", err)
			return
		}

		_ = database.UpdateVoteMessageID(voteID, int64(sent.ID))
	}
}

// VotekickCallback returns a handler for the "vk:" callback prefix.
func VotekickCallback(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.CallbackQuery == nil || update.CallbackQuery.Data == "" {
			return
		}

		data := update.CallbackQuery.Data
		// Format: vk:<voteId>:<choice>
		parts := strings.SplitN(data, ":", 3)
		if len(parts) < 3 {
			return
		}

		voteID := parts[1]
		choice := strToInt(parts[2])
		voterID := update.CallbackQuery.From.ID

		vote, err := database.GetActiveVote(voteID)
		if err != nil || vote == nil {
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "投票已结束。",
			})
			return
		}

		now := util.CurrentTimestamp()
		if now >= vote.ExpiresAt {
			if vote.MessageID != nil {
				b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
					ChatID:    vote.GroupID,
					MessageID: int(*vote.MessageID),
				})
			}
			_ = database.DeleteActiveVote(voteID)
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "投票已过期。",
			})
			return
		}

		if voterID == vote.TargetID {
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "你不能参与关于自己的投票。",
			})
			return
		}

		added, err := database.AddVoteRecord(voteID, voterID, choice)
		if err != nil {
			slog.Error("Failed to add vote record", "error", err)
			return
		}
		if !added {
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "你已经投过票了。",
			})
			return
		}

		yesCount, noCount, _ := database.GetVoteCounts(voteID)

		targetName := fmt.Sprintf("用户 %d", vote.TargetID)
		initiatorName := fmt.Sprintf("用户 %d", vote.InitiatorID)

		targetMember, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
			ChatID: vote.GroupID,
			UserID: vote.TargetID,
		})
		if err == nil {
			if u := chatMemberUser(targetMember); u != nil {
				targetName = util.GetNickname(u)
			}
		}

		initiatorMember, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
			ChatID: vote.GroupID,
			UserID: vote.InitiatorID,
		})
		if err == nil {
			if u := chatMemberUser(initiatorMember); u != nil {
				initiatorName = util.GetNickname(u)
			}
		}

		text := buildVoteText(targetName, initiatorName, yesCount, noCount, vote.ExpiresAt)

		if vote.MessageID != nil {
			b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
				ChatID:    vote.GroupID,
				MessageID: int(*vote.MessageID),
				Text:      text,
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{
						{
							{Text: fmt.Sprintf("赞成 (%d)", yesCount), CallbackData: fmt.Sprintf("vk:%s:1", voteID)},
							{Text: fmt.Sprintf("反对 (%d)", noCount), CallbackData: fmt.Sprintf("vk:%s:0", voteID)},
						},
					},
				},
			})
		}

		if yesCount >= voteThreshold {
			if vote.MessageID != nil {
				b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
					ChatID:    vote.GroupID,
					MessageID: int(*vote.MessageID),
				})
			}

			_, banErr := b.BanChatMember(ctx, &tgbot.BanChatMemberParams{
				ChatID: vote.GroupID,
				UserID: vote.TargetID,
			})
			if banErr != nil {
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: vote.GroupID,
					Text:   fmt.Sprintf("⚠️ 投票通过，但无法移出 %s（权限不足）。", targetName),
				})
			} else {
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: vote.GroupID,
					Text:   fmt.Sprintf("✅ 投票通过，已将 %s 移出群组。", targetName),
				})
			}
			_ = database.DeleteActiveVote(voteID)
			b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "投票通过！",
			})
			return
		}

		choiceText := "反对"
		if choice == 1 {
			choiceText = "赞成"
		}
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            fmt.Sprintf("已投%s。", choiceText),
		})
	}
}
