package validation

import (
	"time"

	ledgerpb "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	in "github.com/tagoKoder/ledger/internal/domain/port/in"
)

func ToCreditAccountCommand(req *ledgerpb.CreditAccountRequest) (in.CreditAccountCommand, error) {
	if req == nil {
		return in.CreditAccountCommand{}, Invalid("request is required")
	}

	idem, err := RequireIdempotencyKey(req.GetIdempotencyKey(), "idempotency_key")
	if err != nil {
		return in.CreditAccountCommand{}, err
	}

	acc, err := RequireUUID(req.GetAccountId(), "account_id")
	if err != nil {
		return in.CreditAccountCommand{}, err
	}

	ccy, err := RequireCurrency(req.GetCurrency())
	if err != nil {
		return in.CreditAccountCommand{}, err
	}

	amt, err := RequireDecimalAmount(req.GetAmount(), "amount")
	if err != nil {
		return in.CreditAccountCommand{}, err
	}

	actor, err := RequireActor(req.GetInitiatedBy())
	if err != nil {
		return in.CreditAccountCommand{}, err
	}

	ext, err := OptionalExternalRef(req.GetExternalRef())
	if err != nil {
		return in.CreditAccountCommand{}, err
	}

	// Campos extra del proto que HOY tu dominio no usa: reason/customer_id.
	// Igual los validamos para no aceptar basura/inyección.
	if _, err := OptionalReason(req.GetReason()); err != nil {
		return in.CreditAccountCommand{}, err
	}
	if req.GetCustomerId() != "" {
		if _, err := RequireUUID(req.GetCustomerId(), "customer_id"); err != nil {
			return in.CreditAccountCommand{}, err
		}
	}

	return in.CreditAccountCommand{
		IdempotencyKey: idem,
		AccountID:      acc,
		Currency:       ccy,
		Amount:         amt,
		InitiatedBy:    actor,
		ExternalRef:    ext,
	}, nil
}

func ToListAccountJournalEntriesQuery(req *ledgerpb.ListAccountJournalEntriesRequest) (in.ListAccountJournalEntriesQuery, error) {
	if req == nil {
		return in.ListAccountJournalEntriesQuery{}, Invalid("request is required")
	}

	acc, err := RequireUUID(req.GetAccountId(), "account_id")
	if err != nil {
		return in.ListAccountJournalEntriesQuery{}, err
	}

	from, err := RequireTimestamp(req.GetFrom(), "from")
	if err != nil {
		return in.ListAccountJournalEntriesQuery{}, err
	}

	to, err := RequireTimestamp(req.GetTo(), "to")
	if err != nil {
		return in.ListAccountJournalEntriesQuery{}, err
	}

	// DoS guard: máximo 90 días (ajústalo)
	if err := ValidateTimeWindow(from, to, 90*24*time.Hour); err != nil {
		return in.ListAccountJournalEntriesQuery{}, err
	}

	page, size, err := NormalizePageSize(req.GetPage(), req.GetSize(), 200, false /* page 0-based en tu proto actual */)
	if err != nil {
		return in.ListAccountJournalEntriesQuery{}, err
	}

	return in.ListAccountJournalEntriesQuery{
		AccountID:   acc,
		FromRFC3339: from.Format(time.RFC3339),
		ToRFC3339:   to.Format(time.RFC3339),
		Page:        page,
		Size:        size,
	}, nil
}

func ToListAccountStatementQuery(req *ledgerpb.ListAccountStatementRequest) (in.ListAccountStatementQuery, error) {
	if req == nil {
		return in.ListAccountStatementQuery{}, Invalid("request is required")
	}

	acc, err := RequireUUID(req.GetAccountId(), "account_id")
	if err != nil {
		return in.ListAccountStatementQuery{}, err
	}

	from, err := RequireTimestamp(req.GetFrom(), "from")
	if err != nil {
		return in.ListAccountStatementQuery{}, err
	}

	to, err := RequireTimestamp(req.GetTo(), "to")
	if err != nil {
		return in.ListAccountStatementQuery{}, err
	}

	// DoS guard: máximo 180 días para estados de cuenta (ajústalo)
	if err := ValidateTimeWindow(from, to, 180*24*time.Hour); err != nil {
		return in.ListAccountStatementQuery{}, err
	}

	// proto dice 1-based para statement
	page, size, err := NormalizePageSize(req.GetPage(), req.GetSize(), 200, true)
	if err != nil {
		return in.ListAccountStatementQuery{}, err
	}

	return in.ListAccountStatementQuery{
		AccountID:           acc,
		From:                from,
		To:                  to,
		Page:                page,
		Size:                size,
		IncludeCounterparty: req.GetIncludeCounterparty(),
	}, nil
}
