package handler

import (
	"testing"

	"github.com/ChouChiu/neptune/internal/model"
)

func TestMatchKeywordRegex(t *testing.T) {
	re, err := compileKeywordRegex(`foo\d+`)
	if err != nil {
		t.Fatalf("compileKeywordRegex() error = %v", err)
	}

	rule := model.KeywordRule{
		ID:           1,
		GroupID:      -100,
		Pattern:      `foo\d+`,
		IsRegex:      1,
		ReplyContent: "matched",
	}
	keywords := []compiledKeywordRule{{
		rule:  rule,
		regex: re,
	}}

	if got := matchKeyword(keywords, "hello FOO42"); got == nil || got.ID != rule.ID {
		t.Fatalf("matchKeyword() did not match regex rule")
	}
	if got := matchKeyword(keywords, `hello foo\d+`); got != nil {
		t.Fatalf("matchKeyword() matched literal regex text: %#v", got)
	}
}
