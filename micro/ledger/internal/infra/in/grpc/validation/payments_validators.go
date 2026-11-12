package validation

import (
	"github.com/google/uuid"
	ledgerpb "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	in "github.com/tagoKoder/ledger/internal/domain/port/in"
)

func ToPostPaymentCommand(req *ledgerpb.PostPaymentRequest) (in.PostPaymentCommand, error) {
	if req == nil {
		return in.PostPaymentCommand{}, Invalid("request is required")
	}

	idem, err := RequireIdempotencyKey(req.GetIdempotencyKey(), "idempotency_key")
	if err != nil {
		return in.PostPaymentCommand{}, err
	}

	src, err := RequireUUID(req.GetSourceAccountId(), "source_account_id")
	if err != nil {
		return in.PostPaymentCommand{}, err
	}

	dst, err := RequireUUID(req.GetDestinationAccountId(), "destination_account_id")
	if err != nil {
		return in.PostPaymentCommand{}, err
	}

	if src == dst {
		return in.PostPaymentCommand{}, Invalid("source_account_id must be different from destination_account_id")
	}

	ccy, err := RequireCurrency(req.GetCurrency())
	if err != nil {
		return in.PostPaymentCommand{}, err
	}

	amt, err := RequireDecimalAmount(req.GetAmount(), "amount")
	if err != nil {
		return in.PostPaymentCommand{}, err
	}

	actor, err := RequireActor(req.GetInitiatedBy())
	if err != nil {
		return in.PostPaymentCommand{}, err
	}

	return in.PostPaymentCommand{
		IdempotencyKey:     idem,
		SourceAccountID:    src,
		DestinationAccount: dst,
		Currency:           ccy,
		Amount:             amt,
		InitiatedBy:        actor,
	}, nil
}

func ToGetPaymentID(req *ledgerpb.GetPaymentRequest) (inUUID string, paymentIDErr error) {
	if req == nil {
		return "", Invalid("request is required")
	}
	id, err := RequireUUID(req.GetPaymentId(), "payment_id")
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// reemplaza la anterior
func ToGetPaymentUUID(req *ledgerpb.GetPaymentRequest) (uuid.UUID, error) {
	if req == nil {
		return uuid.Nil, Invalid("request is required")
	}
	return RequireUUID(req.GetPaymentId(), "payment_id")
}
