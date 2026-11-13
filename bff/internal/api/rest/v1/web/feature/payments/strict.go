// bff\internal\api\rest\v1\web\feature\payments\strict.go
package payments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tagoKoder/bff/internal/util"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
	"github.com/tagoKoder/bff/internal/api/rest/middleware"
	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/security"
)

func (h *Handler) CreatePaymentStrict(
	ctx context.Context,
	params openapi.ExecutePaymentParams,
	body openapi.CreatePaymentRequest,
) (openapi.ExecutePaymentResponseObject, error) {
	corrId := security.CorrelationID(ctx)
	customerID, _ := ctx.Value(middleware.CtxCustomer).(string)
	if customerID == "" {
		return openapi.ExecutePayment403JSONResponse(openapi.ErrorResponse{
			Code:    "FORBIDDEN",
			Message: "missing customer context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	sub, _ := ctx.Value(middleware.CtxSubject).(string)
	if sub == "" {
		return openapi.ExecutePayment401JSONResponse(openapi.ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "missing subject context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	if err := util.ValidatePositiveMoney(body.Amount); err != nil {
		return openapi.ExecutePayment400JSONResponse(openapi.ErrorResponse{Code: "BAD_REQUEST", Message: "invalid amount"}), nil
	}
	if _, err := uuid.Parse(body.SourceAccount); err != nil {
		return openapi.ExecutePayment400JSONResponse(openapi.ErrorResponse{Code: "BAD_REQUEST", Message: "invalid source account"}), nil
	}
	if _, err := uuid.Parse(body.DestinationAccount); err != nil || body.SourceAccount == body.DestinationAccount {
		return openapi.ExecutePayment400JSONResponse(openapi.ErrorResponse{Code: "BAD_REQUEST", Message: "invalid destination account"}), nil
	}

	// Ejecuta pago (micro LedgerPayments)
	out, err := h.clients.LedgerPayments.PostPayment(ctx, ports.PostPaymentInput{
		IdempotencyKey:       params.IdempotencyKey,
		SourceAccountID:      body.SourceAccount,
		DestinationAccountID: body.DestinationAccount,
		Currency:             body.Currency,
		Amount:               body.Amount,
		InitiatedBy:          sub,
	})
	if err != nil {
		return nil, err
	}

	if out.Status != "processing" && out.Status != "posted" && out.Status != "failed" {
		return nil, fmt.Errorf("payments response contains invalid status")
	}
	resp := openapi.CreatePaymentResponse{
		PaymentId: out.PaymentID,
		Status:    mapPaymentStatus(out.Status),
	}
	return openapi.ExecutePayment201JSONResponse{
		Body: resp,
		Headers: openapi.ExecutePayment201ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}

func (h *Handler) GetPaymentStrict(
	ctx context.Context,
	id string,
) (openapi.GetPaymentStatusResponseObject, error) {
	customerID, _ := ctx.Value(middleware.CtxCustomer).(string)
	corrId := security.CorrelationID(ctx)
	if customerID == "" {
		return openapi.GetPaymentStatus403JSONResponse(openapi.ErrorResponse{
			Code:    "FORBIDDEN",
			Message: "missing customer context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	out, err := h.clients.LedgerPayments.GetPayment(ctx, ports.GetPaymentInput{PaymentID: id})
	if err != nil {
		return nil, err
	}

	createdAt, err := time.Parse(time.RFC3339, out.CreatedAtRFC3339)
	if err != nil {
		return nil, err
	}

	steps := make([]openapi.PaymentStep, 0, len(out.Steps))
	for _, s := range out.Steps {
		at, err := time.Parse(time.RFC3339, s.AttemptedAtRFC3339)
		if err != nil {
			return nil, err
		}
		var details *string
		if strings.TrimSpace(s.DetailsJSON) != "" {
			d := s.DetailsJSON
			details = &d
		}
		steps = append(steps, openapi.PaymentStep{
			Step:        s.Step,
			State:       s.State,
			AttemptedAt: at,
			DetailsJson: details,
		})
	}

	resp := openapi.GetPaymentResponse{
		PaymentId:          out.PaymentID,
		Status:             out.Status,
		SourceAccount:      out.SourceAccountID,
		DestinationAccount: out.DestinationAccountID,
		Currency:           out.Currency,
		Amount:             out.Amount,
		Steps:              steps,
		CreatedAt:          createdAt,
	}

	return openapi.GetPaymentStatus200JSONResponse{
		Body: resp,
		Headers: openapi.GetPaymentStatus200ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}

func mapPaymentStatus(s string) openapi.CreatePaymentResponseStatus {
	// PaymentsService.PostPayment: posted|processing|failed. Unknown values are
	// rejected before returning success; see validation below.
	return openapi.CreatePaymentResponseStatus(s)
}
