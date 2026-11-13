// bff\internal\api\rest\v1\web\feature\onboarding\strict.go
package onboarding

import (
	"context"
	"time"

	"github.com/tagoKoder/bff/internal/util"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/security"
)

func (h *Handler) StartOnboardingStrict(
	ctx context.Context,
	body openapi.OnboardingIntentRequest,
) (openapi.StartOnboardingResponseObject, error) {
	cid := security.CorrelationID(ctx)
	if err := util.ValidateMoney(body.MonthlyIncome); err != nil {
		return openapi.StartOnboarding400JSONResponse(openapi.ErrorResponse{Code: "BAD_REQUEST", Message: "invalid monthly_income"}), nil
	}

	channel := "web"
	if body.Channel != nil && *body.Channel != "" {
		channel = *body.Channel
	}

	idCT := ""
	if body.IdFrontContentType != nil {
		idCT = *body.IdFrontContentType
	}
	selfieCT := ""
	if body.SelfieContentType != nil {
		selfieCT = *body.SelfieContentType
	}

	out, err := h.clients.Identity.StartRegistration(ctx, ports.StartRegistrationInput{
		//IdempotencyKey:     idemKey,
		Channel:             channel,
		Email:               string(body.Email),
		Phone:               body.Phone,
		NationalID:          body.NationalId,
		NationalIDIssueDate: body.NationalIdIssueDate,
		FingerprintCode:     body.FingerprintCode,
		MonthlyIncome:       body.MonthlyIncome,
		OccupationType:      string(body.OccupationType),
		IdFrontContentType:  idCT,
		SelfieContentType:   selfieCT,
	})
	if err != nil {
		return nil, err
	}

	uploads := make([]openapi.PresignedUpload, 0, len(out.Uploads))
	for _, u := range out.Uploads {
		hh := make([]openapi.PresignedHeader, 0, len(u.Headers))
		for _, hhh := range u.Headers {
			hh = append(hh, openapi.PresignedHeader{Name: hhh.Name, Value: hhh.Value})
		}

		docType := openapi.KycDocType("unspecified")
		if u.DocType == "id_front" {
			docType = openapi.KycDocType("id_front")
		} else if u.DocType == "selfie" {
			docType = openapi.KycDocType("selfie")
		}

		uploads = append(uploads, openapi.PresignedUpload{
			DocType:          docType,
			Bucket:           u.Bucket,
			Key:              u.Key,
			UploadUrl:        u.UploadURL,
			Headers:          hh,
			MaxBytes:         u.MaxBytes,
			ContentType:      u.ContentType,
			ExpiresInSeconds: u.ExpiresInSeconds,
		})
	}

	state := openapi.RegistrationState("unspecified")
	switch out.State {
	case "started":
		state = openapi.RegistrationState("started")
	case "contact_verified":
		state = openapi.RegistrationState("contact_verified")
	case "consented":
		state = openapi.RegistrationState("consented")
	case "activated":
		state = openapi.RegistrationState("activated")
	case "rejected":
		state = openapi.RegistrationState("rejected")
	}

	var createdAtPtr *time.Time
	if out.CreatedAtRFC3339 != "" {
		if t, e := time.Parse(time.RFC3339, out.CreatedAtRFC3339); e == nil {
			createdAtPtr = &t
		}
	}

	resp := openapi.OnboardingIntentResponse{
		RegistrationId: out.RegistrationID,
		State:          state,
		Uploads:        uploads,
		CreatedAt:      createdAtPtr,
		// otp_channel_hint queda opcional/legacy
	}

	// opcional: si también quieres exponerlo en el body (legacy)
	if cid != "" {
		resp.CorrelationId = &cid
	}

	return openapi.StartOnboarding201JSONResponse{
		Body: resp,
		Headers: openapi.StartOnboarding201ResponseHeaders{
			XCorrelationId: cid,
		},
	}, nil
}

func (h *Handler) ConfirmOnboardingKycStrict(
	ctx context.Context,
	body openapi.ConfirmKycRequest,
) (openapi.ConfirmOnboardingKycResponseObject, error) {

	cid := security.CorrelationID(ctx)

	channel := "web"
	if body.Channel != nil && *body.Channel != "" {
		channel = *body.Channel
	}

	objects := make([]ports.UploadedObject, 0, len(body.Objects))
	for _, o := range body.Objects {
		etag := ""
		if o.Etag != nil {
			etag = *o.Etag
		}
		ct := ""
		if o.ContentType != nil {
			ct = *o.ContentType
		}
		size := int64(0)
		if o.SizeBytes != nil {
			size = *o.SizeBytes
		}

		objects = append(objects, ports.UploadedObject{
			DocType:     string(o.DocType),
			Bucket:      o.Bucket,
			Key:         o.Key,
			ETag:        etag,
			SizeBytes:   size,
			ContentType: ct,
		})
	}

	out, err := h.clients.Identity.ConfirmRegistrationKyc(ctx, ports.ConfirmRegistrationKycInput{
		//IdempotencyKey: idemKey,
		RegistrationID: body.RegistrationId,
		Objects:        objects,
		Channel:        channel,
	})
	if err != nil {
		return nil, err
	}

	state := openapi.RegistrationState("unspecified")
	switch out.State {
	case "started":
		state = openapi.RegistrationState("started")
	case "contact_verified":
		state = openapi.RegistrationState("contact_verified")
	case "consented":
		state = openapi.RegistrationState("consented")
	case "activated":
		state = openapi.RegistrationState("activated")
	case "rejected":
		state = openapi.RegistrationState("rejected")
	}

	statuses := make([]openapi.KycObjectStatus, 0, len(out.Statuses))
	for _, st := range out.Statuses {
		docType := openapi.KycDocType("unspecified")
		if st.DocType == "id_front" {
			docType = openapi.KycDocType("id_front")
		} else if st.DocType == "selfie" {
			docType = openapi.KycDocType("selfie")
		}

		status := openapi.KycUploadStatus("unspecified")
		if st.Status == "pending" {
			status = openapi.KycUploadStatus("pending")
		} else if st.Status == "confirmed" {
			status = openapi.KycUploadStatus("confirmed")
		}

		var etagPtr *string
		if st.ETag != "" {
			etag := st.ETag
			etagPtr = &etag
		}

		statuses = append(statuses, openapi.KycObjectStatus{
			DocType: docType,
			Status:  status,
			Bucket:  st.Bucket,
			Key:     st.Key,
			Etag:    etagPtr,
		})
	}

	var confirmedAtPtr *time.Time
	if out.ConfirmedAtRFC3339 != "" {
		if t, e := time.Parse(time.RFC3339, out.ConfirmedAtRFC3339); e == nil {
			confirmedAtPtr = &t
		}
	}

	return openapi.ConfirmOnboardingKyc200JSONResponse{
		Body: openapi.ConfirmKycResponse{
			RegistrationId: out.RegistrationID,
			State:          state,
			Statuses:       statuses,
			ConfirmedAt:    confirmedAtPtr,
		},
		Headers: openapi.ConfirmOnboardingKyc200ResponseHeaders{
			XCorrelationId: cid},
	}, nil
}

func (h *Handler) ActivateOnboardingStrict(
	ctx context.Context,
	idemKey string, // si REQUIRED
	body openapi.ActivateRequest,
) (openapi.ActivateOnboardingResponseObject, error) {
	corrId := security.CorrelationID(ctx)
	// The HTTP header has no corresponding Proto field; durable deduplication is
	// based on registration_id / activation_ref inside Identity, Accounts and Ledger.
	if len(idemKey) < 8 || len(idemKey) > 128 {
		return openapi.ActivateOnboarding400JSONResponse(openapi.ErrorResponse{Code: "BAD_REQUEST", Message: "invalid idempotency key"}), nil
	}
	if !bool(body.AcceptedTerms) {
		return openapi.ActivateOnboarding400JSONResponse(
			openapi.ErrorResponse{
				Code:    "TERMS_NOT_ACCEPTED",
				Message: "You must accept the terms and conditions",
			},
		), nil
	}
	out, err := h.clients.Identity.ActivateRegistration(ctx, ports.ActivateRegistrationInput{
		//IdempotencyKey: idemKey,
		RegistrationID: body.RegistrationId,
		FullName:       body.FullName,
		Tin:            body.Tin,
		BirthDate:      body.BirthDate,
		Country:        body.Country,
		Email:          string(body.Email),
		Phone:          body.Phone,
		AcceptedTerms:  bool(body.AcceptedTerms),
		Channel:        "web",
	})
	if err != nil {
		return nil, err
	}

	resp := openapi.ActivateResponse{
		CustomerId:    out.CustomerId,
		AccountId:     out.PrimaryAccountId,
		ActivationRef: out.ActivationRef,
	}
	if cid := security.CorrelationID(ctx); cid != "" {
		resp.CorrelationId = &cid
	}
	return openapi.ActivateOnboarding201JSONResponse{
		Body: resp,
		Headers: openapi.ActivateOnboarding201ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}
