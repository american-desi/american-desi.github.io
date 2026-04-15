package generator

import (
	"context"
	"time"
)

// newBackground returns a context with a timeout, detached from the caller.
func newBackground(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
