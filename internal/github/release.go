package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ChouChiu/neptune/internal/util"
)

const (
	telegramAPI     = "https://api.telegram.org"
	maxMessageLength = 4096
	maxRetries       = 3
	retryDelay       = 1 * time.Second
)

// GitHubRelease represents a GitHub release webhook payload.
type GitHubRelease struct {
	Action  string         `json:"action"`
	Release ReleaseDetails `json:"release"`
}

// ReleaseDetails holds the release information.
type ReleaseDetails struct {
	TagName string  `json:"tag_name"`
	Name    *string `json:"name"`
	Body    *string `json:"body"`
	HTMLURL string  `json:"html_url"`
	PreRelease bool `json:"prerelease"`
	Draft   bool    `json:"draft"`
}

// VerifySignature verifies the HMAC-SHA256 signature of a GitHub webhook payload.
func VerifySignature(body []byte, signatureHeader string, secret string) bool {
	if signatureHeader == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if len(expected) != len(signatureHeader) {
		return false
	}

	// Constant-time comparison
	var result byte
	for i := 0; i < len(expected); i++ {
		result |= expected[i] ^ signatureHeader[i]
	}
	return result == 0
}

// calloutRe matches GitHub callout markers.
var calloutRe = regexp.MustCompile(`(?i)^>?\s*\[!(?:NOTE|WARNING|TIP|IMPORTANT|CAUTION)\]\s*`)

// fencedCodeRe matches fenced code blocks.
var fencedCodeRe = regexp.MustCompile("(?s)```([^\n`]*)\n?([\\s\\S]*?)```")

// inlineCodeRe matches inline code.
var inlineCodeRe = regexp.MustCompile("`([^`\n]+?)`")

// imageRe matches markdown images.
var imageRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

// linkRe matches markdown links.
var linkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// headingRe matches markdown headings.
var headingRe = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)

// listMarkerRe matches list markers at line start.
var listMarkerRe = regexp.MustCompile(`(?m)^(\s*)[-*+]\s`)

// boldRe matches bold text.
var boldRe = regexp.MustCompile(`\*\*([^*\n]+?)\*\*`)

// blockquoteRe matches blockquotes.
var blockquoteRe = regexp.MustCompile(`(?m)^>\s?(.*)$`)

// htmlTagRe matches HTML tags.
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// ConvertGfmToMarkdownV2 converts GitHub Flavored Markdown to Telegram MarkdownV2.
func ConvertGfmToMarkdownV2(gfm string) string {
	store := util.NewMarkdownPlaceholderStore()

	// Normalize CRLF
	text := strings.ReplaceAll(gfm, "\r\n", "\n")

	// Strip GitHub callout markers line by line
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = calloutRe.ReplaceAllString(line, "")
	}
	text = strings.Join(lines, "\n")

	// Fenced code blocks → protect
	text = fencedCodeRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := fencedCodeRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return store.Protect("```\n" + util.EscapeMarkdownCode(util.TrimCodeFencePadding(parts[2])) + "\n```")
	})

	// Inline code → protect
	text = inlineCodeRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := inlineCodeRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return store.Protect("`" + util.EscapeMarkdownCode(parts[1]) + "`")
	})

	// Images → remove
	text = imageRe.ReplaceAllString(text, "")

	// Links → protect
	text = linkRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return store.Protect(util.MarkdownLink(parts[1], parts[2]))
	})

	// Headings → bold
	text = headingRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := headingRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return store.Protect(util.MarkdownBold(strings.TrimSpace(parts[1])))
	})

	// List markers → ⦁
	text = listMarkerRe.ReplaceAllString(text, "$1⦁ ")

	// Bold **text** → *text*
	text = boldRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := boldRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return store.Protect(util.MarkdownBold(parts[1]))
	})

	// Blockquotes → protect as Telegram >text
	text = blockquoteRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := blockquoteRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		trimmed := strings.TrimSpace(parts[1])
		if trimmed == "" {
			return ""
		}
		return store.Protect(">" + util.EscapeMd(trimmed))
	})

	// Strip HTML tags
	text = htmlTagRe.ReplaceAllString(text, "")

	return store.Restore(util.EscapeMd(text))
}

// FormatReleaseMessage formats a GitHub release into a Telegram MarkdownV2 message.
func FormatReleaseMessage(release *GitHubRelease) string {
	tagName := util.EscapeMd(release.Release.TagName)
	isOhos := false
	if release.Release.Name != nil {
		isOhos = strings.Contains(strings.ToLower(*release.Release.Name), "ohos")
	}
	header := fmt.Sprintf("🎉 Kazumi %s 已发布", tagName)
	if isOhos {
		header = fmt.Sprintf("🎉 Kazumi %s for OHOS 已发布", tagName)
	}

	releaseURL := release.Release.HTMLURL
	linkLine := util.MarkdownLink("🔗 Release 页面", releaseURL)

	suffix := "\n\n" + linkLine
	maxBodyLength := maxMessageLength - len(header) - len(suffix) - 4

	var body string
	if release.Release.Body != nil {
		body = strings.TrimSpace(*release.Release.Body)
		if len(body) > maxBodyLength {
			body = body[:maxBodyLength] + "…"
		}
	}

	var message string
	if body != "" {
		bodyMd := ConvertGfmToMarkdownV2(body)
		message = fmt.Sprintf("%s\n\n%s\n\n%s", header, bodyMd, linkLine)
	} else {
		message = fmt.Sprintf("%s\n\n%s", header, linkLine)
	}

	return message
}

// SendToTelegram sends a message to a Telegram channel with retry logic.
func SendToTelegram(ctx context.Context, botToken string, channelID string, text string) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPI, botToken)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		slog.Info("Sending release to channel", "channelID", channelID, "attempt", attempt)

		type sendMessageReq struct {
			ChatID    string `json:"chat_id"`
			Text      string `json:"text"`
			ParseMode string `json:"parse_mode"`
		}

		body, err := json.Marshal(sendMessageReq{
			ChatID:    channelID,
			Text:      text,
			ParseMode: "MarkdownV2",
		})
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Error("Release send error", "attempt", attempt, "error", err)
			if attempt < maxRetries {
				time.Sleep(retryDelay)
			}
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			slog.Info("Release sent successfully", "attempt", attempt)
			return nil
		}

		slog.Error("Release send failed", "attempt", attempt, "status", resp.StatusCode, "body", string(respBody))

		if attempt < maxRetries {
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("failed to send release to channel %s after %d attempts", channelID, maxRetries)
}

// HandleGitHubWebhook processes a GitHub release webhook.
func HandleGitHubWebhook(ctx context.Context, body []byte, signatureHeader string, botToken string, channelID string, webhookSecret string) error {
	if !VerifySignature(body, signatureHeader, webhookSecret) {
		return fmt.Errorf("invalid signature")
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return fmt.Errorf("failed to parse webhook body: %w", err)
	}

	// Only process published releases
	if release.Action != "published" {
		slog.Info("Ignoring non-published release", "action", release.Action)
		return nil
	}

	if release.Release.Draft {
		slog.Info("Ignoring draft release")
		return nil
	}

	message := FormatReleaseMessage(&release)
	return SendToTelegram(ctx, botToken, channelID, message)
}
