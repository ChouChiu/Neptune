package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"action":"published"}`)
	secret := "super-secret"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !VerifySignature(body, validSig, secret) {
		t.Fatalf("VerifySignature() = false for valid signature")
	}
	if VerifySignature(body, "sha256=deadbeef", secret) {
		t.Fatalf("VerifySignature() = true for invalid signature")
	}
	if VerifySignature(body, "", secret) {
		t.Fatalf("VerifySignature() = true for empty signature")
	}
}

func TestConvertGfmToMarkdownV2(t *testing.T) {
	gfm := "## What's New\r\n> [!NOTE]\r\n> Important note!\r\n- Fix bug (#1)\r\nCheck [docs](https://example.com/a)b) and `code`."
	got := ConvertGfmToMarkdownV2(gfm)

	if strings.Contains(got, "[!NOTE]") {
		t.Fatalf("callout marker was not stripped: %q", got)
	}
	if !strings.Contains(got, "*What's New*") {
		t.Fatalf("heading not converted to bold: %q", got)
	}
	if !strings.Contains(got, "⦁ Fix bug") {
		t.Fatalf("list marker not converted: %q", got)
	}
	if !strings.Contains(got, "`code`") {
		t.Fatalf("inline code lost: %q", got)
	}
}

func TestFormatReleaseMessageUTF8AndLengthSafety(t *testing.T) {
	longBody := strings.Repeat("🎉测试发布说明._-! ", 600)
	name := "v2.0.0 for OHOS"
	rel := &GitHubRelease{
		Action: "published",
		Release: ReleaseDetails{
			TagName: "v2.0.0",
			Name:    &name,
			Body:    &longBody,
			HTMLURL: "https://github.com/example/repo/releases/tag/v2.0.0",
		},
	}

	msg := FormatReleaseMessage(rel)
	if !utf8.ValidString(msg) {
		t.Fatalf("FormatReleaseMessage produced invalid UTF-8")
	}
	if utf8.RuneCountInString(msg) > maxMessageLength {
		t.Fatalf("message length %d exceeds max %d", utf8.RuneCountInString(msg), maxMessageLength)
	}
	if !strings.Contains(msg, "for OHOS 已发布") {
		t.Fatalf("expected OHOS header in %q", msg)
	}
	if !strings.Contains(msg, "Release 页面") {
		t.Fatalf("expected release link in %q", msg)
	}
}
