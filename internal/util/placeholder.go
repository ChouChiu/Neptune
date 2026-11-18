package util

import (
	"strconv"
	"strings"
)

// ReplacePlaceholders replaces placeholder tokens in text with provided values.
// When markdown is true, {nickname} becomes a clickable MarkdownV2 user link,
// and pre-escaped placeholder tokens (from EscapeMd) are also matched.
// Supported placeholders: {nickname}, {userid}, {groupname}
func ReplacePlaceholders(text string, nickname string, userID int64, groupName string, markdown bool) string {
	result := text
	if nickname != "" {
		replacement := nickname
		if markdown && userID != 0 {
			replacement = "[" + nickname + "](tg://user?id=" + strconv.FormatInt(userID, 10) + ")"
		}
		result = strings.ReplaceAll(result, "{nickname}", replacement)
		if markdown {
			result = strings.ReplaceAll(result, `\{nickname\}`, replacement)
		}
	}
	if userID != 0 {
		uid := strconv.FormatInt(userID, 10)
		result = strings.ReplaceAll(result, "{userid}", uid)
		if markdown {
			result = strings.ReplaceAll(result, `\{userid\}`, uid)
		}
	}
	if groupName != "" {
		result = strings.ReplaceAll(result, "{groupname}", groupName)
		if markdown {
			result = strings.ReplaceAll(result, `\{groupname\}`, groupName)
		}
	}
	return result
}
