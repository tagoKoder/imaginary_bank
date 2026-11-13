// bff\internal\util\trace.go
package util

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

func NewTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Background() context.Context { return context.Background() }

func HasDeadline(ctx context.Context) bool {
	_, ok := ctx.Deadline()
	return ok
}
