package handler

import (
	"bytes"
	"context"
	_ "embed"
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

//go:embed data/system-prompt.json
var systemPromptJSON []byte

//go:embed data/skills.json
var skillsJSON []byte

const (
	mimoAPIEndpoint     = "https://token-plan-sgp.xiaomimimo.com/v1"
	dailyLimit          = 15
	contextDays         = 7
	contextWindowSec    = contextDays * 24 * 60 * 60
	kvTTL               = 691200 // 8 days in seconds
	maxContextMessages  = 50
	apiTimeout          = 25 * time.Second
	maxRetries          = 2
	maxReplyLength      = 2048
	typingRefreshInterval = 4 * time.Second
)

// System prompt data structures.
type systemPromptData struct {
	Character map[string]any        `json:"character"`
	Examples  []systemPromptExample `json:"examples"`
}

type systemPromptExample struct {
	User  string `json:"user"`
	Reply string `json:"reply"`
}

// Skill data structures.
type skill struct {
	Name     string         `json:"name"`
	Triggers []string       `json:"triggers"`
	Content  map[string]any `json:"content"`
}

type skillsData struct {
	Default skill   `json:"default"`
	Dynamic []skill `json:"dynamic"`
}

var (
	systemPrompt string
	defaultSkill skill
	dynamicSkills []skill
	aiContextMu  sync.Mutex // replaces distributed lock for single-process
)

func init() {
	// Parse system prompt
	var spData systemPromptData
	if err := json.Unmarshal(systemPromptJSON, &spData); err != nil {
		slog.Error("Failed to parse system-prompt.json", "error", err)
		panic(err)
	}
	systemPrompt = systemPromptToText(spData)

	// Parse skills
	var skData skillsData
	if err := json.Unmarshal(skillsJSON, &skData); err != nil {
		slog.Error("Failed to parse skills.json", "error", err)
		panic(err)
	}
	defaultSkill = skData.Default
	dynamicSkills = skData.Dynamic
}

// formatCharacterField formats a single character field for the system prompt.
func formatCharacterField(key string, value any) string {
	switch v := value.(type) {
	case []any:
		strs := make([]string, len(v))
		for i, item := range v {
			strs[i] = fmt.Sprintf("%v", item)
		}
		return fmt.Sprintf("%s(%s)", key, strings.Join(strs, " | "))
	case map[string]any:
		parts := make([]string, 0, len(v))
		for k, val := range v {
			parts = append(parts, fmt.Sprintf("%s: %v", k, val))
		}
		return fmt.Sprintf("%s(%s)", key, strings.Join(parts, "；"))
	default:
		return fmt.Sprintf("%s(%v)", key, value)
	}
}

// systemPromptToText converts the JSON system prompt data to text format.
func systemPromptToText(data systemPromptData) string {
	char := data.Character
	name, _ := char["name"].(string)

	lines := make([]string, 0)
	for key, value := range char {
		if key == "name" {
			continue
		}
		displayKey := strings.ToUpper(key[:1]) + key[1:]
		lines = append(lines, formatCharacterField(displayKey, value))
	}

	charBlock := fmt.Sprintf(`[character("%s") {
%s}
]`, name, strings.Join(lines, ",\n"))

	examples := make([]string, len(data.Examples))
	for i, ex := range data.Examples {
		examples[i] = fmt.Sprintf("用户：%s\n涅普顿：%s", ex.User, ex.Reply)
	}

	return fmt.Sprintf("%s\n\n对话示例：\n%s", charBlock, strings.Join(examples, "\n\n"))
}

// formatSkillValue recursively formats a skill value for text output.
func formatSkillValue(value any, indent int) string {
	prefix := strings.Repeat("  ", indent)

	switch v := value.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case []any:
		if len(v) == 0 {
			return ""
		}
		if _, ok := v[0].(string); ok {
			strs := make([]string, len(v))
			for i, item := range v {
				strs[i] = fmt.Sprintf("%v", item)
			}
			return strings.Join(strs, "、")
		}
		items := make([]string, len(v))
		for i, item := range v {
			items[i] = fmt.Sprintf("%s- %s", prefix, formatSkillValue(item, indent+1))
		}
		return strings.Join(items, "\n")
	case map[string]any:
		lines := make([]string, 0, len(v))
		for k, val := range v {
			formatted := formatSkillValue(val, indent+1)
			if formatted == "" {
				continue
			}
			switch val.(type) {
			case []any, map[string]any:
				lines = append(lines, fmt.Sprintf("%s%s:\n%s", prefix, k, formatted))
			default:
				lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, k, formatted))
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

// skillToText converts a skill to text format.
func skillToText(s skill) string {
	return formatSkillValue(s.Content, 0)
}

// matchSkills finds skills matching the user message.
func matchSkills(message string) []skill {
	lower := strings.ToLower(message)
	var matched []skill
	for _, s := range dynamicSkills {
		for _, trigger := range s.Triggers {
			if strings.Contains(lower, strings.ToLower(trigger)) {
				matched = append(matched, s)
				break
			}
		}
	}
	return matched
}

// GetTodayDate returns today's date as YYYY-MM-DD.
func GetTodayDate() string {
	now := time.Now()
	return fmt.Sprintf("%04d-%02d-%02d", now.Year(), now.Month(), now.Day())
}

// ShouldTriggerAi checks if the message should trigger AI chat.
func ShouldTriggerAi(update *models.Update, botID int64) bool {
	msg := update.Message
	if msg == nil {
		return false
	}

	// Check mentions
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
					// We need the bot username to compare - check via @ prefix
					if strings.HasPrefix(strings.ToLower(mentionedText), "@") {
						// Will be validated against bot username in the caller
						return true
					}
				} else if entity.Type == "text_mention" && entity.User != nil && entity.User.ID == botID {
					return true
				}
			}
		}
	}

	// Check reply to bot
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

// getAiContext retrieves AI context from SQLite KV store.
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

// updateAiContext saves AI context to SQLite KV store with pruning.
func updateAiContext(database *db.DB, groupID int64, messages []model.AiContextMessage) error {
	key := fmt.Sprintf("ai:context:%d", groupID)
	cutoff := time.Now().Unix() - int64(contextWindowSec)

	// Filter by timestamp
	filtered := make([]model.AiContextMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Timestamp >= cutoff {
			filtered = append(filtered, msg)
		}
	}

	// Keep last N messages
	if len(filtered) > maxContextMessages {
		filtered = filtered[len(filtered)-maxContextMessages:]
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return err
	}

	return database.KVSet(key, string(data), kvTTL)
}

// callMimoApi calls the MiMo API with retry logic.
func callMimoApi(apiKey string, messages []map[string]string, sysPrompt string) (string, error) {
	type apiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type apiRequest struct {
		Model               string       `json:"model"`
		Messages            []apiMessage `json:"messages"`
		Stream              bool         `json:"stream"`
		Temperature         float64      `json:"temperature"`
		TopP                float64      `json:"top_p"`
		MaxCompletionTokens int          `json:"max_completion_tokens"`
	}
	type apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	apiMessages := make([]apiMessage, 0, len(messages)+1)
	apiMessages = append(apiMessages, apiMessage{Role: "system", Content: sysPrompt})
	for _, m := range messages {
		apiMessages = append(apiMessages, apiMessage{Role: m["role"], Content: m["content"]})
	}

	body, err := json.Marshal(apiRequest{
		Model:               "mimo-v2.5",
		Messages:            apiMessages,
		Stream:              false,
		Temperature:         1.0,
		TopP:                0.95,
		MaxCompletionTokens: 2048,
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
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, mimoAPIEndpoint+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			cancel()
			return "", err
		}
		req.Header.Set("api-key", apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			if ctx.Err() == context.DeadlineExceeded {
				lastError = fmt.Errorf("MiMo API timeout after %s", apiTimeout)
				slog.Error("MiMo API timeout", "attempt", attempt+1)
				continue
			}
			lastError = err
			slog.Error("MiMo API error", "attempt", attempt+1, "error", err)
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

		lastError = fmt.Errorf("MiMo API error: %d - %s", resp.StatusCode, string(respBody))
		slog.Error("MiMo API error", "attempt", attempt+1, "status", resp.StatusCode, "body", string(respBody))

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			continue
		}
		// Non-retryable error
		return "", lastError
	}

	return "", lastError
}

// GetChatResponse gets the full AI chat response including context management.
func GetChatResponse(database *db.DB, apiKey string, groupID int64, userID int64, userMessage string, isAdmin bool, groupContext *GroupContext) (string, error) {
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

	// Single-process mutex replaces distributed lock
	aiContextMu.Lock()
	defer aiContextMu.Unlock()

	context, err := getAiContext(database, groupID)
	if err != nil {
		return "", err
	}

	context = append(context, model.AiContextMessage{
		Role:      "user",
		Content:   userMessage,
		UserID:    &userID,
		Timestamp: time.Now().UnixMilli(),
	})

	if len(context) > maxContextMessages {
		context = context[len(context)-maxContextMessages:]
	}

	apiMessages := make([]map[string]string, len(context))
	for i, msg := range context {
		apiMessages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	matched := matchSkills(userMessage)
	sysPrompt := systemPrompt + "\n" + skillToText(defaultSkill)
	if len(matched) > 0 {
		for _, s := range matched {
			sysPrompt += "\n" + skillToText(s)
		}
	}
	if groupContext != nil {
		sysPrompt += fmt.Sprintf("\n\n[当前群组信息]\n群组名称：%s\n群组ID：%d", groupContext.Title, groupID)
		if groupContext.MemberCount > 0 {
			sysPrompt += fmt.Sprintf("\n成员数：%d", groupContext.MemberCount)
		}
		sysPrompt += "\n\n请根据群组氛围自然地回应，可以适当提及群组相关的话题。"
	}

	reply, err := callMimoApi(apiKey, apiMessages, sysPrompt)
	if err != nil {
		slog.Error("MiMo API call failed", "error", err)
		return "涅普？！刚才好像有什么东西掉线了……主角的网络冒险失败了一次，再试一次吧！", nil
	}

	if len(reply) > maxReplyLength {
		reply = reply[:maxReplyLength-20] + "……涅普！说得太多了啦~"
	}

	context = append(context, model.AiContextMessage{
		Role:      "assistant",
		Content:   reply,
		Timestamp: time.Now().UnixMilli(),
	})

	if err := updateAiContext(database, groupID, context); err != nil {
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

// startTypingIndicator starts a goroutine that sends typing indicator periodically.
func startTypingIndicator(ctx context.Context, b *tgbot.Bot, chatID int64) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		// Send immediately
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

// aiChatJob holds the data needed for background AI processing.
type aiChatJob struct {
	bot              *tgbot.Bot
	database         *db.DB
	apiKey           string
	groupID          int64
	groupTitle       string
	replyToMessageID int
	userID           int64
	message          string
}

// processAiChat runs the AI chat in background.
func processAiChat(ctx context.Context, job *aiChatJob) {
	stopTyping := startTypingIndicator(ctx, job.bot, job.groupID)
	defer stopTyping()

	// Check admin status
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

	// Get member count
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

	reply, err := GetChatResponse(job.database, job.apiKey, job.groupID, job.userID, job.message, isAdmin, groupCtx)
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

// sendAiReply sends the AI reply, trying MarkdownV2 first then falling back to plain text.
func sendAiReply(ctx context.Context, b *tgbot.Bot, sq *db.DB, groupID int64, replyToMessageID int, reply string) {
	formattedReply := util.FormatGeneratedMarkdownV2(reply)

	// Try MarkdownV2 first
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

	// Fallback to plain text
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

// mentionMatchRe matches @username mentions.
var mentionMatchRe = regexp.MustCompile(`@(\w+)`)

// HandleAiChat handles AI chat messages in groups. Returns true if the message was handled.
func HandleAiChat(ctx context.Context, b *tgbot.Bot, database *db.DB, apiKey string, update *models.Update) bool {
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

	// Get bot info
	me, err := b.GetMe(ctx)
	if err != nil {
		slog.Error("Failed to get bot info for AI chat", "error", err)
		return false
	}

	if !ShouldTriggerAi(update, me.ID) {
		return false
	}

	// Strip @mentions from the message
	userMessage := mentionMatchRe.ReplaceAllString(text, "")
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return false
	}

	// Include quoted message if replying
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

	// Run in background goroutine (replaces waitUntil)
	go processAiChat(ctx, &aiChatJob{
		bot:              b,
		database:         database,
		apiKey:           apiKey,
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
