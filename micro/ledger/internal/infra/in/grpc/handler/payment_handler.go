package handler

import (
	"context"

	ledgerpb "github.com/tagoKoder/bank-contracts/gen/go/bank/ledgerpayments/v1"
	in "github.com/tagoKoder/ledger/internal/domain/port/in"
	"github.com/tagoKoder/ledger/internal/infra/in/grpc/mapper"
	"github.com/tagoKoder/ledger/internal/infra/in/grpc/validation"
)

type PaymentsHandler struct {
	ledgerpb.UnimplementedPaymentsServiceServer
	post in.PostPaymentUseCase
	get  in.GetPaymentUseCase
}

func NewPaymentsHandler(post in.PostPaymentUseCase, get in.GetPaymentUseCase) *PaymentsHandler {
	return &PaymentsHandler{post: post, get: get}
}

func (h *PaymentsHandler) PostPayment(ctx context.Context, req *ledgerpb.PostPaymentRequest) (*ledgerpb.PostPaymentResponse, error) {
	cmd, err := validation.ToPostPaymentCommand(req)
	if err != nil {
		return nil, err
	}

	res, err := h.post.PostPayment(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return mapper.ToPostPaymentResponse(res), nil
}

func (h *PaymentsHandler) GetPayment(ctx context.Context, req *ledgerpb.GetPaymentRequest) (*ledgerpb.GetPaymentResponse, error) {
	paymentID, err := validation.ToGetPaymentUUID(req)
	if err != nil {
		return nil, err
	}

	res, err := h.get.GetPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	return mapper.ToGetPaymentResponse(res)
}
