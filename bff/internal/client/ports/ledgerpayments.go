// bff\internal\client\ports\ledgerpayments.go
package ports

import "context"

type PostPaymentInput struct {
	IdempotencyKey       string
	SourceAccountID      string
	DestinationAccountID string
	Currency             string
	Amount               string // decimal string
	InitiatedBy          string
}
type PostPaymentOutput struct {
	PaymentID string
	Status    string
}

type GetPaymentInput struct {
	PaymentID string
}
type PaymentStep struct {
	Step               string
	State              string
	DetailsJSON        string
	AttemptedAtRFC3339 string
}

type GetPaymentOutput struct {
	PaymentID            string
	Status               string
	SourceAccountID      string
	DestinationAccountID string
	Currency             string
	Amount               string
	Steps                []PaymentStep
	CreatedAtRFC3339     string
}

type CreditAccountInput struct {
	IdempotencyKey string
	AccountID      string
	Currency       string
	Amount         string // decimal string
	InitiatedBy    string
	ExternalRef    *string
}
type CreditAccountOutput struct {
	JournalID string
	Status    string
}

type ListAccountJournalEntriesInput struct {
	AccountID   string
	FromRFC3339 *string
	ToRFC3339   *string
	Page        int32
	Size        int32
}
type JournalLine struct {
	GLAccountCode   string
	CounterpartyRef string
	Debit           string
	Credit          string
}
type JournalEntryView struct {
	JournalID       string
	Currency        string
	Status          string
	BookedAtRFC3339 string
	ExternalRef     string
	Lines           []JournalLine
}
type ListAccountJournalEntriesOutput struct {
	Entries []JournalEntryView
	Page    int32
	Size    int32
}

type CounterpartyView struct {
	AccountID     string
	AccountNumber string
	DisplayName   string
	AccountType   string
}

type StatementItem struct {
	JournalID             string
	BookedAtRFC3339       string
	Currency              string
	Direction             string
	Amount                string
	Kind                  string
	Memo                  *string
	CounterpartyAccountID *string
	Counterparty          *CounterpartyView
}

type ListAccountStatementInput struct {
	AccountID           string
	FromRFC3339         *string
	ToRFC3339           *string
	Page                int32
	Size                int32
	IncludeCounterparty bool
}

type ListAccountStatementOutput struct {
	Items []StatementItem
	Page  int32
	Size  int32
}

type LedgerPaymentsPort interface {
	PostPayment(ctx context.Context, in PostPaymentInput) (PostPaymentOutput, error)
	GetPayment(ctx context.Context, in GetPaymentInput) (GetPaymentOutput, error)

	ListAccountStatement(ctx context.Context, in ListAccountStatementInput) (ListAccountStatementOutput, error)
}
