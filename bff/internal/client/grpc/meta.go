// bff\internal\client\grpc\meta.go
package grpc

import (
	"context"

	"google.golang.org/grpc/metadata"
)

type CallMeta struct {
	RequestID string
	Subject   string
	SessionID string
}

const (
	mdRequestID = "x-request-id"
	mdSubject   = "x-subject"
	mdSessionID = "x-session-id"
)

func withMeta(ctx context.Context, m CallMeta) context.Context {
	if m.RequestID == "" && m.Subject == "" && m.SessionID == "" {
		return ctx
	}
	pairs := []string{}
	if m.RequestID != "" {
		pairs = append(pairs, mdRequestID, m.RequestID)
	}
	if m.Subject != "" {
		pairs = append(pairs, mdSubject, m.Subject)
	}
	if m.SessionID != "" {
		pairs = append(pairs, mdSessionID, m.SessionID)
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}
