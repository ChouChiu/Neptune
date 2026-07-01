package handler

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestIsChatMemberJoin(t *testing.T) {
	tests := []struct {
		name      string
		oldStatus models.ChatMemberType
		newStatus models.ChatMemberType
		want      bool
	}{
		{
			name:      "left to member",
			oldStatus: models.ChatMemberTypeLeft,
			newStatus: models.ChatMemberTypeMember,
			want:      true,
		},
		{
			name:      "left to restricted",
			oldStatus: models.ChatMemberTypeLeft,
			newStatus: models.ChatMemberTypeRestricted,
			want:      true,
		},
		{
			name:      "member permission update",
			oldStatus: models.ChatMemberTypeMember,
			newStatus: models.ChatMemberTypeRestricted,
			want:      false,
		},
		{
			name:      "restricted permission update",
			oldStatus: models.ChatMemberTypeRestricted,
			newStatus: models.ChatMemberTypeMember,
			want:      false,
		},
		{
			name:      "left to banned",
			oldStatus: models.ChatMemberTypeLeft,
			newStatus: models.ChatMemberTypeBanned,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChatMemberJoin(tt.oldStatus, tt.newStatus); got != tt.want {
				t.Fatalf("IsChatMemberJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerificationPermissionsIncludeTelegramFields(t *testing.T) {
	restricted := restrictedChatPermissions()
	if restricted.CanEditTag || restricted.CanReactToMessages {
		t.Fatalf("restricted permissions should disable edit tag and reactions")
	}
}
