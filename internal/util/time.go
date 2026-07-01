package util

import (
	"crypto/rand"
	"time"
)

// CurrentTimestamp returns the current Unix timestamp in seconds.
func CurrentTimestamp() int64 {
	return time.Now().Unix()
}

// RandomString generates a random alphanumeric string of the given length.
func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[b[i]%byte(len(letters))]
	}
	return string(b)
}
