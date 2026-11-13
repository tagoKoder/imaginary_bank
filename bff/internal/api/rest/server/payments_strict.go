// bff\internal\api\rest\server\payments_strict.go
package server

import (
	"context"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
)

func (s *Server) ExecutePayment(
	ctx context.Context,
	req openapi.ExecutePaymentRequestObject,
) (openapi.ExecutePaymentResponseObject, error) {
	return s.payments.CreatePaymentStrict(ctx, req.Params, *req.Body)
}

func (s *Server) GetPaymentStatus(
	ctx context.Context,
	req openapi.GetPaymentStatusRequestObject,
) (openapi.GetPaymentStatusResponseObject, error) {
	return s.payments.GetPaymentStrict(ctx, req.Id)
}
