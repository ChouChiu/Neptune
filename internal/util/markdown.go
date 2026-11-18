package util

import (
	"strconv"
	"strings"
)

var (
	mdV2Replacer = strings.NewReplacer(
		`\`, `\\`,
		`_`, `\_`,
		`*`, `\*`,
		`[`, `\[`,
		`]`, `\]`,
		`(`, `\(`,
		`)`, `\)`,
		`~`, `\~`,
		"`", "\\`",
		`>`, `\>`,
		`#`, `\#`,
		`+`, `\+`,
		`-`, `\-`,
		`=`, `\=`,
		`|`, `\|`,
		`{`, `\{`,
		`}`, `\}`,
		`.`, `\.`,
		`!`, `\!`,
	)
	mdCodeReplacer    = strings.NewReplacer("`", "\\`", `\`, `\\`)
	mdLinkURLReplacer = strings.NewReplacer(")", `\)`, `\`, `\\`)
)

// EscapeMd escapes all MarkdownV2 special characters in text.
func EscapeMd(text string) string {
	return mdV2Replacer.Replace(text)
}

// EscapeMarkdownCode escapes backticks and backslashes in code blocks.
func EscapeMarkdownCode(text string) string {
	return mdCodeReplacer.Replace(text)
}

// EscapeMarkdownLinkUrl escapes closing parenthesis and backslashes in URLs.
func EscapeMarkdownLinkUrl(url string) string {
	return mdLinkURLReplacer.Replace(url)
}

// MarkdownBold returns text wrapped in bold MarkdownV2 markers.
func MarkdownBold(text string) string {
	return "*" + EscapeMd(text) + "*"
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
	token := "\u200BMDV2PH" + strconv.Itoa(len(s.placeholders)) + "\u200B"
	s.placeholders = append(s.placeholders, markdown)
	return token
}

// Restore replaces all placeholder tokens with their original content.
func (s *MarkdownPlaceholderStore) Restore(text string) string {
	restored := text
	for i, placeholder := range s.placeholders {
		token := "\u200BMDV2PH" + strconv.Itoa(i) + "\u200B"
		restored = strings.ReplaceAll(restored, token, placeholder)
	}
	return restored
}

// TrimCodeFencePadding removes a single trailing newline from code blocks.
func TrimCodeFencePadding(code string) string {
	return strings.TrimSuffix(code, "\n")
}
