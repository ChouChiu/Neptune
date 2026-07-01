package sessionctx

import (
	"context"
	"net/http"
)

type userIDKey struct{}

// WithUserID stores an authenticated admin user ID in the request context.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserID extracts the authenticated admin user ID from a request.
func UserID(r *http.Request) int64 {
	if v, ok := r.Context().Value(userIDKey{}).(int64); ok {
		return v
	}
	return 0
}
