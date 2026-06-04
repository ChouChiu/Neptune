package handler

import "testing"

func TestCommandArgs(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		command string
		want    string
	}{
		{
			name:    "plain command with args",
			text:    "/setwelcome 欢迎 {nickname} 加入群组！",
			command: "/setwelcome",
			want:    "欢迎 {nickname} 加入群组！",
		},
		{
			name:    "addressed command with args",
			text:    "/setwelcome@neptunego_bot 欢迎 {nickname} 加入群组！",
			command: "/setwelcome",
			want:    "欢迎 {nickname} 加入群组！",
		},
		{
			name:    "addressed command without args",
			text:    "/setwelcome@neptunego_bot",
			command: "/setwelcome",
			want:    "",
		},
		{
			name:    "different command",
			text:    "/setwelcome2 hello",
			command: "/setwelcome",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandArgs(tt.text, tt.command); got != tt.want {
				t.Fatalf("commandArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
