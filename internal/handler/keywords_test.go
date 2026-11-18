package handler

import (
	"testing"

	"github.com/ChouChiu/neptune/internal/model"
)

func TestMatchKeyword(t *testing.T) {
	reFoo, err := compileKeywordRegex(`foo\d+`)
	if err != nil {
		t.Fatalf("compileKeywordRegex() error = %v", err)
	}
	reDev, err := compileKeywordRegex("開發")
	if err != nil {
		t.Fatalf("compileKeywordRegex() error = %v", err)
	}
	reDevSimp, err := compileKeywordRegex("开发")
	if err != nil {
		t.Fatalf("compileKeywordRegex(simplified) error = %v", err)
	}

	keywords := []compiledKeywordRule{
		{
			rule:  model.KeywordRule{ID: 1, Pattern: `foo\d+`, IsRegex: 1},
			regex: reFoo,
		},
		{
			rule:              model.KeywordRule{ID: 2, Pattern: "學習", IsRegex: 0},
			simplifiedPattern: "学习",
		},
		{
			rule:              model.KeywordRule{ID: 3, Pattern: "開發", IsRegex: 1},
			simplifiedPattern: "开发",
			regex:             reDev,
			simplifiedRegex:   reDevSimp,
		},
	}

	if got := matchKeyword(keywords, "hello FOO42"); got == nil || got.ID != 1 {
		t.Fatalf("expected regex rule 1, got %#v", got)
	}
	if got := matchKeyword(keywords, `hello foo\d+`); got != nil {
		t.Fatalf("literal regex text should not match, got %#v", got)
	}
	if got := matchKeyword(keywords, "我在学习Go"); got == nil || got.ID != 2 {
		t.Fatalf("expected plain simplified match 2, got %#v", got)
	}
	if got := matchKeyword(keywords, "正在开发新功能"); got == nil || got.ID != 3 {
		t.Fatalf("expected regex simplified match 3, got %#v", got)
	}
}

func TestSplitCommandKeyAndReply(t *testing.T) {
	tests := []struct {
		args      string
		wantKey   string
		wantReply string
		wantOK    bool
	}{
		{"hello world!", "hello", "world!", true},
		{"hello\nline1\nline2", "hello", "line1\nline2", true},
		{"hello   ", "", "", false},
		{"", "", "", false},
	}

	for _, tt := range tests {
		key, reply, ok := splitCommandKeyAndReply(tt.args)
		if ok != tt.wantOK || key != tt.wantKey || reply != tt.wantReply {
			t.Fatalf("splitCommandKeyAndReply(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.args, key, reply, ok, tt.wantKey, tt.wantReply, tt.wantOK)
		}
	}
}
