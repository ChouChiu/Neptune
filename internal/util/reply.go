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

// CommandArgs extracts the arguments after a command, stripping the bot username.
// For example, "/report@botname reason" → "reason", "/report reason" → "reason".
// Returns empty string if no args.
func CommandArgs(text, cmd string) string {
	if len(text) <= len(cmd) {
		return ""
	}
	rest := text[len(cmd):]
	// Strip @botname suffix: "/report@botname args" → "@botname args"
	if idx := strings.Index(rest, " "); idx >= 0 {
		// Has space after @botname: skip the @botname part
		rest = rest[idx+1:]
	} else if strings.HasPrefix(rest, "@") {
		// Only @botname, no args
		return ""
	}
	return strings.TrimSpace(rest)
}
