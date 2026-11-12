// micro\ledger\internal\infra\out\gateway\accounts_http_gateway.go
package gateway

import (
	"context"
	"strings"
	"time"

	"github.com/sony/gobreaker"
	accountsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/accounts/v1"
	authctx "github.com/tagoKoder/ledger/internal/infra/security/context"
	"google.golang.org/grpc/metadata"
)

type AccountsGRPCGateway struct {
	client         accountsv1.InternalAccountsServiceClient
	timeout        time.Duration
	internalToken  string
	internalHeader string
	cb             *gobreaker.CircuitBreaker
}

func NewAccountsGRPCGateway(
	client accountsv1.InternalAccountsServiceClient,
	timeout time.Duration,
	internalToken string,
	internalHeader string,
) *AccountsGRPCGateway {
	st := gobreaker.Settings{
		Name:        "accounts-grpc",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 3 },
	}

	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if internalHeader == "" {
		internalHeader = "x-internal-token"
	}

	return &AccountsGRPCGateway{
		client:         client,
		timeout:        timeout,
		internalToken:  internalToken,
		internalHeader: internalHeader,
		cb:             gobreaker.NewCircuitBreaker(st),
	}
}

func (g *AccountsGRPCGateway) ValidateAccountsAndLimits(ctx context.Context, req *accountsv1.ValidateAccountsAndLimitsRequest) (*accountsv1.ValidateAccountsAndLimitsResponse, error) {
	out, err := g.cb.Execute(func() (any, error) {
		cctx, cancel := g.withOutgoingMetaAndTimeout(ctx)
		defer cancel()

		return g.client.ValidateAccountsAndLimits(cctx, req)
	})
	if err != nil {
		return nil, err
	}
	return out.(*accountsv1.ValidateAccountsAndLimitsResponse), nil
}

func (g *AccountsGRPCGateway) ReserveHold(ctx context.Context, req *accountsv1.ReserveHoldRequest) (*accountsv1.ReserveHoldResponse, error) {
	out, err := g.cb.Execute(func() (any, error) {
		cctx, cancel := g.withOutgoingMetaAndTimeout(ctx)
		defer cancel()

		return g.client.ReserveHold(cctx, req)
	})
	if err != nil {
		return nil, err
	}
	return out.(*accountsv1.ReserveHoldResponse), nil
}

func (g *AccountsGRPCGateway) ReleaseHold(ctx context.Context, req *accountsv1.ReleaseHoldRequest) (*accountsv1.ReleaseHoldResponse, error) {
	out, err := g.cb.Execute(func() (any, error) {
		cctx, cancel := g.withOutgoingMetaAndTimeout(ctx)
		defer cancel()

		return g.client.ReleaseHold(cctx, req)
	})
	if err != nil {
		return nil, err
	}
	return out.(*accountsv1.ReleaseHoldResponse), nil
}

func (g *AccountsGRPCGateway) withOutgoingMetaAndTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	md := metadata.MD{}

	// 1) Propagar Bearer (BFF-first): el micro reusa el token del usuario
	if tok := authctx.AccessToken(ctx); tok != "" {
		md.Append("authorization", "Bearer "+tok)
	}

	// 2) Correlation ID
	if cid := authctx.CorrelationID(ctx); cid != "" {
		md.Append("x-correlation-id", cid)
	}

	// 3) Idempotency Key (si está en ctx)
	if ik := authctx.IdempotencyKey(ctx); ik != "" {
		md.Append("idempotency-key", ik)
	}

	// 4) Header interno opcional (si lo mantienes)
	if g.internalToken != "" {
		h := sanitizeMetadataKey(g.internalHeader)
		md.Append(h, g.internalToken)
	}

	outCtx := metadata.NewOutgoingContext(ctx, md)
	return context.WithTimeout(outCtx, g.timeout)
}

func (g *AccountsGRPCGateway) BatchGetAccountSummaries(ctx context.Context, req *accountsv1.BatchGetAccountSummariesRequest) (*accountsv1.BatchGetAccountSummariesResponse, error) {
	return g.client.BatchGetAccountSummaries(ctx, req)
}

func sanitizeMetadataKey(k string) string {
	k = strings.TrimSpace(strings.ToLower(k))
	k = strings.ReplaceAll(k, "_", "-")
	if k == "" {
		return "x-internal-token"
	}
	// gRPC metadata keys deben ser lowercase ASCII
	for _, r := range k {
		if r < 0x21 || r > 0x7E {
			return "x-internal-token"
		}
	}
	return k
}
