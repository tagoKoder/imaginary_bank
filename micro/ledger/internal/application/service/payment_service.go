// micro\ledger\internal\application\service\payment_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	accountsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/accounts/v1"
	"github.com/tagoKoder/ledger/internal/application/uow"
	"github.com/tagoKoder/ledger/internal/domain/model"
	dm "github.com/tagoKoder/ledger/internal/domain/model"
	in "github.com/tagoKoder/ledger/internal/domain/port/in"
	out "github.com/tagoKoder/ledger/internal/domain/port/out"
	authctx "github.com/tagoKoder/ledger/internal/infra/security/context"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type paymentService struct {
	uow      uow.UnitOfWorkManager
	accounts out.AccountsGatewayPort
	audit    out.AuditPort
}

func NewPaymentService(u uow.UnitOfWorkManager, accounts out.AccountsGatewayPort, audit out.AuditPort) in.PaymentService {
	return &paymentService{uow: u, accounts: accounts, audit: audit}
}

func (s *paymentService) PostPayment(ctx context.Context, cmd in.PostPaymentCommand) (*in.PostPaymentResult, error) {
	actor := authctx.ActorFrom(ctx)
	initiatedBy := actor.Subject
	if initiatedBy == "" {
		initiatedBy = cmd.InitiatedBy
	}

	amt, err := decimal.NewFromString(cmd.Amount)
	if err != nil {
		return nil, err
	}

	// 1) Idempotencia (read)
	var cachedJSON string
	if err := s.uow.DoRead(ctx, func(r uow.ReadRepos) error {
		rec, err := r.Idempotency().Get(ctx, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if rec != nil {
			cachedJSON = rec.ResponseJSON
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if cachedJSON != "" {
		var pr *in.PostPaymentResult
		_ = json.Unmarshal([]byte(cachedJSON), &pr)
		return pr, nil
	}

	// 2) Validación en Accounts (gRPC)
	vresp, err := s.accounts.ValidateAccountsAndLimits(ctx, &accountsv1.ValidateAccountsAndLimitsRequest{
		SourceAccountId:      cmd.SourceAccountID.String(),
		DestinationAccountId: cmd.DestinationAccount.String(),
		Currency:             cmd.Currency,
		Amount:               amt.StringFixed(6),
	})
	if err != nil {
		return nil, err
	}
	if vresp != nil && !vresp.Ok {
		reason := ""
		if vresp.Reason != nil {
			reason = vresp.Reason.Value
		}
		return nil, fmt.Errorf("accounts validation failed: %s", reason)
	}

	// IDs estables
	paymentID := uuid.New()
	now := time.Now().UTC()

	// customer_id (tu entity y DB dicen NOT NULL: garantiza no-nil aquí)
	customerID := uuidPtrFromString(actor.CustomerID)
	if customerID == nil {
		return nil, fmt.Errorf("customer_id is required in context (got empty/invalid actor.CustomerID)")
	}

	// HoldID estable = paymentID (puntero)
	holdID := paymentID
	holdIDPtr := &holdID

	// 3) ReserveHold FUERA del TX (saga)
	reserveKey := cmd.IdempotencyKey + ":reserve_hold"
	rresp, err := s.accounts.ReserveHold(ctx, &accountsv1.ReserveHoldRequest{
		Id: cmd.SourceAccountID.String(),
		Hold: &accountsv1.HoldRequest{
			Currency: cmd.Currency,
			Amount:   amt.StringFixed(6),
			Reason:   wrapperspb.String("payment"),
		},
		HoldId:         paymentID.String(),
		IdempotencyKey: reserveKey,
	})
	if err != nil {
		return nil, err
	}
	if rresp != nil && !rresp.Ok {
		st := ""
		if rresp.Status != nil {
			st = rresp.Status.Value
		}
		return nil, fmt.Errorf("reserve hold failed status=%s", st)
	}

	// Si el TX falla luego, compensamos con ReleaseHold
	releaseKey := cmd.IdempotencyKey + ":release_hold"
	releaseReq := &accountsv1.ReleaseHoldRequest{
		Id: cmd.SourceAccountID.String(),
		Hold: &accountsv1.HoldRequest{
			Currency: cmd.Currency,
			Amount:   amt.StringFixed(6),
			Reason:   wrapperspb.String("payment-compensation"),
		},
		HoldId:         paymentID.String(),
		IdempotencyKey: releaseKey,
	}

	// 4) TX: persist payment + journal + outbox + idempotency + steps + status update
	var result *in.PostPaymentResult
	err = s.uow.DoWrite(ctx, func(w uow.WriteRepos) error {
		// 4.1 Genera journal_id dentro del TX y guárdalo en payments.journal_id
		jid := uuid.New()

		// 4.2 Insert payment (processing)
		p := &dm.Payment{
			ID:             paymentID,
			IdempotencyKey: cmd.IdempotencyKey,
			SourceAccount:  cmd.SourceAccountID,
			DestAccount:    cmd.DestinationAccount,
			Amount:         amt,
			Currency:       cmd.Currency,
			Status:         dm.PaymentProcessing,

			CustomerID:    customerID,
			HoldID:        holdIDPtr,
			JournalID:     &jid,
			CorrelationID: "", // si luego tienes un getter real, lo llenas aquí

			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := w.Payments().Insert(ctx, p); err != nil {
			return err
		}

		// 4.3 journal entry (doble partida)
		srcRef := cmd.SourceAccountID.String()
		dstRef := cmd.DestinationAccount.String()

		lines := []dm.EntryLine{
			{ID: uuid.New(), JournalID: jid, GLAccountID: dm.GLOutID, GLAccountCode: "GL_OUT", CounterpartyRef: &srcRef, Debit: amt, Credit: decimal.Zero},
			{ID: uuid.New(), JournalID: jid, GLAccountID: dm.GLInID, GLAccountCode: "GL_IN", CounterpartyRef: &dstRef, Debit: decimal.Zero, Credit: amt},
		}
		if err := EnsureBalanced(lines); err != nil {
			return err
		}

		j := &dm.JournalEntry{
			ID:          jid,
			ExternalRef: "payment:" + paymentID.String(),
			BookedAt:    now,
			Status:      dm.JournalPosted,
			Currency:    cmd.Currency,
			CreatedBy:   initiatedBy,
			Lines:       lines,
		}
		if err := w.Journals().InsertJournal(ctx, j); err != nil {
			return err
		}

		// 4.4 payment steps (si falla no aborta el TX; son evidencia/trace)
		_ = w.Payments().InsertStep(ctx, &dm.PaymentStep{
			ID: uuid.New(), PaymentID: paymentID,
			Step: "reserve_hold", State: "ok",
			DetailsJSON: `{"hold_id":"` + paymentID.String() + `"}`,
			AttemptedAt: now,
		})
		_ = w.Payments().InsertStep(ctx, &dm.PaymentStep{
			ID: uuid.New(), PaymentID: paymentID,
			Step: "post_ledger", State: "ok",
			DetailsJSON: `{"journal_id":"` + jid.String() + `","hold_id":"` + paymentID.String() + `"}`,
			AttemptedAt: now,
		})

		// 4.5 outbox payment.posted
		payload := map[string]any{
			"payment_id":             paymentID.String(),
			"source_account_id":      srcRef,
			"destination_account_id": dstRef,
			"currency":               cmd.Currency,
			"amount":                 amt.StringFixed(6),
			"hold_id":                holdIDPtr.String(),
			"journal_id":             jid.String(),
			"occurred_at":            now.Format(time.RFC3339Nano),
		}
		pj, _ := json.Marshal(payload)

		if err := w.Outbox().Insert(ctx, &dm.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "payment",
			AggregateID:   paymentID,
			EventType:     "payment.posted",
			PayloadJSON:   string(pj),
			Published:     false,
			CreatedAt:     now,
		}); err != nil {
			return err
		}

		// 4.6 status -> posted (actualiza updated_at también)
		if err := w.Payments().UpdateStatus(ctx, paymentID, string(dm.PaymentPosted)); err != nil {
			return err
		}

		// 4.7 idempotency record
		result = &in.PostPaymentResult{PaymentID: paymentID, Status: "posted"}
		rj, _ := json.Marshal(result)
		if err := w.Idempotency().Put(ctx, &model.IdempotencyRecord{
			Key:          cmd.IdempotencyKey,
			Operation:    "PostPayment",
			ResponseJSON: string(rj),
			StatusCode:   200,
			CreatedAt:    now,
		}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		// compensación: ReleaseHold best-effort
		_, _ = s.accounts.ReleaseHold(ctx, releaseReq)
		return nil, err
	}

	_ = s.audit.Record(ctx, "payments", paymentID.String(), now, map[string]any{
		"currency": cmd.Currency,
		"amount":   amt.StringFixed(6),
	})

	return result, nil
}

func (s *paymentService) GetPayment(ctx context.Context, paymentID uuid.UUID) (*in.GetPaymentResult, error) {
	var res *in.GetPaymentResult
	err := s.uow.DoRead(ctx, func(r uow.ReadRepos) error {
		p, err := r.Payments().FindById(ctx, paymentID)
		if err != nil {
			return err
		}
		steps, _ := r.Payments().ListSteps(ctx, paymentID)

		res = &in.GetPaymentResult{
			PaymentID:        paymentID,
			Status:           string(p.Status),
			IdempotencyKey:   p.IdempotencyKey,
			SourceAccountID:  p.SourceAccount,
			DestAccountID:    p.DestAccount,
			Currency:         p.Currency,
			Amount:           p.Amount.StringFixed(6),
			CreatedAtRFC3339: p.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		for _, s := range steps {
			res.Steps = append(res.Steps, in.PaymentStepView{
				Step: s.Step, State: s.State,
				DetailsJSON:        s.DetailsJSON,
				AttemptedAtRFC3339: s.AttemptedAt.UTC().Format(time.RFC3339Nano),
			})
		}
		return nil
	})
	return res, err
}

func uuidPtrFromString(s string) *uuid.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &u
}
