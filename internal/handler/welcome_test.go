package handler

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestIsChatMemberJoin(t *testing.T) {
	tests := []struct {
		name string
		old  models.ChatMemberType
		new  models.ChatMemberType
		want bool
	}{
		{"left to member", models.ChatMemberTypeLeft, models.ChatMemberTypeMember, true},
		{"left to restricted", models.ChatMemberTypeLeft, models.ChatMemberTypeRestricted, true},
		{"member permission update", models.ChatMemberTypeMember, models.ChatMemberTypeRestricted, false},
		{"left to banned", models.ChatMemberTypeLeft, models.ChatMemberTypeBanned, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChatMemberJoin(tt.old, tt.new); got != tt.want {
				t.Fatalf("IsChatMemberJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}
