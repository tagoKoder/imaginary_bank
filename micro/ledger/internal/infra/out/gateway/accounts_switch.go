package gateway

import (
	"context"
	"sync/atomic"

	accountsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/accounts/v1"
	out "github.com/tagoKoder/ledger/internal/domain/port/out"
)

// OJO: atomic.Value exige mismo tipo dinámico siempre.
// Por eso guardamos SIEMPRE este holder (mismo tipo), y dentro va la interfaz.
type accountsHolder struct {
	impl out.AccountsGatewayPort
}

type AccountsGatewaySwitch struct {
	current atomic.Value // guarda accountsHolder SIEMPRE
}

func NewAccountsGatewaySwitch(initial out.AccountsGatewayPort) *AccountsGatewaySwitch {
	if initial == nil {
		initial = NewUnavailableAccountsGateway()
	}
	sw := &AccountsGatewaySwitch{}
	sw.current.Store(accountsHolder{impl: initial})
	return sw
}

func (s *AccountsGatewaySwitch) Set(next out.AccountsGatewayPort) {
	if next == nil {
		next = NewUnavailableAccountsGateway()
	}
	s.current.Store(accountsHolder{impl: next})
}

func (s *AccountsGatewaySwitch) impl() out.AccountsGatewayPort {
	v := s.current.Load()
	if v == nil {
		return NewUnavailableAccountsGateway()
	}
	return v.(accountsHolder).impl
}

// ---- Delegación ----

func (s *AccountsGatewaySwitch) ValidateAccountsAndLimits(ctx context.Context, req *accountsv1.ValidateAccountsAndLimitsRequest) (*accountsv1.ValidateAccountsAndLimitsResponse, error) {
	return s.impl().ValidateAccountsAndLimits(ctx, req)
}

func (s *AccountsGatewaySwitch) ReserveHold(ctx context.Context, req *accountsv1.ReserveHoldRequest) (*accountsv1.ReserveHoldResponse, error) {
	return s.impl().ReserveHold(ctx, req)
}

func (s *AccountsGatewaySwitch) ReleaseHold(ctx context.Context, req *accountsv1.ReleaseHoldRequest) (*accountsv1.ReleaseHoldResponse, error) {
	return s.impl().ReleaseHold(ctx, req)
}
