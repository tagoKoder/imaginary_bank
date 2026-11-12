package gateway

import (
	"context"

	accountsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/accounts/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnavailableAccountsGateway devuelve UNAVAILABLE en todas las llamadas.
// Sirve para que el micro levante aunque Accounts todavía no esté listo.
type UnavailableAccountsGateway struct {
	msg string
}

func NewUnavailableAccountsGateway() *UnavailableAccountsGateway {
	return &UnavailableAccountsGateway{msg: "accounts gateway not ready"}
}

func (g *UnavailableAccountsGateway) ValidateAccountsAndLimits(ctx context.Context, req *accountsv1.ValidateAccountsAndLimitsRequest) (*accountsv1.ValidateAccountsAndLimitsResponse, error) {
	return nil, status.Error(codes.Unavailable, g.msg)
}

func (g *UnavailableAccountsGateway) ReserveHold(ctx context.Context, req *accountsv1.ReserveHoldRequest) (*accountsv1.ReserveHoldResponse, error) {
	return nil, status.Error(codes.Unavailable, g.msg)
}

func (g *UnavailableAccountsGateway) ReleaseHold(ctx context.Context, req *accountsv1.ReleaseHoldRequest) (*accountsv1.ReleaseHoldResponse, error) {
	return nil, status.Error(codes.Unavailable, g.msg)
}

func (g *UnavailableAccountsGateway) BatchGetAccountSummaries(ctx context.Context, req *accountsv1.BatchGetAccountSummariesRequest) (*accountsv1.BatchGetAccountSummariesResponse, error) {
	return nil, status.Error(codes.Unavailable, g.msg)
}
