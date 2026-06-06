package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/model"
	"github.com/kazumi-group/neptune/internal/util"
)

const (
	dailyLimit            = 15
	contextDays           = 7
	contextWindowSec      = contextDays * 24 * 60 * 60
	kvTTL                 = 691200 // 8 days in seconds
	maxContextMessages    = 50
	apiTimeout            = 120 * time.Second
	maxRetries            = 2
	maxReplyLength        = 2048
	typingRefreshInterval = 4 * time.Second
)

var (
	groupContextLocks sync.Map // map[int64]*sync.Mutex
)

func getGroupContextLock(groupID int64) *sync.Mutex {
	lock, _ := groupContextLocks.LoadOrStore(groupID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

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

func getAiContext(database *db.DB, groupID int64) ([]model.AiContextMessage, error) {
	key := fmt.Sprintf("ai:context:%d", groupID)
	data, err := database.KVGet(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return []model.AiContextMessage{}, nil
	}

	var messages []model.AiContextMessage
	if err := json.Unmarshal([]byte(*data), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func updateAiContext(database *db.DB, groupID int64, messages []model.AiContextMessage) error {
	key := fmt.Sprintf("ai:context:%d", groupID)
	cutoff := time.Now().Unix() - int64(contextWindowSec)

	filtered := make([]model.AiContextMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Timestamp >= cutoff {
			filtered = append(filtered, msg)
		}
	}

	if len(filtered) > maxContextMessages {
		filtered = filtered[len(filtered)-maxContextMessages:]
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return err
	}

	return database.KVSet(key, string(data), kvTTL)
}

// callHermesApi calls the Hermes Agent API Server with retry logic.
func callHermesApi(apiURL, apiKey string, messages []map[string]string, groupInfo string) (string, error) {
	type apiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type apiRequest struct {
		Model    string       `json:"model"`
		Messages []apiMessage `json:"messages"`
		Stream   bool         `json:"stream"`
	}
	type apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	apiMessages := make([]apiMessage, 0, len(messages)+1)
	if groupInfo != "" {
		apiMessages = append(apiMessages, apiMessage{Role: "system", Content: groupInfo})
	}
	for _, m := range messages {
		apiMessages = append(apiMessages, apiMessage{Role: m["role"], Content: m["content"]})
	}

	body, err := json.Marshal(apiRequest{
		Model:    "hermes-agent",
		Messages: apiMessages,
		Stream:   false,
	})
	if err != nil {
		return "", err
	}

	var lastError error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			cancel()
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			if ctx.Err() == context.DeadlineExceeded {
				lastError = fmt.Errorf("Hermes API timeout after %s", apiTimeout)
				slog.Error("Hermes API timeout", "attempt", attempt+1)
				continue
			}
			lastError = err
			slog.Error("Hermes API error", "attempt", attempt+1, "error", err)
			if attempt == maxRetries {
				break
			}
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			lastError = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var apiResp apiResponse
			if err := json.Unmarshal(respBody, &apiResp); err != nil {
				return "", err
			}
			if len(apiResp.Choices) > 0 {
				return apiResp.Choices[0].Message.Content, nil
			}
			return "", nil
		}

		lastError = fmt.Errorf("Hermes API error: %d - %s", resp.StatusCode, string(respBody))
		slog.Error("Hermes API error", "attempt", attempt+1, "status", resp.StatusCode, "body", string(respBody))

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			continue
		}
		return "", lastError
	}

	return "", lastError
}

// GetChatResponse gets the full AI chat response including context management.
func GetChatResponse(database *db.DB, hermesAPIURL, hermesAPIKey string, groupID int64, userID int64, userMessage string, isAdmin bool, groupContext *GroupContext) (string, error) {
	today := GetTodayDate()

	var currentUsage int
	if !isAdmin {
		var err error
		currentUsage, err = database.IncrementAiUsage(userID, groupID, today)
		if err != nil {
			return "", err
		}
		if currentUsage > dailyLimit {
			return "涅普涅普~今天的主角光环能量用完啦！明天再来找涅普玩吧~♪（每日限额15次）", nil
		}
	}

	groupLock := getGroupContextLock(groupID)
	groupLock.Lock()
	defer groupLock.Unlock()

	ctx, err := getAiContext(database, groupID)
	if err != nil {
		return "", err
	}

	ctx = append(ctx, model.AiContextMessage{
		Role:      "user",
		Content:   userMessage,
		UserID:    &userID,
		Timestamp: time.Now().UnixMilli(),
	})

	if len(ctx) > maxContextMessages {
		ctx = ctx[len(ctx)-maxContextMessages:]
	}

	apiMessages := make([]map[string]string, len(ctx))
	for i, msg := range ctx {
		apiMessages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	var groupInfo string
	if groupContext != nil {
		groupInfo = fmt.Sprintf("[当前群组信息]\n群组名称：%s\n群组ID：%d", groupContext.Title, groupID)
		if groupContext.MemberCount > 0 {
			groupInfo += fmt.Sprintf("\n成员数：%d", groupContext.MemberCount)
		}
	}

	reply, err := callHermesApi(hermesAPIURL, hermesAPIKey, apiMessages, groupInfo)
	if err != nil {
		slog.Error("Hermes API call failed", "error", err)
		return "涅普？！刚才好像有什么东西掉线了……主角的网络冒险失败了一次，再试一次吧！", nil
	}

	if len(reply) > maxReplyLength {
		reply = reply[:maxReplyLength-20] + "……涅普！说得太多了啦~"
	}

	ctx = append(ctx, model.AiContextMessage{
		Role:      "assistant",
		Content:   reply,
		Timestamp: time.Now().UnixMilli(),
	})

	if err := updateAiContext(database, groupID, ctx); err != nil {
		slog.Error("Failed to update AI context", "error", err)
	}

	if isAdmin {
		return reply, nil
	}

	remaining := dailyLimit - currentUsage
	return fmt.Sprintf("%s\n\n_剩余次数: %d/%d_", reply, remaining, dailyLimit), nil
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
	hermesAPIURL     string
	hermesAPIKey     string
	groupID          int64
	groupTitle       string
	replyToMessageID int
	userID           int64
	message          string
}

func processAiChat(ctx context.Context, job *aiChatJob) {
	// React to the user's message to indicate processing
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

	var groupCtx *GroupContext
	memberCount, err := job.bot.GetChatMemberCount(ctx, &tgbot.GetChatMemberCountParams{
		ChatID: job.groupID,
	})
	if err == nil {
		groupCtx = &GroupContext{
			Title:       job.groupTitle,
			MemberCount: memberCount,
		}
	} else {
		groupCtx = &GroupContext{Title: job.groupTitle}
	}

	reply, err := GetChatResponse(job.database, job.hermesAPIURL, job.hermesAPIKey, job.groupID, job.userID, job.message, isAdmin, groupCtx)
	if err != nil {
		slog.Error("AI chat background error", "error", err)
		_, _ = job.bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: job.groupID,
			Text:   "涅普？！出了点状况，主角光环暂时失效了……再试一次吧！",
			ReplyParameters: &models.ReplyParameters{
				MessageID: job.replyToMessageID,
			},
		})
		return
	}

	slog.Info("AI chat response", "groupID", job.groupID, "userID", job.userID, "length", len(reply))

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
func HandleAiChat(ctx context.Context, b *tgbot.Bot, database *db.DB, hermesAPIURL, hermesAPIKey string, update *models.Update) bool {
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
		hermesAPIURL:     hermesAPIURL,
		hermesAPIKey:     hermesAPIKey,
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
