// bff\internal\api\rest\v1\web\feature\accounts\strict.go
package accounts

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
	"github.com/tagoKoder/bff/internal/api/rest/middleware"
	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/security"
	"github.com/tagoKoder/bff/internal/util"
)

func (h *Handler) OverviewStrict(ctx context.Context) (openapi.GetAccountsOverviewResponseObject, error) {
	corrId := security.CorrelationID(ctx)
	customerID, _ := ctx.Value(middleware.CtxCustomer).(string)
	if customerID == "" {
		return openapi.GetAccountsOverview403JSONResponse(openapi.ErrorResponse{
			Code:    "FORBIDDEN",
			Message: "missing customer context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	out, err := h.clients.Accounts.ListAccounts(ctx, ports.ListAccountsInput{CustomerID: customerID})
	if err != nil {
		return nil, err
	}

	resp := openapi.AccountsOverviewResponse{Accounts: make([]openapi.AccountItem, 0, len(out.Accounts))}
	for _, a := range out.Accounts {
		if a.Balances == nil {
			return nil, fmt.Errorf("accounts response missing balances")
		}
		bal := openapi.AccountBalances{}
		if a.Balances != nil {
			bal = openapi.AccountBalances{
				Ledger:    a.Balances.Ledger,
				Available: a.Balances.Available,
				Hold:      a.Balances.Hold,
			}
		}
		resp.Accounts = append(resp.Accounts, openapi.AccountItem{
			Id:            a.ID,
			AccountNumber: &a.AccountNumber,
			ProductType:   a.ProductType,
			Currency:      a.Currency,
			Status:        a.Status,
			Balances:      bal,
		})
	}

	return openapi.GetAccountsOverview200JSONResponse{
		Body: resp,
		Headers: openapi.GetAccountsOverview200ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}

func (h *Handler) PatchLimitsStrict(
	ctx context.Context,
	accountID string,
	params openapi.PatchAccountLimitsParams,
	body openapi.PatchAccountLimitsHttpRequest,
) (openapi.PatchAccountLimitsResponseObject, error) {

	_ = params.XCSRFToken
	corrID := security.CorrelationID(ctx)

	badRequest := func(message string) openapi.PatchAccountLimitsResponseObject {
		return openapi.PatchAccountLimits400JSONResponse(
			openapi.ErrorResponse{
				Code:    "BAD_REQUEST",
				Message: message,
				Details: &map[string]interface{}{
					"correlation_id": corrID,
				},
			},
		)
	}

	// PATCH: both fields are optional, but at least
	// one must be provided.
	if body.DailyOut == nil && body.DailyIn == nil {
		return badRequest(
			"at least one limit must be provided",
		), nil
	}

	// Preserve nil to distinguish an omitted field
	// from a field explicitly set to zero.
	var dailyOut *string

	if body.DailyOut != nil {
		if err := util.ValidateMoney(*body.DailyOut); err != nil {
			return badRequest(
				"invalid daily_out value",
			), nil
		}

		dailyOut = body.DailyOut
	}

	var dailyIn *string

	if body.DailyIn != nil {
		if err := util.ValidateMoney(*body.DailyIn); err != nil {
			return badRequest(
				"invalid daily_in value",
			), nil
		}

		dailyIn = body.DailyIn
	}

	// Call Accounts microservice.
	out, err := h.clients.Accounts.PatchAccountLimits(
		ctx,
		ports.PatchAccountLimitsInput{
			ID:       accountID,
			DailyOut: dailyOut,
			DailyIn:  dailyIn,
		},
	)

	if err != nil {
		return nil, err
	}

	return openapi.PatchAccountLimits200JSONResponse{
		Body: openapi.PatchAccountLimitsHttpResponse{
			AccountId: out.AccountID,
			DailyOut:  out.DailyOut,
			DailyIn:   out.DailyIn,
		},
		Headers: openapi.PatchAccountLimits200ResponseHeaders{
			XCorrelationId: corrID,
		},
	}, nil
}

// =========================
// NUEVO: Lookup por account_number (Accounts micro)
// GET /api/v1/accounts/lookup?account_number=...
// =========================
func (h *Handler) LookupByNumberStrict(
	ctx context.Context,
	params openapi.LookupAccountByNumberParams,
) (openapi.LookupAccountByNumberResponseObject, error) {

	corrId := security.CorrelationID(ctx)

	// requerido por OpenAPI
	acctNum := params.AccountNumber
	if acctNum == "" {
		return openapi.LookupAccountByNumber400JSONResponse(openapi.ErrorResponse{
			Code:    "BAD_REQUEST",
			Message: "missing account_number",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	out, err := h.clients.Accounts.GetAccountByNumber(ctx, ports.GetAccountByNumberInput{
		AccountNumber: acctNum,
		// IncludeInactive: nil, // opcional si luego lo agregas
	})
	if err != nil {
		return nil, err
	}
	accountId, err := uuid.Parse(out.Account.AccountID)
	if err != nil {
		return nil, err
	}

	resp := openapi.AccountLookupResponse{
		Account: openapi.AccountLookupItem{
			AccountId:     accountId,
			AccountNumber: out.Account.AccountNumber,
			DisplayName:   out.Account.DisplayName,
			ProductType:   out.Account.ProductType,
			Currency:      out.Account.Currency,
			Status:        out.Account.Status,
		},
	}

	return openapi.LookupAccountByNumber200JSONResponse{
		Body: resp,
		Headers: openapi.LookupAccountByNumber200ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}

// =========================
// NUEVO: Statement / movimientos (LedgerPayments micro)
// GET /api/v1/accounts/{id}/statement
// =========================
func (h *Handler) StatementStrict(
	ctx context.Context,
	accountID string,
	params openapi.GetAccountStatementParams,
) (openapi.GetAccountStatementResponseObject, error) {

	corrId := security.CorrelationID(ctx)

	// defaults
	page := int32(1)
	size := int32(20)
	if params.Page != nil && *params.Page > 0 {
		page = int32(*params.Page)
	}
	if params.Size != nil && *params.Size > 0 {
		if *params.Size > 200 {
			size = 200
		} else {
			size = int32(*params.Size)
		}
	}

	// from/to vienen como date-time -> normalmente time.Time en openapi gen
	var fromRFC *string
	if params.From != nil {
		s := params.From.Format(time.RFC3339)
		fromRFC = &s
	}
	var toRFC *string
	if params.To != nil {
		s := params.To.Format(time.RFC3339)
		toRFC = &s
	}

	includeCP := false
	if params.IncludeCounterparty != nil {
		includeCP = *params.IncludeCounterparty
	}

	out, err := h.clients.LedgerPayments.ListAccountStatement(ctx, ports.ListAccountStatementInput{
		AccountID:           accountID,
		FromRFC3339:         fromRFC,
		ToRFC3339:           toRFC,
		Page:                page,
		Size:                size,
		IncludeCounterparty: includeCP,
	})
	if err != nil {
		return nil, err
	}

	items := make([]openapi.StatementItem, 0, len(out.Items))

	for _, it := range out.Items {
		t, err := time.Parse(time.RFC3339, it.BookedAtRFC3339)
		if err != nil {
			return nil, err
		}

		// CounterpartyAccountId opcional
		var cpAccountUUID *uuid.UUID
		if it.CounterpartyAccountID != nil && *it.CounterpartyAccountID != "" {
			parsed, err := uuid.Parse(*it.CounterpartyAccountID)
			if err != nil {
				return nil, err
			}
			cpAccountUUID = &parsed
		}

		// Counterparty opcional (solo si viene)
		var cp *openapi.CounterpartyView
		if it.Counterparty != nil {
			// si viene counterparty pero NO viene account id, igual no revientes:
			// usa cpAccountUUID si existe, si no deja AccountId en nil/zero según tu spec
			// (aquí asumo que AccountId NO es pointer en openapi y requiere valor)
			// The enriched object has its own account_id; do not manufacture uuid.Nil.
			cpID, parseErr := uuid.Parse(it.Counterparty.AccountID)
			if parseErr != nil || cpID == uuid.Nil {
				return nil, fmt.Errorf("ledger response contains invalid counterparty")
			}
			if cpAccountUUID != nil && *cpAccountUUID != cpID {
				return nil, fmt.Errorf("ledger response contains inconsistent counterparty")
			}
			cp = &openapi.CounterpartyView{
				AccountId:     cpID,
				AccountNumber: it.Counterparty.AccountNumber,
				DisplayName:   it.Counterparty.DisplayName,
				AccountType:   it.Counterparty.AccountType,
			}
		}

		items = append(items, openapi.StatementItem{
			JournalId:             it.JournalID,
			BookedAt:              t,
			Currency:              it.Currency,
			Direction:             openapi.StatementItemDirection(it.Direction),
			Amount:                it.Amount,
			Kind:                  it.Kind,
			Memo:                  it.Memo,
			CounterpartyAccountId: cpAccountUUID, // <- nil si no existe
			Counterparty:          cp,            // <- nil si no existe
		})
	}

	resp := openapi.AccountStatementResponse{
		Items: items,
		Page:  int(page),
		Size:  int(size),
		Total: nil, // el proto no trae total; lo dejamos null
	}

	return openapi.GetAccountStatement200JSONResponse{
		Body: resp,
		Headers: openapi.GetAccountStatement200ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}
