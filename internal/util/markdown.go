package util

import (
	"regexp"
	"strconv"
	"strings"
)

// MDV2Special matches all MarkdownV2 special characters.
var MDV2Special = regexp.MustCompile(`[_*[\]()~` + "`" + `>#+\-=|{}.!\\]`)

// EscapeMd escapes all MarkdownV2 special characters in text.
func EscapeMd(text string) string {
	return MDV2Special.ReplaceAllString(text, "\\$0")
}

// EscapeMarkdown escapes MarkdownV2 special characters and converts list markers.
func EscapeMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Convert list markers (-, +, *) to ⦁
		if len(line) > 0 && (line[0] == '-' || line[0] == '+' || line[0] == '*') {
			if len(line) > 1 && line[1] == ' ' {
				lines[i] = "⦁ " + line[2:]
			}
		}
	}
	return EscapeMd(strings.Join(lines, "\n"))
}

// EscapeMarkdownCode escapes backticks and backslashes in code blocks.
func EscapeMarkdownCode(text string) string {
	re := regexp.MustCompile("[`\\\\]")
	return re.ReplaceAllString(text, "\\$0")
}

// EscapeMarkdownLinkUrl escapes closing parenthesis and backslashes in URLs.
func EscapeMarkdownLinkUrl(url string) string {
	re := regexp.MustCompile("[)\\\\]")
	return re.ReplaceAllString(url, "\\$0")
}

// MarkdownBold returns text wrapped in bold MarkdownV2 markers.
func MarkdownBold(text string) string {
	return "*" + EscapeMd(text) + "*"
}

// MarkdownItalic returns text wrapped in italic MarkdownV2 markers.
func MarkdownItalic(text string) string {
	return "_" + EscapeMd(text) + "_"
}

// MarkdownLink returns a MarkdownV2 formatted link.
func MarkdownLink(text, url string) string {
	return "[" + EscapeMd(text) + "](" + EscapeMarkdownLinkUrl(url) + ")"
}

// MarkdownPlaceholderStore protects MarkdownV2 content from escaping.
type MarkdownPlaceholderStore struct {
	placeholders []string
}

// NewMarkdownPlaceholderStore creates a new placeholder store.
func NewMarkdownPlaceholderStore() *MarkdownPlaceholderStore {
	return &MarkdownPlaceholderStore{
		placeholders: make([]string, 0),
	}
}

// Protect stores the markdown content and returns a placeholder token.
func (s *MarkdownPlaceholderStore) Protect(markdown string) string {
	token := "\u200BMDV2PH" + strconv.FormatInt(int64(len(s.placeholders)), 10) + "\u200B"
	s.placeholders = append(s.placeholders, markdown)
	return token
}

// Restore replaces all placeholder tokens with their original content.
func (s *MarkdownPlaceholderStore) Restore(text string) string {
	restored := text
	for i, placeholder := range s.placeholders {
		token := "\u200BMDV2PH" + strconv.FormatInt(int64(i), 10) + "\u200B"
		restored = strings.ReplaceAll(restored, token, placeholder)
	}
	return restored
}

// TrimCodeFencePadding removes trailing newline from code blocks.
func TrimCodeFencePadding(code string) string {
	if strings.HasSuffix(code, "\n") {
		return code[:len(code)-1]
	}
	return code
}

// FormatCodeBlock formats code as a MarkdownV2 code block.
func FormatCodeBlock(language, code string) string {
	trimmedLanguage := strings.TrimSpace(language)
	safeLanguage := ""
	if matched, _ := regexp.MatchString("^[A-Za-z0-9]+$", trimmedLanguage); matched {
		safeLanguage = trimmedLanguage
	}
	openingFence := "```\n"
	if safeLanguage != "" {
		openingFence = "```" + safeLanguage + "\n"
	}
	return openingFence + EscapeMarkdownCode(TrimCodeFencePadding(code)) + "\n```"
}

// FormatGeneratedMarkdownV2 converts GFM-style markdown to Telegram MarkdownV2.
func FormatGeneratedMarkdownV2(text string) string {
	store := NewMarkdownPlaceholderStore()
	formatted := strings.ReplaceAll(text, "\r\n", "\n")

	// Fenced code blocks
	codeBlockRe := regexp.MustCompile("(?s)```([^\n`]*)\n?([\\s\\S]*?)```")
	formatted = codeBlockRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := codeBlockRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return store.Protect(FormatCodeBlock(parts[1], parts[2]))
	})

	// Inline code
	inlineCodeRe := regexp.MustCompile("`([^`\n]+?)`")
	formatted = inlineCodeRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := inlineCodeRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return store.Protect("`" + EscapeMarkdownCode(parts[1]) + "`")
	})

	// Links
	linkRe := regexp.MustCompile(`\[([^\]\n]+?)\]\(([^)\s]+?)\)`)
	formatted = linkRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return store.Protect(MarkdownLink(parts[1], parts[2]))
	})

	// Headings (bold)
	headingRe := regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	formatted = headingRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := headingRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return store.Protect(MarkdownBold(strings.TrimSpace(parts[1])))
	})

	// Bold
	boldRe := regexp.MustCompile(`\*\*([^*\n]+?)\*\*`)
	formatted = boldRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := boldRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return store.Protect(MarkdownBold(parts[1]))
	})

	// Italic with underscores
	italicUnderscoreRe := regexp.MustCompile(`(^|[\s(\[{])_([^_\n]+?)_(?=$|[\s)\]},.!?;:])`)
	formatted = italicUnderscoreRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := italicUnderscoreRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return parts[1] + store.Protect(MarkdownItalic(parts[2]))
	})

	// Italic with asterisks
	italicAsteriskRe := regexp.MustCompile(`(^|[\s(\[{])\*([^*\n]+?)\*(?=$|[\s)\]},.!?;:])`)
	formatted = italicAsteriskRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := italicAsteriskRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return parts[1] + store.Protect(MarkdownItalic(parts[2]))
	})

	// List markers
	lines := strings.Split(formatted, "\n")
	for i, line := range lines {
		listMarkerRe := regexp.MustCompile(`^(\s*)[-+*]\s+`)
		if listMarkerRe.MatchString(line) {
			lines[i] = listMarkerRe.ReplaceAllString(line, "$1⦁ ")
		}
	}
	formatted = strings.Join(lines, "\n")

	return store.Restore(EscapeMd(formatted))
}