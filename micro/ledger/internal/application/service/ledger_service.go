// micro\ledger\internal\application\service\ledger_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	accountsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/accounts/v1"
	"github.com/tagoKoder/ledger/internal/application/uow"
	dm "github.com/tagoKoder/ledger/internal/domain/model"
	in "github.com/tagoKoder/ledger/internal/domain/port/in"
	out "github.com/tagoKoder/ledger/internal/domain/port/out"
	authctx "github.com/tagoKoder/ledger/internal/infra/security/context"
)

type ledgerAppService struct {
	uow      uow.UnitOfWorkManager
	accounts out.AccountsGatewayPort
	audit    out.AuditPort
}

func NewLedgerAppService(uow uow.UnitOfWorkManager, accounts out.AccountsGatewayPort, audit out.AuditPort) in.LedgerAppService {
	return &ledgerAppService{uow: uow, accounts: accounts, audit: audit}
}

func (s *ledgerAppService) CreditAccount(ctx context.Context, cmd in.CreditAccountCommand) (*in.CreditAccountResult, error) {
	actor := authctx.ActorFrom(ctx)
	initiatedBy := actor.Subject
	if initiatedBy == "" {
		initiatedBy = cmd.InitiatedBy
	}

	amt, err := decimal.NewFromString(cmd.Amount)
	if err != nil {
		return nil, err
	}

	// Idempotencia
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
		var prev *in.CreditAccountResult
		_ = json.Unmarshal([]byte(cachedJSON), &prev)
		return prev, nil
	}

	vresp, err := s.accounts.ValidateAccountsAndLimits(ctx, &accountsv1.ValidateAccountsAndLimitsRequest{
		SourceAccountId:      cmd.AccountID.String(),
		DestinationAccountId: cmd.AccountID.String(),
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

	journalID := uuid.New()
	now := time.Now().UTC()

	var result *in.CreditAccountResult
	err = s.uow.DoWrite(ctx, func(w uow.WriteRepos) error {
		// Journal lines:
		// - Débito a "System Funding" (GL_SYSTEM_FUND)
		// - Crédito a cuenta del cliente (GL_CUSTOMER_CASH)
		accRef := cmd.AccountID.String()

		lines := []dm.EntryLine{
			{
				ID: uuid.New(), JournalID: journalID,
				GLAccountID: dm.GLSystemFundID, GLAccountCode: "GL_SYSTEM_FUND",
				CounterpartyRef: nil,
				Debit:           amt, Credit: decimal.Zero,
			},
			{
				ID: uuid.New(), JournalID: journalID,
				GLAccountID: dm.GLCustomerCashID, GLAccountCode: "GL_CUSTOMER_CASH",
				CounterpartyRef: &accRef,
				Debit:           decimal.Zero, Credit: amt,
			},
		}
		if err := EnsureBalanced(lines); err != nil {
			return err
		}

		j := &dm.JournalEntry{
			ID:          journalID,
			ExternalRef: cmd.ExternalRef,
			BookedAt:    now,
			Status:      dm.JournalPosted,
			Currency:    cmd.Currency,
			CreatedBy:   initiatedBy,
			Lines:       lines,
		}
		if err := w.Journals().InsertJournal(ctx, j); err != nil {
			return err
		}

		// Outbox ledger.posted
		payload := map[string]any{
			"journal_id":  journalID.String(),
			"account_id":  cmd.AccountID.String(),
			"currency":    cmd.Currency,
			"amount":      amt.StringFixed(6),
			"occurred_at": now.Format(time.RFC3339Nano),
			"postings": []map[string]string{
				{
					"account_id":  cmd.AccountID.String(),
					"d_ledger":    amt.StringFixed(6),
					"d_available": amt.StringFixed(6),
					"d_hold":      "0.000000",
				},
			},
		}
		pj, _ := json.Marshal(payload)

		if err := w.Outbox().Insert(ctx, &dm.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "ledger",
			AggregateID:   journalID,
			EventType:     "ledger.posted",
			PayloadJSON:   string(pj),
			Published:     false,
			CreatedAt:     now,
		}); err != nil {
			return err
		}

		// Idempotency save
		result = &in.CreditAccountResult{JournalID: journalID, Status: "posted"}
		rj, _ := json.Marshal(result)
		record := &dm.IdempotencyRecord{
			Key:          cmd.IdempotencyKey,
			Operation:    "PostTopup",
			ResponseJSON: string(rj),
			StatusCode:   200,
			CreatedAt:    now,
		}
		if err := w.Idempotency().Put(ctx, record); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, "journal_entries", journalID.String(), now, map[string]any{
		"account_id": cmd.AccountID.String(),
		"currency":   cmd.Currency,
		"amount":     amt.StringFixed(6),
	})

	return result, nil
}

func (s *ledgerAppService) CreditAccountSystem(ctx context.Context, cmd in.CreditAccountSystemCommand) (*in.CreditAccountResult, error) {
	// NO usa initiated_by del usuario. Es “system/service”.
	initiatedBy := "system"

	amt, err := decimal.NewFromString(cmd.Amount)
	if err != nil {
		return nil, err
	}

	// Idempotencia (igual que CreditAccount)
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
		var prev *in.CreditAccountResult
		_ = json.Unmarshal([]byte(cachedJSON), &prev)
		return prev, nil
	}

	// Validación básica en Accounts (self-credit)
	vresp, err := s.accounts.ValidateAccountsAndLimits(ctx, &accountsv1.ValidateAccountsAndLimitsRequest{
		SourceAccountId:      cmd.AccountID.String(),
		DestinationAccountId: cmd.AccountID.String(),
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

	journalID := uuid.New()
	now := time.Now().UTC()

	var result *in.CreditAccountResult
	err = s.uow.DoWrite(ctx, func(w uow.WriteRepos) error {
		accRef := cmd.AccountID.String()

		// Igual que CreditAccount: SystemFund -> CustomerCash
		lines := []dm.EntryLine{
			{
				ID: uuid.New(), JournalID: journalID,
				GLAccountID: dm.GLSystemFundID, GLAccountCode: "GL_SYSTEM_FUND",
				CounterpartyRef: nil,
				Debit:           amt, Credit: decimal.Zero,
			},
			{
				ID: uuid.New(), JournalID: journalID,
				GLAccountID: dm.GLCustomerCashID, GLAccountCode: "GL_CUSTOMER_CASH",
				CounterpartyRef: &accRef,
				Debit:           decimal.Zero, Credit: amt,
			},
		}
		if err := EnsureBalanced(lines); err != nil {
			return err
		}

		j := &dm.JournalEntry{
			ID:          journalID,
			ExternalRef: cmd.ExternalRef,
			BookedAt:    now,
			Status:      dm.JournalPosted,
			Currency:    cmd.Currency,
			CreatedBy:   initiatedBy,
			Lines:       lines,
		}
		if err := w.Journals().InsertJournal(ctx, j); err != nil {
			return err
		}

		// outbox ledger.posted
		payload := map[string]any{
			"journal_id":   journalID.String(),
			"currency":     cmd.Currency,
			"reason":       cmd.Reason,
			"external_ref": cmd.ExternalRef,
			"occurred_at":  now.Format(time.RFC3339Nano),
			"postings": []map[string]string{
				{
					"account_id":  cmd.AccountID.String(),
					"d_ledger":    amt.StringFixed(6),
					"d_available": amt.StringFixed(6),
					"d_hold":      "0.000000",
				},
			},
		}
		pj, _ := json.Marshal(payload)

		if err := w.Outbox().Insert(ctx, &dm.OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "ledger",
			AggregateID:   journalID,
			EventType:     "ledger.posted",
			PayloadJSON:   string(pj),
			Published:     false,
			CreatedAt:     now,
		}); err != nil {
			return err
		}

		result = &in.CreditAccountResult{JournalID: journalID, Status: "posted"}
		rj, _ := json.Marshal(result)

		if err := w.Idempotency().Put(ctx, &dm.IdempotencyRecord{
			Key:          cmd.IdempotencyKey,
			Operation:    "CreditAccountSystem:" + cmd.Reason,
			ResponseJSON: string(rj),
			StatusCode:   200,
			CreatedAt:    now,
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, "ledger.credit_system", journalID.String(), now, map[string]any{
		"account_id":   cmd.AccountID.String(),
		"currency":     cmd.Currency,
		"amount":       amt.StringFixed(6),
		"reason":       cmd.Reason,
		"external_ref": cmd.ExternalRef,
	})

	return result, nil
}

func (s *ledgerAppService) ListAccountJournalEntries(ctx context.Context, q in.ListAccountJournalEntriesQuery) (*in.ListAccountJournalEntriesResult, error) {

	from := time.Time{}
	to := time.Now().UTC()

	if q.FromRFC3339 != "" {
		t, err := time.Parse(time.RFC3339Nano, q.FromRFC3339)
		if err == nil {
			from = t
		}
	}
	if q.ToRFC3339 != "" {
		t, err := time.Parse(time.RFC3339Nano, q.ToRFC3339)
		if err == nil {
			to = t
		}
	}

	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 {
		q.Size = 20
	}

	var entries []dm.JournalEntry
	err := s.uow.DoRead(ctx, func(r uow.ReadRepos) error {
		var err error
		entries, err = r.Journals().ListActivityByAccount(ctx, q.AccountID, from, to, q.Page, q.Size)
		return err
	})
	if err != nil {
		return nil, err
	}

	views := make([]in.JournalEntryView, 0, len(entries))
	for _, e := range entries {
		v := in.JournalEntryView{
			JournalID:       e.ID.String(),
			Currency:        e.Currency,
			Status:          string(e.Status),
			BookedAtRFC3339: e.BookedAt.UTC().Format(time.RFC3339Nano),
			ExternalRef:     e.ExternalRef,
		}
		for _, l := range e.Lines {
			cp := ""
			if l.CounterpartyRef != nil {
				cp = *l.CounterpartyRef
			}
			v.Lines = append(v.Lines, in.JournalLineView{
				GLAccountCode:   l.GLAccountCode,
				CounterpartyRef: cp,
				Debit:           l.Debit.StringFixed(6),
				Credit:          l.Credit.StringFixed(6),
			})
		}
		views = append(views, v)
	}

	return &in.ListAccountJournalEntriesResult{Entries: views, Page: q.Page, Size: q.Size}, nil
}

func (s *ledgerAppService) ListAccountStatement(ctx context.Context, q in.ListAccountStatementQuery) (*in.ListAccountStatementResult, error) {
	// defaults
	from := q.From
	if from.IsZero() {
		from = time.Time{}
	}
	to := q.To
	if to.IsZero() {
		to = time.Now().UTC()
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	size := q.Size
	if size <= 0 {
		size = 20
	}

	var entries []dm.JournalEntry
	err := s.uow.DoRead(ctx, func(r uow.ReadRepos) error {
		var e error
		entries, e = r.Journals().ListActivityByAccount(ctx, q.AccountID, from, to, page, size)
		return e
	})
	if err != nil {
		return nil, err
	}

	accStr := q.AccountID.String()

	// 1) construir proyección base
	items := make([]in.StatementItemView, 0, len(entries))
	counterpartyIDsSet := map[string]struct{}{}

	for _, je := range entries {
		totalCredit := decimal.Zero
		totalDebit := decimal.Zero
		counterpartyOther := ""

		for _, l := range je.Lines {
			cp := ""
			if l.CounterpartyRef != nil {
				cp = *l.CounterpartyRef
			}

			// suma neta SOLO sobre líneas pertenecientes a esta cuenta
			if cp == accStr {
				totalCredit = totalCredit.Add(l.Credit)
				totalDebit = totalDebit.Add(l.Debit)
				continue
			}

			// detecta contraparte (otra cuenta del journal)
			if cp != "" && cp != accStr && counterpartyOther == "" {
				counterpartyOther = cp
			}
		}

		net := totalCredit.Sub(totalDebit)
		direction := "credit"
		amt := net
		if net.IsNegative() {
			direction = "debit"
			amt = net.Abs()
		}

		kind := "other"
		memo := je.ExternalRef
		if strings.HasPrefix(memo, "payment:") {
			kind = "transfer"
		} else if strings.HasPrefix(memo, "bonus:") || strings.Contains(memo, "registration_bonus") {
			kind = "bonus"
		}

		it := in.StatementItemView{
			JournalID:             je.ID.String(),
			BookedAt:              je.BookedAt.UTC(),
			Currency:              je.Currency,
			Direction:             direction,
			Amount:                amt.StringFixed(6),
			Kind:                  kind,
			Memo:                  memo,
			CounterpartyAccountID: counterpartyOther,
		}

		if counterpartyOther != "" {
			counterpartyIDsSet[counterpartyOther] = struct{}{}
		}

		items = append(items, it)
	}

	// 2) enrichment opcional (1 llamada batch por página)
	if q.IncludeCounterparty && len(counterpartyIDsSet) > 0 {
		ids := make([]string, 0, len(counterpartyIDsSet))
		for id := range counterpartyIDsSet {
			ids = append(ids, id)
		}

		resp, err := s.accounts.BatchGetAccountSummaries(ctx, &accountsv1.BatchGetAccountSummariesRequest{
			AccountIds: ids,
		})
		if err != nil {
			// en demo: puedes decidir si fallar o degradar.
			// Recomendación para UX: degradar (devuelve statement sin counterparty)
			resp = nil
		}

		lookup := map[string]*in.CounterpartyView{}
		if resp != nil {
			for _, a := range resp.Accounts {
				lookup[a.AccountId] = &in.CounterpartyView{
					AccountID:     a.AccountId,
					AccountNumber: a.AccountNumber,
					DisplayName:   a.DisplayName,
					AccountType:   a.ProductType.String(),
				}
			}
		}

		for i := range items {
			cpID := items[i].CounterpartyAccountID
			if cpID == "" {
				continue
			}
			if v, ok := lookup[cpID]; ok {
				items[i].Counterparty = v
			}
		}
	}

	// opcional: auditoría best-effort (alineado ASVS)
	now := time.Now().UTC()
	_ = s.audit.Record(ctx, "ledger.statement", q.AccountID.String(), now, map[string]any{
		"from":                 from.Format(time.RFC3339Nano),
		"to":                   to.Format(time.RFC3339Nano),
		"page":                 page,
		"size":                 size,
		"include_counterparty": q.IncludeCounterparty,
	})

	return &in.ListAccountStatementResult{
		Items: items,
		Page:  page,
		Size:  size,
	}, nil
}
