// bff\internal\client\grpc\outgoing_metadata.go
package grpc

import (
	"context"
	"strings"

	"github.com/tagoKoder/bff/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	mdAuthorization  = "authorization"
	mdCorrelationID  = "x-correlation-id"
	mdRouteTemplate  = "x-route-template"
	mdForwardedFor   = "x-forwarded-for"
	mdUserAgent      = "x-user-agent"
	mdIdempotencyKey = "idempotency-key"
)

func ensureBearer(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return ""
	}
	low := strings.ToLower(tok)
	if strings.HasPrefix(low, "bearer ") {
		return tok
	}
	return "Bearer " + tok
}

func injectOutgoing(ctx context.Context) context.Context {
	var md metadata.MD
	if existing, ok := metadata.FromOutgoingContext(ctx); ok && existing != nil {
		md = existing.Copy()
	} else {
		md = metadata.New(nil)
	}

	if cid := security.CorrelationID(ctx); cid != "" {
		md.Set(mdCorrelationID, cid)
	}
	if rt := security.RouteTemplate(ctx); rt != "" {
		md.Set(mdRouteTemplate, rt)
	}
	if ip := security.ClientIP(ctx); ip != "" {
		md.Set(mdForwardedFor, ip)
	}
	if ua := security.UserAgent(ctx); ua != "" {
		md.Set(mdUserAgent, ua)
	}
	if ik := security.IdempotencyKey(ctx); ik != "" {
		md.Set(mdIdempotencyKey, ik)
	}

	if tok := ensureBearer(security.AccessToken(ctx)); tok != "" {
		md.Set(mdAuthorization, tok)
	}

	return metadata.NewOutgoingContext(ctx, md)
}

func OutgoingHeadersUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = injectOutgoing(ctx)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func OutgoingHeadersStreamInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx = injectOutgoing(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// Si ya lo tienes en conn.go encadenado, puedes dejarlo como NOOP para no duplicar.
// O eliminarlo del chain y quedarte solo con OutgoingHeaders*.
func ClientMetadataInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// ya inyecta OutgoingHeadersUnaryInterceptor
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
