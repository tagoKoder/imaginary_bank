// bff\internal\api\rest\v1\web\feature\profile\strict.go
package profile

import (
	"context"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
	"github.com/tagoKoder/bff/internal/api/rest/middleware"
	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/security"
)

func (h *Handler) PatchStrict(
	ctx context.Context,
	params openapi.UpdateProfileParams,
	body openapi.PatchProfileRequest,
) (openapi.UpdateProfileResponseObject, error) {
	_ = params.XCSRFToken // validado arriba
	corrId := security.CorrelationID(ctx)
	customerID, _ := ctx.Value(middleware.CtxCustomer).(string)
	if customerID == "" {
		return openapi.UpdateProfile403JSONResponse(openapi.ErrorResponse{
			Code:    "FORBIDDEN",
			Message: "missing customer context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	var pref *ports.PreferencesPatch
	if body.Preferences != nil && len(*body.Preferences) > 0 {
		if len(*body.Preferences) > 1 {
			return openapi.UpdateProfile400JSONResponse(openapi.ErrorResponse{Code: "BAD_REQUEST", Message: "only one preference can be updated per request"}), nil
		}
		p0 := (*body.Preferences)[0]
		ch := mapPref(p0.Channel)
		pref = &ports.PreferencesPatch{
			Channel: &ch,
			OptIn:   &p0.OptIn,
		}
	}

	var contact *ports.CustomerContactPatch
	if body.Email != nil || body.Phone != nil {
		contact = &ports.CustomerContactPatch{Email: body.Email, Phone: body.Phone}
	}
	if body.FullName == nil && contact == nil && pref == nil {
		return openapi.UpdateProfile400JSONResponse(openapi.ErrorResponse{Code: "BAD_REQUEST", Message: "no profile fields provided"}), nil
	}
	_, err := h.clients.Accounts.PatchCustomer(ctx, ports.PatchCustomerInput{
		ID:          customerID,
		FullName:    body.FullName,
		Contact:     contact,
		Preferences: pref,
	})
	if err != nil {
		return openapi.UpdateProfile502JSONResponse(openapi.ErrorResponse{
			Code:    "UPSTREAM_ERROR",
			Message: "accounts service unavailable",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	return openapi.UpdateProfile200JSONResponse{
		Body: openapi.PatchProfileResponse{Updated: true},
		Headers: openapi.UpdateProfile200ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}

func mapPref(s openapi.PatchProfileRequestPreferencesChannel) ports.PreferenceChannel {
	switch s {
	case openapi.Email:
		return ports.PrefEmail
	case openapi.Sms:
		return ports.PrefSMS
	case openapi.Push:
		return ports.PrefPush
	default:
		return ports.PrefEmail
	}
}
