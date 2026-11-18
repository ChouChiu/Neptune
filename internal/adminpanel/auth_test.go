package adminpanel

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ChouChiu/neptune/internal/db"
)

func signTelegramLoginData(botToken string, data map[string]string) map[string]string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, data[k]))
	}
	checkString := strings.Join(parts, "\n")
	secretKey := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secretKey[:])
	mac.Write([]byte(checkString))

	out := make(map[string]string, len(data)+1)
	for k, v := range data {
		out[k] = v
	}
	out["hash"] = hex.EncodeToString(mac.Sum(nil))
	return out
}

func TestVerifyTelegramAuth(t *testing.T) {
	const botToken = "123456:test-token"
	now := time.Now().Unix()

	valid := signTelegramLoginData(botToken, map[string]string{
		"id":         "42",
		"first_name": "Neptune",
		"auth_date":  strconv.FormatInt(now, 10),
	})
	if !VerifyTelegramAuth(botToken, valid) {
		t.Fatalf("VerifyTelegramAuth() = false for valid data")
	}

	tampered := signTelegramLoginData(botToken, map[string]string{
		"id":         "42",
		"first_name": "Neptune",
		"auth_date":  strconv.FormatInt(now, 10),
	})
	tampered["id"] = "99"
	if VerifyTelegramAuth(botToken, tampered) {
		t.Fatalf("VerifyTelegramAuth() = true for tampered id")
	}

	expired := signTelegramLoginData(botToken, map[string]string{
		"id":        "42",
		"auth_date": strconv.FormatInt(now-int64(telegramAuthMaxAge.Seconds())-60, 10),
	})
	if VerifyTelegramAuth(botToken, expired) {
		t.Fatalf("VerifyTelegramAuth() = true for expired auth_date")
	}

	future := signTelegramLoginData(botToken, map[string]string{
		"id":        "42",
		"auth_date": strconv.FormatInt(now+600, 10),
	})
	if VerifyTelegramAuth(botToken, future) {
		t.Fatalf("VerifyTelegramAuth() = true for future auth_date")
	}
}

func TestSignAndVerifySession(t *testing.T) {
	const botToken = "123456:test-token"
	expiresAt := time.Now().Add(time.Hour).Unix()

	token := SignSession(botToken, 777, expiresAt)
	if got := VerifySession(botToken, token); got != 777 {
		t.Fatalf("VerifySession() = %d, want 777", got)
	}

	// Tampered signature.
	if got := VerifySession(botToken, token+"x"); got != 0 {
		t.Fatalf("VerifySession(tampered) = %d, want 0", got)
	}

	// Expired session.
	expiredToken := SignSession(botToken, 777, time.Now().Add(-time.Minute).Unix())
	if got := VerifySession(botToken, expiredToken); got != 0 {
		t.Fatalf("VerifySession(expired) = %d, want 0", got)
	}

	// Malformed session.
	if got := VerifySession(botToken, "invalid"); got != 0 {
		t.Fatalf("VerifySession(malformed) = %d, want 0", got)
	}
}

func TestResolveReportRejectsDuplicateResolution(t *testing.T) {
	t.Chdir("../..")
	database, err := db.New(filepath.Join(t.TempDir(), "admin_test.db"))
	if err != nil {
		t.Fatalf("db.New() error = %v", err)
	}
	defer func() { _ = database.Close() }()
	if err := database.InitDatabase(); err != nil {
		t.Fatalf("InitDatabase() error = %v", err)
	}

	var adminID int64 = 1001
	const groupID int64 = -100123
	if err := database.ConnectAdmin(adminID, groupID); err != nil {
		t.Fatalf("ConnectAdmin() error = %v", err)
	}
	if err := database.AddReport(groupID, 2001, 2002, nil, "bad msg", "spam"); err != nil {
		t.Fatalf("AddReport() error = %v", err)
	}
	reports, err := database.GetReports(nil, &adminID)
	if err != nil || len(reports) != 1 {
		t.Fatalf("GetReports() failed: len=%d err=%v", len(reports), err)
	}

	// Mark report already resolved to verify ResolveReport returns 409 Conflict without side effects.
	if _, err := database.ResolvePendingReport(reports[0].ID, "dismissed", adminID); err != nil {
		t.Fatalf("ResolvePendingReport() error = %v", err)
	}

	const botToken = "123456:test-token"
	srv := NewServer(database, botToken, "testbot", NewLogCollector(10))
	session := SignSession(botToken, adminID, time.Now().Add(time.Hour).Unix())

	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/reports/%d/resolve", reports[0].ID),
		bytes.NewBufferString(`{"status":"approved"}`),
	)
	req.AddCookie(&http.Cookie{Name: "nep_session", Value: session})
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("ResolveReport status = %d, want %d (body: %s)", rec.Code, http.StatusConflict, rec.Body.String())
	}
}
