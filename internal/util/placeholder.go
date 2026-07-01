package util

import (
	"strconv"
	"strings"
)

// ReplacePlaceholders replaces placeholder tokens in text with provided values.
// When markdown is true, {nickname} becomes a clickable MarkdownV2 user link.
// Supported placeholders: {nickname}, {userid}, {groupname}
func ReplacePlaceholders(text string, nickname string, userID int64, groupName string, markdown bool) string {
	result := text
	if nickname != "" {
		if markdown && userID != 0 {
			result = strings.ReplaceAll(result, "{nickname}",
				"["+nickname+"](tg://user?id="+strconv.FormatInt(userID, 10)+")")
		} else {
			result = strings.ReplaceAll(result, "{nickname}", nickname)
		}
	}
	if userID != 0 {
		result = strings.ReplaceAll(result, "{userid}", strconv.FormatInt(userID, 10))
	}
	if groupName != "" {
		result = strings.ReplaceAll(result, "{groupname}", groupName)
	}
	return result
}
