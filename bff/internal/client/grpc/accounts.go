// bff\internal\client\grpc\accounts.go
package grpc

import (
	"context"
	"time"

	accountsv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/accounts/v1"
	"github.com/tagoKoder/bff/internal/client/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ ports.AccountsPort = (*AccountsClient)(nil)

type AccountsClient struct {
	customers accountsv1.CustomersServiceClient
	accounts  accountsv1.AccountsServiceClient
	timeout   time.Duration
}

func NewAccountsClient(conn *grpc.ClientConn) *AccountsClient {
	return &AccountsClient{
		customers: accountsv1.NewCustomersServiceClient(conn),
		accounts:  accountsv1.NewAccountsServiceClient(conn),
		timeout:   5 * time.Second,
	}
}

func (c *AccountsClient) CreateCustomer(ctx context.Context, in ports.CreateCustomerInput) (ports.CreateCustomerOutput, error) {
	/*ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()*/

	req := &accountsv1.CreateCustomerRequest{
		FullName:       in.FullName,
		BirthDate:      in.BirthDate,
		Tin:            in.TIN,
		RiskSegment:    mapRisk(in.RiskSegment),
		Email:          in.Email,
		Phone:          in.Phone,
		IdempotencyKey: in.IdempotencyKey,
		ExternalRef:    ptrString(in.ExternalRef),
	}
	if in.Address != nil {
		req.Address = &accountsv1.CustomerAddressCreate{
			Country:    in.Address.Country,
			Line1:      in.Address.Line1,
			Line2:      in.Address.Line2,
			City:       in.Address.City,
			Province:   in.Address.Province,
			PostalCode: in.Address.PostalCode,
		}
	}
	ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, err := c.customers.CreateCustomer(ctx2, req)
	if err != nil {
		return ports.CreateCustomerOutput{}, err
	}
	return ports.CreateCustomerOutput{CustomerID: res.CustomerId}, nil
}

func (c *AccountsClient) PatchCustomer(ctx context.Context, in ports.PatchCustomerInput) (ports.PatchCustomerOutput, error) {
	/*ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()*/

	req := &accountsv1.PatchCustomerRequest{Id: in.ID}
	if in.FullName != nil {
		req.FullName = wrapperspb.String(*in.FullName)
	}
	if in.RiskSegment != nil {
		req.RiskSegment = &accountsv1.RiskSegmentValue{Value: mapRisk(*in.RiskSegment)}
	}
	if in.CustomerStatus != nil {
		req.CustomerStatus = &accountsv1.CustomerStatusValue{Value: mapCustStatus(*in.CustomerStatus)}
	}
	if in.Contact != nil {
		req.Contact = &accountsv1.CustomerContactPatch{
			Email: strWrap(in.Contact.Email),
			Phone: strWrap(in.Contact.Phone),
		}
	}
	if in.Preferences != nil {
		req.Preferences = &accountsv1.PreferencesPatch{
			Channel: prefChanWrap(in.Preferences.Channel),
			OptIn:   boolWrap(in.Preferences.OptIn),
		}
	}

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.customers.PatchCustomer(ctx2, req)
	if err != nil {
		return ports.PatchCustomerOutput{}, err
	}
	return ports.PatchCustomerOutput{CustomerID: res.CustomerId}, nil
}

func (c *AccountsClient) ListAccounts(ctx context.Context, in ports.ListAccountsInput) (ports.ListAccountsOutput, error) {
	/*ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()*/

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.accounts.ListAccounts(ctx2, &accountsv1.ListAccountsRequest{CustomerId: in.CustomerID})
	if err != nil {
		return ports.ListAccountsOutput{}, err
	}

	out := ports.ListAccountsOutput{Accounts: make([]ports.AccountView, 0, len(res.Accounts))}
	for _, a := range res.Accounts {
		v := ports.AccountView{
			ID:            a.Id,
			CustomerID:    a.CustomerId,
			AccountNumber: a.AccountNumber,
			ProductType:   a.ProductType,
			Currency:      a.Currency,
			Status:        a.Status,
		}
		if a.OpenedAt != nil {
			v.OpenedAtRFC3339 = a.OpenedAt.AsTime().Format(time.RFC3339)
		}
		if a.UpdatedAt != nil {
			v.UpdatedAtRFC3339 = a.UpdatedAt.AsTime().Format(time.RFC3339)
		}
		if a.Balances != nil {
			v.Balances = &ports.Balances{
				Ledger:    a.Balances.Ledger,
				Available: a.Balances.Available,
				Hold:      a.Balances.Hold,
			}
		}
		out.Accounts = append(out.Accounts, v)
	}

	return out, nil
}

func (c *AccountsClient) CreateAccount(ctx context.Context, in ports.CreateAccountInput) (ports.CreateAccountOutput, error) {
	/*ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()*/

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.accounts.CreateAccount(ctx2, &accountsv1.CreateAccountRequest{
		CustomerId:     in.CustomerID,
		ProductType:    mapProduct(in.ProductType),
		Currency:       in.Currency,
		IdempotencyKey: in.IdempotencyKey,
		ExternalRef:    ptrString(in.ExternalRef),
	})
	if err != nil {
		return ports.CreateAccountOutput{}, err
	}
	return ports.CreateAccountOutput{AccountID: res.AccountId, AccountNumber: res.AccountNumber}, nil
}

func (c *AccountsClient) GetAccountBalances(ctx context.Context, in ports.GetAccountBalancesInput) (ports.GetAccountBalancesOutput, error) {
	/*ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()*/

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.accounts.GetAccountBalances(ctx2, &accountsv1.GetAccountBalancesRequest{Id: in.ID})
	if err != nil {
		return ports.GetAccountBalancesOutput{}, err
	}
	b := ports.Balances{Ledger: "0", Available: "0", Hold: "0"}
	if res.Balances != nil {
		b = ports.Balances{Ledger: res.Balances.Ledger, Available: res.Balances.Available, Hold: res.Balances.Hold}
	}
	return ports.GetAccountBalancesOutput{AccountID: res.AccountId, Balances: b}, nil
}

func (c *AccountsClient) PatchAccountLimits(ctx context.Context, in ports.PatchAccountLimitsInput) (ports.PatchAccountLimitsOutput, error) {
	/*ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()*/

	req := &accountsv1.PatchAccountLimitsRequest{Id: in.ID}
	if in.DailyOut != nil {
		req.DailyOut = wrapperspb.String(*in.DailyOut)
	}
	if in.DailyIn != nil {
		req.DailyIn = wrapperspb.String(*in.DailyIn)
	}

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.accounts.PatchAccountLimits(ctx2, req)
	if err != nil {
		return ports.PatchAccountLimitsOutput{}, err
	}
	return ports.PatchAccountLimitsOutput{
		AccountID: res.AccountId,
		DailyOut:  res.DailyOut,
		DailyIn:   res.DailyIn,
	}, nil
}

func (c *AccountsClient) GetAccountByNumber(ctx context.Context, in ports.GetAccountByNumberInput) (ports.GetAccountByNumberOutput, error) {
	/*ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()*/

	req := &accountsv1.GetAccountByNumberRequest{
		AccountNumber: in.AccountNumber,
	}
	if in.IncludeInactive != nil {
		req.IncludeInactive = wrapperspb.Bool(*in.IncludeInactive)
	}

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.accounts.GetAccountByNumber(ctx2, req)
	if err != nil {
		return ports.GetAccountByNumberOutput{}, err
	}
	if res.Account == nil {
		return ports.GetAccountByNumberOutput{}, status.Error(codes.NotFound, "account not found")
	}

	// product_type es enum => lo pasamos como string consistente con tu OpenAPI
	pt := res.Account.ProductType.String()
	// opcional: normalizar a "CHECKING"/"SAVINGS"
	if pt == "PRODUCT_TYPE_CHECKING" {
		pt = "CHECKING"
	} else if pt == "PRODUCT_TYPE_SAVINGS" {
		pt = "SAVINGS"
	}

	return ports.GetAccountByNumberOutput{
		Account: ports.AccountLookupView{
			AccountID:     res.Account.AccountId,
			CustomerID:    res.Account.CustomerId,
			AccountNumber: res.Account.AccountNumber,
			DisplayName:   res.Account.DisplayName,
			ProductType:   pt,
			Currency:      res.Account.Currency,
			Status:        res.Account.Status,
		},
	}, nil
}

// helpers
func strWrap(p *string) *wrapperspb.StringValue {
	if p == nil {
		return nil
	}
	return wrapperspb.String(*p)
}
func boolWrap(p *bool) *wrapperspb.BoolValue {
	if p == nil {
		return nil
	}
	return wrapperspb.Bool(*p)
}
func prefChanWrap(p *ports.PreferenceChannel) *accountsv1.PreferenceChannelValue {
	if p == nil {
		return nil
	}
	return &accountsv1.PreferenceChannelValue{Value: mapPref(*p)}
}

func mapRisk(r ports.RiskSegment) accountsv1.RiskSegment {
	switch r {
	case ports.RiskLow:
		return accountsv1.RiskSegment_RISK_SEGMENT_LOW
	case ports.RiskMedium:
		return accountsv1.RiskSegment_RISK_SEGMENT_MEDIUM
	case ports.RiskHigh:
		return accountsv1.RiskSegment_RISK_SEGMENT_HIGH
	default:
		return accountsv1.RiskSegment_RISK_SEGMENT_UNSPECIFIED
	}
}
func mapCustStatus(s ports.CustomerStatus) accountsv1.CustomerStatus {
	switch s {
	case ports.CustomerActive:
		return accountsv1.CustomerStatus_CUSTOMER_STATUS_ACTIVE
	case ports.CustomerSuspended:
		return accountsv1.CustomerStatus_CUSTOMER_STATUS_SUSPENDED
	default:
		return accountsv1.CustomerStatus_CUSTOMER_STATUS_UNSPECIFIED
	}
}
func mapPref(p ports.PreferenceChannel) accountsv1.PreferenceChannel {
	switch p {
	case ports.PrefEmail:
		return accountsv1.PreferenceChannel_PREFERENCE_CHANNEL_EMAIL
	case ports.PrefSMS:
		return accountsv1.PreferenceChannel_PREFERENCE_CHANNEL_SMS
	case ports.PrefPush:
		return accountsv1.PreferenceChannel_PREFERENCE_CHANNEL_PUSH
	default:
		return accountsv1.PreferenceChannel_PREFERENCE_CHANNEL_UNSPECIFIED
	}
}
func mapProduct(p ports.ProductType) accountsv1.ProductType {
	switch p {
	case ports.ProductChecking:
		return accountsv1.ProductType_PRODUCT_TYPE_CHECKING
	case ports.ProductSavings:
		return accountsv1.ProductType_PRODUCT_TYPE_SAVINGS
	default:
		return accountsv1.ProductType_PRODUCT_TYPE_UNSPECIFIED
	}
}

func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
