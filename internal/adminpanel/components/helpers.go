package components

import (
	"fmt"
	"time"
)

// FormatTimeAgo returns a human-readable time ago string from a Unix timestamp.
func FormatTimeAgo(ts int64) string {
	diff := time.Now().Unix() - ts
	switch {
	case diff < 0:
		return "刚刚"
	case diff < 60:
		return fmt.Sprintf("%d秒前", diff)
	case diff < 3600:
		return fmt.Sprintf("%d分钟前", diff/60)
	case diff < 86400:
		return fmt.Sprintf("%d小时前", diff/3600)
	default:
		return fmt.Sprintf("%d天前", diff/86400)
	}
}
