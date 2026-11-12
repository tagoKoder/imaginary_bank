// micro\ledger\internal\infra\in\grpc\mapper\mapper.go
package mapper

import (
	"time"

	"github.com/google/uuid"
	ledgerpb "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	"github.com/tagoKoder/ledger/internal/domain/port/in"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ToPostPaymentCommand(req *ledgerpb.PostPaymentRequest) (in.PostPaymentCommand, error) {
	src, err := uuid.Parse(req.GetSourceAccountId())
	if err != nil {
		return in.PostPaymentCommand{}, err
	}
	dst, err := uuid.Parse(req.GetDestinationAccountId())
	if err != nil {
		return in.PostPaymentCommand{}, err
	}

	return in.PostPaymentCommand{
		IdempotencyKey:     req.GetIdempotencyKey(),
		SourceAccountID:    src,
		DestinationAccount: dst,
		Currency:           req.GetCurrency(),
		Amount:             req.GetAmount(),
		InitiatedBy:        req.GetInitiatedBy(),
	}, nil
}

func ToPostPaymentResponse(res *in.PostPaymentResult) *ledgerpb.PostPaymentResponse {
	return &ledgerpb.PostPaymentResponse{
		PaymentId: res.PaymentID.String(),
		Status:    res.Status,
	}
}

func ToGetPaymentID(req *ledgerpb.GetPaymentRequest) (uuid.UUID, error) {
	return uuid.Parse(req.GetPaymentId())
}

func ToGetPaymentResponse(res *in.GetPaymentResult) (*ledgerpb.GetPaymentResponse, error) {
	createdAt, _ := time.Parse(time.RFC3339Nano, res.CreatedAtRFC3339)
	out := &ledgerpb.GetPaymentResponse{
		PaymentId:            res.PaymentID.String(),
		Status:               res.Status,
		SourceAccountId:      res.SourceAccountID.String(),
		DestinationAccountId: res.DestAccountID.String(),
		Currency:             res.Currency,
		Amount:               res.Amount,
		CreatedAt:            timestamppb.New(createdAt),
	}
	for _, s := range res.Steps {
		t, _ := time.Parse(time.RFC3339Nano, s.AttemptedAtRFC3339)
		out.Steps = append(out.Steps, &ledgerpb.PaymentStep{
			Step:        s.Step,
			State:       s.State,
			DetailsJson: s.DetailsJSON,
			AttemptedAt: timestamppb.New(t),
		})
	}
	return out, nil
}

func ToCreditAccountCommand(req *ledgerpb.CreditAccountRequest) (in.CreditAccountCommand, error) {
	acc, err := uuid.Parse(req.GetAccountId())
	if err != nil {
		return in.CreditAccountCommand{}, err
	}

	return in.CreditAccountCommand{
		IdempotencyKey: req.GetIdempotencyKey(),
		AccountID:      acc,
		Currency:       req.GetCurrency(),
		Amount:         req.GetAmount(),
		InitiatedBy:    req.GetInitiatedBy(),
		ExternalRef:    req.GetExternalRef(),
	}, nil
}

func ToCreditAccountResponse(res *in.CreditAccountResult) *ledgerpb.CreditAccountResponse {
	return &ledgerpb.CreditAccountResponse{
		JournalId: res.JournalID.String(),
		Status:    res.Status,
	}
}

func ToListAccountJournalEntriesQuery(req *ledgerpb.ListAccountJournalEntriesRequest) (in.ListAccountJournalEntriesQuery, error) {
	acc, err := uuid.Parse(req.GetAccountId())
	if err != nil {
		return in.ListAccountJournalEntriesQuery{}, err
	}

	from := ""
	to := ""
	if req.From != nil {
		from = req.From.AsTime().UTC().Format(time.RFC3339Nano)
	}
	if req.To != nil {
		to = req.To.AsTime().UTC().Format(time.RFC3339Nano)
	}

	return in.ListAccountJournalEntriesQuery{
		AccountID:   acc,
		FromRFC3339: from,
		ToRFC3339:   to,
		Page:        int(req.GetPage()),
		Size:        int(req.GetSize()),
	}, nil
}

func ToListAccountJournalEntriesResponse(res *in.ListAccountJournalEntriesResult) *ledgerpb.ListAccountJournalEntriesResponse {
	out := &ledgerpb.ListAccountJournalEntriesResponse{Page: int32(res.Page), Size: int32(res.Size)}
	for _, e := range res.Entries {
		bt, _ := time.Parse(time.RFC3339Nano, e.BookedAtRFC3339)
		je := &ledgerpb.JournalEntryView{
			JournalId:   e.JournalID,
			Currency:    e.Currency,
			Status:      e.Status,
			BookedAt:    timestamppb.New(bt),
			ExternalRef: e.ExternalRef,
		}
		for _, l := range e.Lines {
			je.Lines = append(je.Lines, &ledgerpb.JournalLine{
				GlAccountCode:   l.GLAccountCode,
				CounterpartyRef: l.CounterpartyRef,
				Debit:           l.Debit,
				Credit:          l.Credit,
			})
		}
		out.Entries = append(out.Entries, je)
	}
	return out
}

func ToListAccountStatementQuery(req *ledgerpb.ListAccountStatementRequest) (in.ListAccountStatementQuery, error) {
	acc, err := uuid.Parse(req.GetAccountId())
	if err != nil {
		return in.ListAccountStatementQuery{}, err
	}

	from := time.Time{}
	to := time.Time{}
	if req.From != nil {
		from = req.From.AsTime().UTC()
	}
	if req.To != nil {
		to = req.To.AsTime().UTC()
	}

	return in.ListAccountStatementQuery{
		AccountID:           acc,
		From:                from,
		To:                  to,
		Page:                int(req.GetPage()),
		Size:                int(req.GetSize()),
		IncludeCounterparty: req.GetIncludeCounterparty(),
	}, nil
}

func ToListAccountStatementResponse(res *in.ListAccountStatementResult) *ledgerpb.ListAccountStatementResponse {
	out := &ledgerpb.ListAccountStatementResponse{
		Page: int32(res.Page),
		Size: int32(res.Size),
	}

	for _, it := range res.Items {
		pbIt := &ledgerpb.StatementItem{
			JournalId:             it.JournalID,
			BookedAt:              timestamppb.New(it.BookedAt),
			Currency:              it.Currency,
			Direction:             it.Direction,
			Amount:                it.Amount,
			Kind:                  it.Kind,
			Memo:                  it.Memo,
			CounterpartyAccountId: it.CounterpartyAccountID,
		}
		if it.Counterparty != nil {
			pbIt.Counterparty = &ledgerpb.CounterpartyView{
				AccountId:     it.Counterparty.AccountID,
				AccountNumber: it.Counterparty.AccountNumber,
				DisplayName:   it.Counterparty.DisplayName,
				AccountType:   it.Counterparty.AccountType,
			}
		}
		out.Items = append(out.Items, pbIt)
	}

	return out
}

func ToCreditAccountSystemResponse(res *in.CreditAccountResult) *ledgerpb.CreditAccountSystemResponse {
	return &ledgerpb.CreditAccountSystemResponse{
		JournalId: res.JournalID.String(),
		Status:    res.Status,
	}
}
