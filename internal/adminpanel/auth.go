package adminpanel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const sessionTTL = 86400 // 24 hours

// VerifyTelegramAuth verifies the Telegram Login Widget data using HMAC-SHA256.
// The secret_key is SHA256(bot_token), and the signature is HMAC(secret_key, dataCheckString).
func VerifyTelegramAuth(botToken string, data map[string]string) bool {
	hash, ok := data["hash"]
	if !ok || hash == "" {
		return false
	}

	// Build data_check_string: sorted key=value pairs (excluding "hash")
	var keys []string
	for k := range data {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, data[k]))
	}
	dataCheckString := strings.Join(parts, "\n")

	// secret_key = SHA256(bot_token)
	secretHash := sha256.Sum256([]byte(botToken))

	// HMAC-SHA256(secret_key, data_check_string)
	mac := hmac.New(sha256.New, secretHash[:])
	mac.Write([]byte(dataCheckString))
	computed := hex.EncodeToString(mac.Sum(nil))

	return timingSafeEqual(computed, hash)
}

// SignSession creates a signed session token: userId:expiresAt:hmac_signature.
func SignSession(botToken string, userID int64, expiresAt int64) string {
	payload := fmt.Sprintf("%d:%d", userID, expiresAt)
	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s:%s", payload, sig)
}

// VerifySession verifies a session token and returns the user ID if valid.
// Returns 0 if the session is invalid or expired.
func VerifySession(botToken string, cookie string) int64 {
	parts := strings.SplitN(cookie, ":", 3)
	if len(parts) != 3 {
		return 0
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0
	}
	expiresAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0
	}

	// Check expiration
	if time.Now().Unix() > expiresAt {
		return 0
	}

	// Verify signature
	payload := fmt.Sprintf("%s:%s", parts[0], parts[1])
	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !timingSafeEqual(expected, parts[2]) {
		return 0
	}

	return userID
}

// timingSafeEqual performs a constant-time string comparison.
func timingSafeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
