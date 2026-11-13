// bff\internal\client\ports\accounts.go
package ports

import "context"

type RiskSegment string

const (
	RiskLow    RiskSegment = "LOW"
	RiskMedium RiskSegment = "MEDIUM"
	RiskHigh   RiskSegment = "HIGH"
)

type CustomerStatus string

const (
	CustomerActive    CustomerStatus = "ACTIVE"
	CustomerSuspended CustomerStatus = "SUSPENDED"
)

type PreferenceChannel string

const (
	PrefEmail PreferenceChannel = "EMAIL"
	PrefSMS   PreferenceChannel = "SMS"
	PrefPush  PreferenceChannel = "PUSH"
)

type ProductType string

const (
	ProductChecking ProductType = "CHECKING"
	ProductSavings  ProductType = "SAVINGS"
)

type CustomerAddressCreate struct {
	Country    string
	Line1      string
	Line2      string
	City       string
	Province   string
	PostalCode string
}

type CreateCustomerInput struct {
	IdempotencyKey string
	FullName       string
	BirthDate      string
	TIN            string
	RiskSegment    RiskSegment
	Email          string
	Phone          string
	Address        *CustomerAddressCreate
	ExternalRef    *string
}
type CreateCustomerOutput struct {
	CustomerID string
}

type CustomerContactPatch struct {
	Email *string
	Phone *string
}
type PreferencesPatch struct {
	Channel *PreferenceChannel
	OptIn   *bool
}
type PatchCustomerInput struct {
	ID             string
	FullName       *string
	RiskSegment    *RiskSegment
	CustomerStatus *CustomerStatus
	Contact        *CustomerContactPatch
	Preferences    *PreferencesPatch
}
type PatchCustomerOutput struct {
	CustomerID string
}

type CreateAccountInput struct {
	CustomerID     string
	ProductType    ProductType
	Currency       string
	IdempotencyKey string
	ExternalRef    *string
}
type CreateAccountOutput struct {
	AccountID     string
	AccountNumber string
}

type Balances struct {
	Ledger    string
	Available string
	Hold      string
}
type AccountView struct {
	ID               string
	CustomerID       string
	AccountNumber    string
	ProductType      string
	Currency         string
	Status           string
	OpenedAtRFC3339  string
	UpdatedAtRFC3339 string
	Balances         *Balances
}

type ListAccountsInput struct {
	CustomerID string
}
type ListAccountsOutput struct {
	Accounts []AccountView
}

type GetAccountBalancesInput struct {
	ID string
}
type GetAccountBalancesOutput struct {
	AccountID string
	Balances  Balances
}

type PatchAccountLimitsInput struct {
	ID       string
	DailyOut *string
	DailyIn  *string
}
type PatchAccountLimitsOutput struct {
	AccountID string
	DailyOut  string
	DailyIn   string
}

// INTERNAL
type ValidateAccountsAndLimitsInput struct {
	SourceAccountID      string
	DestinationAccountID string
	Currency             string
	Amount               string
}
type ValidateAccountsAndLimitsOutput struct {
	OK     bool
	Reason *string
}

type HoldRequest struct {
	Currency string
	Amount   string
	Reason   *string
}

type ReserveHoldInput struct {
	AccountID      string
	Hold           HoldRequest
	HoldID         string
	IdempotencyKey string
}
type ReserveHoldOutput struct {
	OK      bool
	NewHold string
	HoldID  string
	Status  *string
}

type ReleaseHoldInput struct {
	AccountID      string
	Hold           HoldRequest
	HoldID         string
	IdempotencyKey string
}
type ReleaseHoldOutput struct {
	OK      bool
	NewHold string
	Status  *string
}

type GetAccountByNumberInput struct {
	AccountNumber   string
	IncludeInactive *bool
}

type AccountLookupView struct {
	AccountID     string
	AccountNumber string
	DisplayName   string
	ProductType   string
	Currency      string
	Status        string
	CustomerID    string
}

type GetAccountByNumberOutput struct {
	Account AccountLookupView
}

type AccountsPort interface {
	CreateCustomer(ctx context.Context, in CreateCustomerInput) (CreateCustomerOutput, error)
	PatchCustomer(ctx context.Context, in PatchCustomerInput) (PatchCustomerOutput, error)

	ListAccounts(ctx context.Context, in ListAccountsInput) (ListAccountsOutput, error)
	CreateAccount(ctx context.Context, in CreateAccountInput) (CreateAccountOutput, error)
	GetAccountBalances(ctx context.Context, in GetAccountBalancesInput) (GetAccountBalancesOutput, error)
	PatchAccountLimits(ctx context.Context, in PatchAccountLimitsInput) (PatchAccountLimitsOutput, error)

	GetAccountByNumber(ctx context.Context, in GetAccountByNumberInput) (GetAccountByNumberOutput, error)
}
