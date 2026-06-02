package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/model"
	"github.com/kazumi-group/neptune/internal/util"
)

// keywordCacheEntry holds compiled keyword rules for a group.
type keywordCacheEntry struct {
	rules     []compiledKeywordRule
	expiresAt int64
}

// compiledKeywordRule holds a keyword rule with pre-compiled regex or simplified pattern.
type compiledKeywordRule struct {
	rule             model.KeywordRule
	simplifiedPattern string
}

var (
	keywordCache   = make(map[int64]*keywordCacheEntry)
	keywordCacheMu sync.RWMutex
)

const keywordCacheTTL = 60 // seconds

// getCachedKeywords returns cached keyword rules for a group, refreshing if expired.
func getCachedKeywords(database *db.DB, groupID int64) []compiledKeywordRule {
	now := time.Now().UnixMilli()

	keywordCacheMu.RLock()
	cached, ok := keywordCache[groupID]
	keywordCacheMu.RUnlock()

	if ok && cached.expiresAt > now {
		return cached.rules
	}

	// Refresh in background (non-blocking)
	go func() {
		rules, err := refreshKeywords(database, groupID)
		if err != nil {
			slog.Error("Failed to refresh keyword cache", "group_id", groupID, "error", err)
			return
		}
		keywordCacheMu.Lock()
		keywordCache[groupID] = &keywordCacheEntry{
			rules:     rules,
			expiresAt: time.Now().UnixMilli() + keywordCacheTTL*1000,
		}
		keywordCacheMu.Unlock()
	}()

	// Return stale cache if available
	if ok {
		return cached.rules
	}

	// No cache at all - block and fetch
	rules, err := refreshKeywords(database, groupID)
	if err != nil {
		slog.Error("Failed to fetch keywords", "group_id", groupID, "error", err)
		return nil
	}
	keywordCacheMu.Lock()
	keywordCache[groupID] = &keywordCacheEntry{
		rules:     rules,
		expiresAt: time.Now().UnixMilli() + keywordCacheTTL*1000,
	}
	keywordCacheMu.Unlock()
	return rules
}

// refreshKeywords fetches and compiles keyword rules from the database.
func refreshKeywords(database *db.DB, groupID int64) ([]compiledKeywordRule, error) {
	rawRules, err := database.GetKeywords(groupID)
	if err != nil {
		return nil, err
	}

	rules := make([]compiledKeywordRule, 0, len(rawRules))
	for _, rule := range rawRules {
		simplified := util.ToSimplified(rule.Pattern)
		rules = append(rules, compiledKeywordRule{
			rule:              rule,
			simplifiedPattern: strings.ToLower(simplified),
		})
	}
	return rules, nil
}

// HandleKeywordMatch checks if a group message matches any keyword rules.
// Returns true if a match was found and replied.
func HandleKeywordMatch(ctx context.Context, b *tgbot.Bot, database *db.DB, update *models.Update) bool {
	if update.Message == nil {
		return false
	}
	if update.Message.Chat.Type != "group" && update.Message.Chat.Type != "supergroup" {
		return false
	}

	groupID := update.Message.Chat.ID
	text := update.Message.Text
	if text == "" {
		return false
	}

	keywords := getCachedKeywords(database, groupID)
	if len(keywords) == 0 {
		return false
	}

	matched := matchKeyword(keywords, text)
	if matched == nil {
		return false
	}

	nickname := ""
	if update.Message.From != nil {
		nickname = util.GetNickname(update.Message.From)
	}

	var userID int64
	if update.Message.From != nil {
		userID = update.Message.From.ID
	}

	groupName := ""
	if update.Message.Chat.Title != "" {
		groupName = update.Message.Chat.Title
	}

	replyContent := util.ReplacePlaceholders(
		util.EscapeMd(matched.ReplyContent),
		util.EscapeMd(nickname),
		userID,
		util.EscapeMd(groupName),
	)

	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		Text:            replyContent,
		ParseMode:       models.ParseModeMarkdown,
		ReplyParameters: util.ReplyOptions(update.Message),
	})
	return true
}

// matchKeyword finds the first matching keyword rule for the given text.
func matchKeyword(keywords []compiledKeywordRule, text string) *model.KeywordRule {
	lowerText := strings.ToLower(text)
	simplifiedText := strings.ToLower(util.ToSimplified(text))

	for _, compiled := range keywords {
		rule := compiled.rule
		if rule.IsRegex != 0 {
			// Regex matching - try original pattern on original text
			if strings.Contains(lowerText, strings.ToLower(rule.Pattern)) {
				return &rule
			}
			// Try simplified pattern on simplified text
			if strings.Contains(simplifiedText, compiled.simplifiedPattern) {
				return &rule
			}
		} else {
			// Plain matching - only test simplified contains
			if strings.Contains(simplifiedText, compiled.simplifiedPattern) {
				return &rule
			}
		}
	}
	return nil
}

// AddKeyword returns a handler for the /addkeyword command.
func AddKeyword(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		args := ""
		if update.Message != nil && update.Message.Text != "" {
			cmd := "/addkeyword"
			if len(update.Message.Text) > len(cmd)+1 {
				args = update.Message.Text[len(cmd)+1:]
			}
		}
		args = trimSpace(args)

		if args == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /addkeyword <关键词> <回复内容>",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		spaceIdx := strings.Index(args, " ")
		if spaceIdx == -1 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "请同时提供关键词和回复内容。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		keyword := args[:spaceIdx]
		reply := args[spaceIdx+1:]

		if len(keyword) > 200 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "关键词过长（最大 200 字符）。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if len(reply) > 4096 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "回复内容过长（最大 4096 字符）。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.AddKeyword(groupID, keyword, false, reply, ""); err != nil {
			slog.Error("Failed to add keyword", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("✅ 已添加关键词规则: %s", keyword),
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// AddRegex returns a handler for the /addregex command.
func AddRegex(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		args := ""
		if update.Message != nil && update.Message.Text != "" {
			cmd := "/addregex"
			if len(update.Message.Text) > len(cmd)+1 {
				args = update.Message.Text[len(cmd)+1:]
			}
		}
		args = trimSpace(args)

		if args == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /addregex <正则表达式> <回复内容>",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		spaceIdx := strings.Index(args, " ")
		if spaceIdx == -1 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "请同时提供正则表达式和回复内容。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		pattern := args[:spaceIdx]
		reply := args[spaceIdx+1:]

		if len(pattern) > 200 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "正则表达式过长（最大 200 字符）。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		// ReDoS protection: test regex complexity
		testStart := time.Now()
		_ = strings.Contains(strings.Repeat("a", 1000), pattern)
		if time.Since(testStart) > 100*time.Millisecond {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "正则表达式过于复杂，可能导致性能问题。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		if err := database.AddKeyword(groupID, pattern, true, reply, ""); err != nil {
			slog.Error("Failed to add regex", "error", err)
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            fmt.Sprintf("✅ 已添加正则规则: %s", pattern),
			ReplyParameters: util.ReplyOptions(update.Message),
		})
	}
}

// ListKeywords returns a handler for the /listkeywords command.
func ListKeywords(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		keywords, err := database.GetKeywords(groupID)
		if err != nil {
			slog.Error("Failed to get keywords", "error", err)
			return
		}

		if len(keywords) == 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "暂无关键词规则。",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		lines := make([]string, len(keywords))
		for i, k := range keywords {
			icon := "🔤"
			if k.IsRegex != 0 {
				icon = "🔍"
			}
			lines[i] = fmt.Sprintf("%d. %s %s → %s", i+1, icon, k.Pattern, k.ReplyContent)
		}
		text := fmt.Sprintf("📋 *关键词规则*\n\n%s", util.EscapeMd(strings.Join(lines, "\n")))

		_, err = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			Text:            text,
			ParseMode:       models.ParseModeMarkdown,
			ReplyParameters: util.ReplyOptions(update.Message),
		})
		if err != nil {
			// Fallback to plain text
			plainText := fmt.Sprintf("📋 关键词规则\n\n%s", strings.Join(lines, "\n"))
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            plainText,
				ReplyParameters: util.ReplyOptions(update.Message),
			})
		}
	}
}

// RemoveKeyword returns a handler for the /removekeyword command.
func RemoveKeywordCmd(database *db.DB) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		allowed, groupID := requireAdmin(ctx, b, database, update)
		if !allowed {
			return
		}

		keyword := ""
		if update.Message != nil && update.Message.Text != "" {
			cmd := "/removekeyword"
			if len(update.Message.Text) > len(cmd)+1 {
				keyword = update.Message.Text[len(cmd)+1:]
			}
		}
		keyword = trimSpace(keyword)

		if keyword == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            "用法: /removekeyword <关键词>",
				ReplyParameters: util.ReplyOptions(update.Message),
			})
			return
		}

		removed, err := database.RemoveKeyword(groupID, keyword)
		if err != nil {
			slog.Error("Failed to remove keyword", "error", err)
			return
		}

		if removed {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            fmt.Sprintf("✅ 已删除关键词: %s", keyword),
				ReplyParameters: util.ReplyOptions(update.Message),
			})
		} else {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				Text:            fmt.Sprintf("未找到关键词: %s", keyword),
				ReplyParameters: util.ReplyOptions(update.Message),
			})
		}
	}
}
