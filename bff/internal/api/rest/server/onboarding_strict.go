// bff\internal\api\rest\server\onboarding_strict.go
package server

import (
	"context"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
)

func (s *Server) StartOnboarding(
	ctx context.Context,
	req openapi.StartOnboardingRequestObject,
) (openapi.StartOnboardingResponseObject, error) {
	return s.onboarding.StartOnboardingStrict(ctx, *req.Body)
}

func (s *Server) ConfirmOnboardingKyc(
	ctx context.Context,
	req openapi.ConfirmOnboardingKycRequestObject,
) (openapi.ConfirmOnboardingKycResponseObject, error) {
	return s.onboarding.ConfirmOnboardingKycStrict(ctx, *req.Body)
}

func (s *Server) ActivateOnboarding(
	ctx context.Context,
	req openapi.ActivateOnboardingRequestObject,
) (openapi.ActivateOnboardingResponseObject, error) {
	return s.onboarding.ActivateOnboardingStrict(ctx, req.Params.IdempotencyKey, *req.Body)
}
