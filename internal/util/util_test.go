package util

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestGenerateCaptcha(t *testing.T) {
	res := GenerateCaptcha(5)
	if len(res.Text) != 5 {
		t.Fatalf("captcha length = %d, want 5", len(res.Text))
	}
	for i := 0; i < len(res.Text); i++ {
		if !strings.ContainsRune(captchaChars, rune(res.Text[i])) {
			t.Fatalf("unexpected captcha char %q", res.Text[i])
		}
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(res.PNG))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
	if cfg.Width != 560 || cfg.Height != 460 {
		t.Fatalf("captcha dimensions = %dx%d, want 560x460", cfg.Width, cfg.Height)
	}
	if empty := GenerateCaptcha(0); empty.Text != "" || len(empty.PNG) != 0 {
		t.Fatalf("GenerateCaptcha(0) = %+v, want empty", empty)
	}
}

func TestCommandArgs(t *testing.T) {
	tests := []struct {
		text string
		cmd  string
		want string
	}{
		{"/warn spam", "/warn", "spam"},
		{"/warn@neptune_bot spam link", "/warn", "spam link"},
		{"/rule@neptune_bot\n1. Be nice", "/rule", "1. Be nice"},
		{"/warn@neptune_bot", "/warn", ""},
		{"/warning hello", "/warn", ""},
	}

	for _, tt := range tests {
		if got := CommandArgs(tt.text, tt.cmd); got != tt.want {
			t.Fatalf("CommandArgs(%q, %q) = %q, want %q", tt.text, tt.cmd, got, tt.want)
		}
	}
}

func TestReplacePlaceholders(t *testing.T) {
	tpl := "Hi {nickname} ({userid}) in {groupname}"
	if got := ReplacePlaceholders(tpl, "Nep", 123, "Planeptune", false); got != "Hi Nep (123) in Planeptune" {
		t.Fatalf("plain = %q", got)
	}

	escapedTpl := EscapeMd("Hi {nickname} ({userid}) in {groupname}!")
	got := ReplacePlaceholders(escapedTpl, EscapeMd("Nep_Bot"), 123, EscapeMd("Planeptune (Main)"), true)
	want := `Hi [Nep\_Bot](tg://user?id=123) \(123\) in Planeptune \(Main\)\!`
	if got != want {
		t.Fatalf("escaped markdown = %q, want %q", got, want)
	}
}

func TestMarkdownAndChineseHelpers(t *testing.T) {
	escaped := EscapeMd(`_ * [ ] ( ) ~ ` + "`" + ` > # + - = | { } . ! \`)
	want := `\_ \* \[ \] \( \) \~ \` + "`" + ` \> \# \+ \- \= \| \{ \} \. \! \\`
	if escaped != want {
		t.Fatalf("EscapeMd() = %q, want %q", escaped, want)
	}

	if got := MarkdownLink("click.here", "https://example.com/(test)"); got != `[click\.here](https://example.com/(test\))` {
		t.Fatalf("MarkdownLink() = %q", got)
	}

	store := NewMarkdownPlaceholderStore()
	protected := store.Protect("*bold*")
	if got := store.Restore(EscapeMd("hello " + protected + "!")); got != `hello *bold*\!` {
		t.Fatalf("Restore() = %q", got)
	}

	if got := ToSimplified("學習繁體中文"); got != "学习繁体中文" {
		t.Fatalf("ToSimplified() = %q", got)
	}
}
