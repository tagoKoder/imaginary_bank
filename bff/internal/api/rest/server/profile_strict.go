// bff\internal\api\rest\server\profile_strict.go
package server

import (
	"context"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
)

func (s *Server) UpdateProfile(
	ctx context.Context,
	req openapi.UpdateProfileRequestObject,
) (openapi.UpdateProfileResponseObject, error) {
	return s.profile.PatchStrict(ctx, req.Params, *req.Body)
}
