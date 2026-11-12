package validation

import (
	ledgerpb "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	in "github.com/tagoKoder/ledger/internal/domain/port/in"
)

func ToCreditAccountSystemCommand(req *ledgerpb.CreditAccountSystemRequest) (in.CreditAccountSystemCommand, error) {
	if req == nil {
		return in.CreditAccountSystemCommand{}, Invalid("request is required")
	}

	idem, err := RequireIdempotencyKey(req.GetIdempotencyKey(), "idempotency_key")
	if err != nil {
		return in.CreditAccountSystemCommand{}, err
	}

	acc, err := RequireUUID(req.GetAccountId(), "account_id")
	if err != nil {
		return in.CreditAccountSystemCommand{}, err
	}

	ccy, err := RequireCurrency(req.GetCurrency())
	if err != nil {
		return in.CreditAccountSystemCommand{}, err
	}

	amt, err := RequireDecimalAmount(req.GetAmount(), "amount")
	if err != nil {
		return in.CreditAccountSystemCommand{}, err
	}

	ext, err := RequireNonBlank(req.GetExternalRef(), "external_ref", 140)
	if err != nil {
		return in.CreditAccountSystemCommand{}, err
	}
	if _, err := OptionalExternalRef(ext); err != nil { // reusa regex safe
		return in.CreditAccountSystemCommand{}, err
	}

	reason, err := RequireNonBlank(req.GetReason(), "reason", 64)
	if err != nil {
		return in.CreditAccountSystemCommand{}, err
	}
	reason, err = OptionalReason(reason)
	if err != nil {
		return in.CreditAccountSystemCommand{}, err
	}
	if reason == "" {
		return in.CreditAccountSystemCommand{}, Invalid("reason is required")
	}

	// MVP: solo permitimos registration_bonus por ahora
	if reason != "registration_bonus" {
		return in.CreditAccountSystemCommand{}, Forbidden("reason not allowed")
	}

	return in.CreditAccountSystemCommand{
		IdempotencyKey: idem,
		AccountID:      acc,
		Currency:       ccy,
		Amount:         amt,
		ExternalRef:    ext,
		Reason:         reason,
	}, nil
}
