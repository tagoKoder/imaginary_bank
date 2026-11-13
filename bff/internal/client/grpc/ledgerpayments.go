// bff\internal\client\grpc\ledgerpayments.go
package grpc

import (
	"context"
	"time"

	ledgerpaymentsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	"github.com/tagoKoder/bff/internal/client/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ ports.LedgerPaymentsPort = (*LedgerPaymentsClient)(nil)

type LedgerPaymentsClient struct {
	payments ledgerpaymentsv1.PaymentsServiceClient
	ledger   ledgerpaymentsv1.LedgerServiceClient
	timeout  time.Duration
}

func NewLedgerPaymentsClient(conn *grpc.ClientConn) *LedgerPaymentsClient {
	return &LedgerPaymentsClient{
		payments: ledgerpaymentsv1.NewPaymentsServiceClient(conn),
		ledger:   ledgerpaymentsv1.NewLedgerServiceClient(conn),
		timeout:  6 * time.Second,
	}
}

func (c *LedgerPaymentsClient) PostPayment(ctx context.Context, in ports.PostPaymentInput) (ports.PostPaymentOutput, error) {

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.payments.PostPayment(ctx2, &ledgerpaymentsv1.PostPaymentRequest{
		IdempotencyKey:       in.IdempotencyKey,
		SourceAccountId:      in.SourceAccountID,
		DestinationAccountId: in.DestinationAccountID,
		Currency:             in.Currency,
		Amount:               in.Amount,
		InitiatedBy:          in.InitiatedBy,
	})
	if err != nil {
		return ports.PostPaymentOutput{}, err
	}
	return ports.PostPaymentOutput{PaymentID: res.PaymentId, Status: res.Status}, nil
}

func (c *LedgerPaymentsClient) GetPayment(ctx context.Context, in ports.GetPaymentInput) (ports.GetPaymentOutput, error) {

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.payments.GetPayment(ctx2, &ledgerpaymentsv1.GetPaymentRequest{PaymentId: in.PaymentID})
	if err != nil {
		return ports.GetPaymentOutput{}, err
	}

	out := ports.GetPaymentOutput{
		PaymentID: res.PaymentId,
		Status:    res.Status,
		//IdempotencyKey:       res.IdempotencyKey,
		SourceAccountID:      res.SourceAccountId,
		DestinationAccountID: res.DestinationAccountId,
		Currency:             res.Currency,
		Amount:               res.Amount,
		Steps:                make([]ports.PaymentStep, 0, len(res.Steps)),
	}
	if res.CreatedAt != nil {
		out.CreatedAtRFC3339 = res.CreatedAt.AsTime().Format(time.RFC3339)
	}
	for _, s := range res.Steps {
		step := ports.PaymentStep{
			Step:        s.Step,
			State:       s.State,
			DetailsJSON: s.DetailsJson,
		}
		if s.AttemptedAt != nil {
			step.AttemptedAtRFC3339 = s.AttemptedAt.AsTime().Format(time.RFC3339)
		}
		out.Steps = append(out.Steps, step)
	}
	return out, nil
}

func (c *LedgerPaymentsClient) ListAccountStatement(ctx context.Context, in ports.ListAccountStatementInput) (ports.ListAccountStatementOutput, error) {

	req := &ledgerpaymentsv1.ListAccountStatementRequest{
		AccountId:           in.AccountID,
		Page:                in.Page,
		Size:                in.Size,
		IncludeCounterparty: in.IncludeCounterparty,
	}

	if in.FromRFC3339 != nil {
		t, err := time.Parse(time.RFC3339, *in.FromRFC3339)
		if err != nil {
			return ports.ListAccountStatementOutput{}, status.Error(codes.InvalidArgument, "invalid from timestamp")
		}
		req.From = timestamppb.New(t)
	}
	if in.ToRFC3339 != nil {
		t, err := time.Parse(time.RFC3339, *in.ToRFC3339)
		if err != nil {
			return ports.ListAccountStatementOutput{}, status.Error(codes.InvalidArgument, "invalid to timestamp")
		}
		req.To = timestamppb.New(t)
	}

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.ledger.ListAccountStatement(ctx2, req)
	if err != nil {
		return ports.ListAccountStatementOutput{}, err
	}

	out := ports.ListAccountStatementOutput{
		Items: make([]ports.StatementItem, 0, len(res.Items)),
		Page:  res.Page,
		Size:  res.Size,
	}

	for _, it := range res.Items {
		var booked string
		if it.BookedAt != nil {
			booked = it.BookedAt.AsTime().Format(time.RFC3339)
		}

		var memo *string
		if it.Memo != "" {
			s := it.Memo
			memo = &s
		}

		var cpAccID *string
		if it.CounterpartyAccountId != "" {
			s := it.CounterpartyAccountId
			cpAccID = &s
		}

		var cp *ports.CounterpartyView
		if it.Counterparty != nil {
			cp = &ports.CounterpartyView{
				AccountID:     it.Counterparty.AccountId,
				AccountNumber: it.Counterparty.AccountNumber,
				DisplayName:   it.Counterparty.DisplayName,
				AccountType:   it.Counterparty.AccountType,
			}
		}

		out.Items = append(out.Items, ports.StatementItem{
			JournalID:             it.JournalId,
			BookedAtRFC3339:       booked,
			Currency:              it.Currency,
			Direction:             it.Direction,
			Amount:                it.Amount,
			Kind:                  it.Kind,
			Memo:                  memo,
			CounterpartyAccountID: cpAccID,
			Counterparty:          cp,
		})
	}

	return out, nil
}
