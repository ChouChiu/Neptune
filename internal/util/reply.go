package util

import "github.com/go-telegram/bot/models"

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