package handler

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/maibot"
	"github.com/ChouChiu/neptune/internal/util"
)

const (
	dailyLimit            = 15
	maxReplyLength        = 2048
	typingRefreshInterval = 4 * time.Second
)

// GetTodayDate returns today's date as YYYY-MM-DD.
func GetTodayDate() string {
	now := time.Now()
	return fmt.Sprintf("%04d-%02d-%02d", now.Year(), now.Month(), now.Day())
}

// ShouldTriggerAi checks if the message should trigger AI chat.
func ShouldTriggerAi(update *models.Update, botID int64, botUsername string) bool {
	msg := update.Message
	if msg == nil {
		return false
	}

	botMention := "@" + botUsername
	if msg.Entities != nil {
		for _, entity := range msg.Entities {
			if entity.Type == "mention" || entity.Type == "text_mention" {
				if entity.Type == "mention" && msg.Text != "" {
					mentionedText := ""
					runes := []rune(msg.Text)
					end := entity.Offset + entity.Length
					if end <= len(runes) {
						mentionedText = string(runes[entity.Offset:end])
					}
					if botUsername != "" && strings.EqualFold(mentionedText, botMention) {
						return true
					}
				} else if entity.Type == "text_mention" && entity.User != nil && entity.User.ID == botID {
					return true
				}
			}
		}
	}

	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.ID == botID {
		repliedText := msg.ReplyToMessage.Text
		if msg.ReplyToMessage.Caption != "" {
			repliedText += msg.ReplyToMessage.Caption
		}
		systemKeywords := []string{
			"验证", "欢迎", "命令", "踢人", "投票", "群规", "关键词", "Pong",
		}
		for _, kw := range systemKeywords {
			if strings.Contains(repliedText, kw) {
				return false
			}
		}
		return true
	}

	return false
}

func removeBotMention(text, botUsername string) string {
	if botUsername == "" {
		return text
	}
	re := regexp.MustCompile(`(?i)@` + regexp.QuoteMeta(botUsername) + `\b`)
	return re.ReplaceAllString(text, "")
}

// GroupContext holds group information for AI context.
type GroupContext struct {
	Title       string
	MemberCount int
}

func startTypingIndicator(ctx context.Context, b *tgbot.Bot, chatID int64) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		_, _ = b.SendChatAction(ctx, &tgbot.SendChatActionParams{
			ChatID: chatID,
			Action: "typing",
		})
		ticker := time.NewTicker(typingRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = b.SendChatAction(ctx, &tgbot.SendChatActionParams{
					ChatID: chatID,
					Action: "typing",
				})
			}
		}
	}()

	return cancel
}

type aiChatJob struct {
	bot              *tgbot.Bot
	database         *db.DB
	maiBotClient     *maibot.Client
	groupID          int64
	groupTitle       string
	replyToMessageID int
	userID           int64
	message          string
}

func processAiChat(ctx context.Context, job *aiChatJob) {
	_, _ = job.bot.SetMessageReaction(ctx, &tgbot.SetMessageReactionParams{
		ChatID:    job.groupID,
		MessageID: job.replyToMessageID,
		Reaction: []models.ReactionType{
			{Type: models.ReactionTypeTypeEmoji, ReactionTypeEmoji: &models.ReactionTypeEmoji{Type: models.ReactionTypeTypeEmoji, Emoji: "👀"}},
		},
	})

	stopTyping := startTypingIndicator(ctx, job.bot, job.groupID)
	defer stopTyping()

	member, err := job.bot.GetChatMember(ctx, &tgbot.GetChatMemberParams{
		ChatID: job.groupID,
		UserID: job.userID,
	})
	isAdmin := false
	if err == nil && (member.Type == models.ChatMemberTypeOwner || member.Type == models.ChatMemberTypeAdministrator) {
		isAdmin = true
	}
	if !isAdmin {
		connected, _ := job.database.IsAdminConnected(job.userID, job.groupID)
		isAdmin = connected
	}

	today := GetTodayDate()
	if !isAdmin {
		currentUsage, err := job.database.IncrementAiUsage(job.userID, job.groupID, today)
		if err != nil {
			slog.Error("Failed to check AI usage", "error", err)
		} else if currentUsage > dailyLimit {
			_, _ = job.bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: job.groupID,
				Text:   "涅普涅普~今天的主角光环能量用完啦！明天再来找涅普玩吧~♪（每日限额15次）",
				ReplyParameters: &models.ReplyParameters{
					MessageID: job.replyToMessageID,
				},
			})
			return
		}
	}

	groupIDStr := fmt.Sprintf("%d", job.groupID)
	userIDStr := fmt.Sprintf("%d", job.userID)
	nickname := ""
	if job.message != "" {
		nickname = fmt.Sprintf("user_%d", job.userID)
	}

	reply, err := job.maiBotClient.SendMessage(groupIDStr, job.groupTitle, userIDStr, nickname, job.message)
	if err != nil {
		slog.Error("MaiBot call failed", "error", err)
		_, _ = job.bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: job.groupID,
			Text:   "涅普？！刚才好像有什么东西掉线了……主角的网络冒险失败了一次，再试一次吧！",
			ReplyParameters: &models.ReplyParameters{
				MessageID: job.replyToMessageID,
			},
		})
		return
	}

	if reply == "" {
		return
	}

	if len(reply) > maxReplyLength {
		reply = reply[:maxReplyLength-20] + "……涅普！说得太多了啦~"
	}

	slog.Info("MaiBot response", "groupID", job.groupID, "userID", job.userID, "length", len(reply))

	sendAiReply(ctx, job.bot, job.database, job.groupID, job.replyToMessageID, reply)
}

func sendAiReply(ctx context.Context, b *tgbot.Bot, sq *db.DB, groupID int64, replyToMessageID int, reply string) {
	formattedReply := util.FormatGeneratedMarkdownV2(reply)

	_, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    groupID,
		Text:      formattedReply,
		ParseMode: models.ParseModeMarkdown,
		ReplyParameters: &models.ReplyParameters{
			MessageID: replyToMessageID,
		},
	})
	if err == nil {
		return
	}

	slog.Error("AI chat MarkdownV2 send failed, falling back to plain text",
		"groupID", groupID, "error", err,
		"formattedPreview", formattedReply[:min(200, len(formattedReply))])

	_, err = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: groupID,
		Text:   reply,
		ReplyParameters: &models.ReplyParameters{
			MessageID: replyToMessageID,
		},
	})
	if err != nil {
		slog.Error("AI chat plain text fallback also failed", "groupID", groupID, "error", err)
	}
}

// HandleAiChat handles AI chat messages in groups. Returns true if the message was handled.
func HandleAiChat(ctx context.Context, b *tgbot.Bot, database *db.DB, maiBotClient *maibot.Client, update *models.Update) bool {
	if update.Message == nil {
		return false
	}
	if update.Message.Chat.Type != "group" && update.Message.Chat.Type != "supergroup" {
		return false
	}

	text := update.Message.Text
	if text == "" {
		return false
	}

	userID := update.Message.From.ID
	if userID == 0 {
		return false
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		slog.Error("Failed to get bot info for AI chat", "error", err)
		return false
	}

	if !ShouldTriggerAi(update, me.ID, me.Username) {
		return false
	}

	userMessage := removeBotMention(text, me.Username)
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return false
	}

	quoted := update.Message.ReplyToMessage
	if quoted != nil {
		quotedText := quoted.Text
		if quoted.Caption != "" {
			quotedText = quoted.Caption
		}
		if quotedText != "" {
			userMessage = fmt.Sprintf("[引用消息] %s\n\n%s", quotedText, userMessage)
		}
	}

	groupID := update.Message.Chat.ID
	groupTitle := update.Message.Chat.Title

	slog.Info("AI chat request",
		"groupID", groupID,
		"userID", userID,
		"messageLength", len(userMessage),
	)

	go processAiChat(ctx, &aiChatJob{
		bot:              b,
		database:         database,
		maiBotClient:     maiBotClient,
		groupID:          groupID,
		groupTitle:       groupTitle,
		replyToMessageID: update.Message.ID,
		userID:           userID,
		message:          userMessage,
	})

	return true
}

// GetBotID returns the bot's user ID, fetching it if needed.
func GetBotID(ctx context.Context, b *tgbot.Bot) int64 {
	me, err := b.GetMe(ctx)
	if err != nil {
		slog.Error("Failed to get bot ID", "error", err)
		return 0
	}
	return me.ID
}
