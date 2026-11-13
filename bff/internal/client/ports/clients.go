// bff\internal\client\ports\clients.go
package ports

type Clients struct {
	Identity       IdentityPort
	Accounts       AccountsPort
	LedgerPayments LedgerPaymentsPort
	Close          func() error
}
