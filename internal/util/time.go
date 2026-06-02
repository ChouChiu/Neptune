package util

import "time"

// CurrentTimestamp returns the current Unix timestamp in seconds.
func CurrentTimestamp() int64 {
	return time.Now().Unix()
}