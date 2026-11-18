package util

import (
	"strings"

	"github.com/go-telegram/bot/models"
)

// ReplyOptions returns reply parameters for replying to a message.
// If the message is nil, returns nil.
func ReplyOptions(msg *models.Message) *models.ReplyParameters {
	if msg == nil {
		return nil
	}
	return &models.ReplyParameters{
		MessageID: msg.ID,
	}
}

// CommandArgs extracts the arguments after a command, stripping any @botname suffix.
// For example, "/report@botname reason" → "reason", "/report reason" → "reason".
// Returns empty string if the command does not match or has no args.
func CommandArgs(text, command string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	name := fields[0]
	if name != command && !strings.HasPrefix(name, command+"@") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(text, name))
}
