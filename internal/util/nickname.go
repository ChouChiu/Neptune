package util

import "github.com/go-telegram/bot/models"

// GetNickname returns a display name for the user.
// If the user has a last name, it returns "FirstName LastName".
// Otherwise, it returns just the first name.
func GetNickname(user *models.User) string {
	if user.LastName != "" {
		return user.FirstName + " " + user.LastName
	}
	return user.FirstName
}
