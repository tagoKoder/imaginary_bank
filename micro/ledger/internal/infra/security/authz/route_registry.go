package authz

import "maps"

const (
	T_PAYMENT_NEW           = "PAYMENT_NEW"
	T_PAYMENT_BY_ID         = "PAYMENT_BY_ID"
	T_ACCOUNT_BY_ID         = "ACCOUNT_BY_ID"
	T_ACCOUNT_SYSTEM_CREDIT = "ACCOUNT_SYSTEM_CREDIT"
	T_ACCOUNT_STATEMENT     = "ACCOUNT_STATEMENT"
)

type Registry struct {
	routes map[string]RouteDef
}

func NewRegistry() *Registry {
	r := map[string]RouteDef{
		// Payments
		"/bank.ledgerpayments.v1.PaymentsService/PostPayment": {
			ActionID: "ledger.payments.post", Critical: true, Mode: ModeAuthz, RequireCustomerLink: true, ResourceTemplate: T_PAYMENT_NEW,
		},
		"/bank.ledgerpayments.v1.PaymentsService/GetPayment": {
			ActionID: "ledger.payments.get", Critical: false, Mode: ModeAuthz, RequireCustomerLink: true, ResourceTemplate: T_PAYMENT_BY_ID,
		},

		// Ledger user-facing credit (si existe) - normalmente AUTHZ
		"/bank.ledgerpayments.v1.LedgerService/CreditAccount": {
			ActionID: "ledger.accounts.credit", Critical: true, Mode: ModeAuthz, RequireCustomerLink: true, ResourceTemplate: T_ACCOUNT_BY_ID,
		},

		// System credit: SOLO service/system (igual AVP pero con template diferente)
		"/bank.ledgerpayments.v1.LedgerService/CreditAccountSystem": {
			ActionID: "ledger.accounts.credit_system", Critical: true, Mode: ModeServiceOnly, RequireCustomerLink: false, ResourceTemplate: T_ACCOUNT_SYSTEM_CREDIT,
		},

		"/bank.ledgerpayments.v1.LedgerService/ListAccountJournalEntries": {
			ActionID: "ledger.journals.list", Critical: false, Mode: ModeAuthz, RequireCustomerLink: true, ResourceTemplate: T_ACCOUNT_BY_ID,
		},

		"/bank.ledgerpayments.v1.LedgerService/ListAccountStatement": {
			ActionID: "ledger.statement.list", Critical: false, Mode: ModeAuthz, RequireCustomerLink: true, ResourceTemplate: T_ACCOUNT_STATEMENT,
		},
	}

	return &Registry{routes: r}
}

func (r *Registry) Get(fullMethod string) (RouteDef, bool) {
	d, ok := r.routes[fullMethod]
	return d, ok
}

func (r *Registry) Clone() map[string]RouteDef { return maps.Clone(r.routes) }
