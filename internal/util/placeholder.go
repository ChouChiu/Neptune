package util

import (
	"strconv"
	"strings"
)

// ReplacePlaceholders replaces placeholder tokens in text with provided values.
// Supported placeholders: {nickname}, {userid}, {groupname}
func ReplacePlaceholders(text string, nickname string, userID int64, groupName string) string {
	result := text
	if nickname != "" {
		result = strings.ReplaceAll(result, "{nickname}", nickname)
	}
	if userID != 0 {
		result = strings.ReplaceAll(result, "{userid}", strconv.FormatInt(userID, 10))
	}
	if groupName != "" {
		result = strings.ReplaceAll(result, "{groupname}", groupName)
	}
	return result
}