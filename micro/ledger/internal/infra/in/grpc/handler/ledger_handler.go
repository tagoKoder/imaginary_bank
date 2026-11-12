package handler

import (
	"context"

	ledgerpb "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	in "github.com/tagoKoder/ledger/internal/domain/port/in"
	"github.com/tagoKoder/ledger/internal/infra/in/grpc/mapper"
	"github.com/tagoKoder/ledger/internal/infra/in/grpc/validation"
)

type LedgerHandler struct {
	ledgerpb.UnimplementedLedgerServiceServer
	creaAcc  in.LedgerAppService
	listAcc  in.ListAccountJournalEntriesUseCase
	listStmt in.ListAccountStatementUseCase
}

func NewLedgerHandler(creaAcc in.LedgerAppService, listAcc in.ListAccountJournalEntriesUseCase, listStmt in.ListAccountStatementUseCase) *LedgerHandler {
	return &LedgerHandler{creaAcc: creaAcc, listAcc: listAcc, listStmt: listStmt}
}

func (h *LedgerHandler) CreditAccount(ctx context.Context, req *ledgerpb.CreditAccountRequest) (*ledgerpb.CreditAccountResponse, error) {
	cmd, err := validation.ToCreditAccountCommand(req)
	if err != nil {
		return nil, err
	}

	res, err := h.creaAcc.CreditAccount(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return mapper.ToCreditAccountResponse(res), nil
}

func (h *LedgerHandler) ListAccountJournalEntries(ctx context.Context, req *ledgerpb.ListAccountJournalEntriesRequest) (*ledgerpb.ListAccountJournalEntriesResponse, error) {
	q, err := validation.ToListAccountJournalEntriesQuery(req)
	if err != nil {
		return nil, err
	}

	res, err := h.listAcc.ListAccountJournalEntries(ctx, q)
	if err != nil {
		return nil, err
	}

	return mapper.ToListAccountJournalEntriesResponse(res), nil
}

func (h *LedgerHandler) ListAccountStatement(ctx context.Context, req *ledgerpb.ListAccountStatementRequest) (*ledgerpb.ListAccountStatementResponse, error) {
	q, err := validation.ToListAccountStatementQuery(req)
	if err != nil {
		return nil, err
	}

	res, err := h.listStmt.ListAccountStatement(ctx, q)
	if err != nil {
		return nil, err
	}

	return mapper.ToListAccountStatementResponse(res), nil
}

func (h *LedgerHandler) CreditAccountSystem(ctx context.Context, req *ledgerpb.CreditAccountSystemRequest) (*ledgerpb.CreditAccountSystemResponse, error) {
	cmd, err := validation.ToCreditAccountSystemCommand(req)
	if err != nil {
		return nil, err
	}

	res, err := h.creaAcc.CreditAccountSystem(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return mapper.ToCreditAccountSystemResponse(res), nil
}
