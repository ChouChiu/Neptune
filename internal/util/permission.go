package util

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ChouChiu/neptune/internal/db"
)

// CheckAdminPermission checks if a user has admin permission.
// In groups: checks if the user is a Telegram administrator or creator.
// In private chat: checks if the user has a connected admin group.
// Returns (allowed, groupId).
func CheckAdminPermission(ctx context.Context, b *bot.Bot, db *db.DB, msg *models.Message) (bool, int64) {
	if msg == nil || msg.From == nil {
		return false, 0
	}
	userID := msg.From.ID
	chatID := msg.Chat.ID

	// Private chat: check if admin has a connected group
	if msg.Chat.Type == "private" {
		groupID, err := db.GetAdminGroupID(userID)
		if err != nil || groupID == nil {
			return false, 0
		}
		return true, *groupID
	}

	// Group chat: check if user is Telegram admin
	member, err := b.GetChatMember(ctx, &bot.GetChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		return false, 0
	}

	// Check if user is administrator or creator
	if member.Type == models.ChatMemberTypeAdministrator || member.Type == models.ChatMemberTypeOwner {
		return true, chatID
	}

	return false, 0
}

// CheckGroupAdmin checks if a user is an admin connected to a specific group.
func CheckGroupAdmin(db *db.DB, userID int64, groupID int64) bool {
	result, err := db.IsAdminConnected(userID, groupID)
	if err != nil {
		return false
	}
	return result
}