// bff\internal\client\grpc\identity.go
package grpc

import (
	"context"
	"strings"
	"time"

	identityv1 "github.com/tagoKoder/bank-contracts/gen/go/bank/identity/v1"
	"github.com/tagoKoder/bff/internal/client/ports"
	"google.golang.org/grpc"
)

var _ ports.IdentityPort = (*IdentityClient)(nil)

type IdentityClient struct {
	oidc       identityv1.OidcAuthServiceClient
	onboarding identityv1.OnboardingServiceClient
	timeout    time.Duration
}

func NewIdentityClient(conn *grpc.ClientConn) *IdentityClient {
	return &IdentityClient{
		oidc:       identityv1.NewOidcAuthServiceClient(conn),
		onboarding: identityv1.NewOnboardingServiceClient(conn),
		timeout:    5 * time.Second,
	}
}

func (c *IdentityClient) StartOidcLogin(ctx context.Context, in ports.StartOidcLoginInput) (ports.StartOidcLoginOutput, error) {

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.oidc.StartOidcLogin(ctx2, &identityv1.StartOidcLoginRequest{
		Channel:            in.Channel,
		RedirectAfterLogin: in.RedirectAfterLogin,
	})
	if err != nil {
		return ports.StartOidcLoginOutput{}, err
	}
	return ports.StartOidcLoginOutput{AuthorizationURL: res.AuthorizationUrl, State: res.State}, nil
}

func (c *IdentityClient) CompleteOidcLogin(ctx context.Context, in ports.CompleteOidcLoginInput) (ports.CompleteOidcLoginOutput, error) {

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.oidc.CompleteOidcLogin(ctx2, &identityv1.CompleteOidcLoginRequest{
		Code:      in.Code,
		State:     in.State,
		Ip:        in.IP,
		UserAgent: in.UserAgent,
		Channel:   in.Channel,
	})
	if err != nil {
		return ports.CompleteOidcLoginOutput{}, err
	}

	u := ports.OidcUser{}
	if res.User != nil {
		u = ports.OidcUser{Name: res.User.Name, Email: res.User.Email, Roles: append([]string{}, res.User.Roles...)}
	}

	return ports.CompleteOidcLoginOutput{
		IdentityID:         res.IdentityId,
		SubjectIDOidc:      res.SubjectIdOidc,
		Provider:           res.Provider,
		User:               u,
		SessionID:          res.SessionId,
		SessionExpiresIn:   res.SessionExpiresIn,
		RedirectAfterLogin: res.RedirectAfterLogin,
	}, nil
}

func (c *IdentityClient) RefreshSession(ctx context.Context, in ports.RefreshSessionInput) (ports.RefreshSessionOutput, error) {
	//ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	//defer cancel()

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.oidc.RefreshSession(ctx2, &identityv1.RefreshSessionRequest{
		SessionId: in.SessionID,
		Ip:        in.IP,
		UserAgent: in.UserAgent,
	})
	if err != nil {
		return ports.RefreshSessionOutput{}, err
	}

	return ports.RefreshSessionOutput{
		SessionID:            res.SessionId,
		SessionExpiresIn:     res.SessionExpiresIn,
		AccessToken:          res.AccessToken,
		AccessTokenExpiresIn: res.AccessTokenExpiresIn,
	}, nil
}

func (c *IdentityClient) LogoutSession(ctx context.Context, in ports.LogoutSessionInput) (ports.LogoutSessionOutput, error) {

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.oidc.LogoutSession(ctx2, &identityv1.LogoutSessionRequest{SessionId: in.SessionID})
	if err != nil {
		return ports.LogoutSessionOutput{}, err
	}
	return ports.LogoutSessionOutput{Success: res.Success}, nil
}

func (c *IdentityClient) GetSessionInfo(ctx context.Context, in ports.GetSessionInfoInput) (ports.GetSessionInfoOutput, error) {

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.oidc.GetSessionInfo(ctx2, &identityv1.GetSessionInfoRequest{
		SessionId: in.SessionID,
		Ip:        in.IP,
		UserAgent: in.UserAgent,
	})
	if err != nil {
		return ports.GetSessionInfoOutput{}, err
	}

	u := ports.OidcUser{}
	if res.User != nil {
		u = ports.OidcUser{Name: res.User.Name, Email: res.User.Email, Roles: append([]string{}, res.User.Roles...)}
	}

	return ports.GetSessionInfoOutput{
		IdentityID:           res.IdentityId,
		SubjectIDOidc:        res.SubjectIdOidc,
		Provider:             res.Provider,
		User:                 u,
		CustomerID:           res.CustomerId,
		UserStatus:           res.UserStatus,
		SessionExpiresIn:     res.SessionExpiresIn,
		AccessToken:          res.AccessToken,
		AccessTokenExpiresIn: res.AccessTokenExpiresIn,
	}, nil
}

// ==========================
// Onboarding presigned
// ==========================

func (c *IdentityClient) StartRegistration(ctx context.Context, in ports.StartRegistrationInput) (ports.StartRegistrationOutput, error) {

	req := &identityv1.StartRegistrationRequest{
		Channel:             in.Channel,
		NationalId:          in.NationalID,
		NationalIdIssueDate: in.NationalIDIssueDate,
		FingerprintCode:     in.FingerprintCode,
		MonthlyIncome:       in.MonthlyIncome,
		OccupationType:      mapOccupation(in.OccupationType),
		Email:               in.Email,
		Phone:               in.Phone,
		IdFrontContentType:  strings.TrimSpace(in.IdFrontContentType),
		SelfieContentType:   strings.TrimSpace(in.SelfieContentType),
	}

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.onboarding.StartRegistration(ctx2, req)
	if err != nil {
		return ports.StartRegistrationOutput{}, err
	}

	created := ""
	if res.CreatedAt != nil {
		created = res.CreatedAt.AsTime().Format(time.RFC3339)
	}

	uploads := make([]ports.PresignedUpload, 0, len(res.Uploads))
	for _, u := range res.Uploads {
		hh := make([]ports.PresignedHeader, 0, len(u.Headers))
		for _, h := range u.Headers {
			hh = append(hh, ports.PresignedHeader{Name: h.Name, Value: h.Value})
		}
		uploads = append(uploads, ports.PresignedUpload{
			DocType:          mapDocTypeOut(u.DocType),
			Bucket:           u.Bucket,
			Key:              u.Key,
			UploadURL:        u.UploadUrl,
			Headers:          hh,
			MaxBytes:         u.MaxBytes,
			ContentType:      u.ContentType,
			ExpiresInSeconds: u.ExpiresInSeconds,
		})
	}

	return ports.StartRegistrationOutput{
		RegistrationID:   res.RegistrationId,
		State:            mapRegStateOut(res.State),
		CreatedAtRFC3339: created,
		Uploads:          uploads,
	}, nil
}

func (c *IdentityClient) ConfirmRegistrationKyc(ctx context.Context, in ports.ConfirmRegistrationKycInput) (ports.ConfirmRegistrationKycOutput, error) {

	objs := make([]*identityv1.UploadedObject, 0, len(in.Objects))
	for _, o := range in.Objects {
		objs = append(objs, &identityv1.UploadedObject{
			DocType:     mapDocTypeIn(o.DocType),
			Bucket:      o.Bucket,
			Key:         o.Key,
			Etag:        o.ETag,
			SizeBytes:   o.SizeBytes,
			ContentType: o.ContentType,
		})
	}

	ctx2, cancel := context.WithTimeout(ctx, c.timeout)

	defer cancel()

	res, err := c.onboarding.ConfirmRegistrationKyc(ctx2, &identityv1.ConfirmRegistrationKycRequest{
		RegistrationId: in.RegistrationID,
		Objects:        objs,
		Channel:        in.Channel,
	})
	if err != nil {
		return ports.ConfirmRegistrationKycOutput{}, err
	}

	confirmed := ""
	if res.ConfirmedAt != nil {
		confirmed = res.ConfirmedAt.AsTime().Format(time.RFC3339)
	}

	statuses := make([]ports.KycObjectStatus, 0, len(res.Statuses))
	for _, st := range res.Statuses {
		statuses = append(statuses, ports.KycObjectStatus{
			DocType: mapDocTypeOut(st.DocType),
			Status:  mapUploadStatusOut(st.Status),
			Bucket:  st.Bucket,
			Key:     st.Key,
			ETag:    st.Etag,
		})
	}

	return ports.ConfirmRegistrationKycOutput{
		RegistrationID:     res.RegistrationId,
		State:              mapRegStateOut(res.State),
		Statuses:           statuses,
		ConfirmedAtRFC3339: confirmed,
	}, nil
}

func (c *IdentityClient) ActivateRegistration(ctx context.Context, in ports.ActivateRegistrationInput) (ports.ActivateRegistrationOutput, error) {
	ctx2, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, err := c.onboarding.ActivateRegistration(ctx2, &identityv1.ActivateRegistrationRequest{
		RegistrationId: in.RegistrationID,
		Channel:        in.Channel,
		FullName:       in.FullName,
		Tin:            in.Tin,
		BirthDate:      in.BirthDate,
		Country:        in.Country,
		Email:          in.Email,
		Phone:          in.Phone,
		AcceptedTerms:  in.AcceptedTerms,
	})
	if err != nil {
		return ports.ActivateRegistrationOutput{}, err
	}
	accounts := make([]ports.ActivatedAccount, 0, len(res.Accounts))
	for _, a := range res.Accounts {
		accounts = append(accounts, ports.ActivatedAccount{AccountID: a.AccountId, Currency: a.Currency, ProductType: a.ProductType})
	}
	return ports.ActivateRegistrationOutput{
		RegistrationId:   res.RegistrationId,
		CustomerId:       res.CustomerId,
		State:            mapRegStateOut(res.State),
		PrimaryAccountId: res.PrimaryAccountId,
		ActivationRef:    res.ActivationRef,
		Accounts:         accounts,
		BonusJournalId:   res.BonusJournalId,
		CorrelationID:    res.CorrelationId,
	}, nil
}

func mapOccupation(s string) identityv1.OccupationType {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "student":
		return identityv1.OccupationType_OCCUPATION_TYPE_STUDENT
	case "employee":
		return identityv1.OccupationType_OCCUPATION_TYPE_EMPLOYEE
	case "self_employed":
		return identityv1.OccupationType_OCCUPATION_TYPE_SELF_EMPLOYED
	case "unemployed":
		return identityv1.OccupationType_OCCUPATION_TYPE_UNEMPLOYED
	case "retired":
		return identityv1.OccupationType_OCCUPATION_TYPE_RETIRED
	default:
		return identityv1.OccupationType_OCCUPATION_TYPE_UNSPECIFIED
	}
}

func mapDocTypeIn(s string) identityv1.KycDocType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "id_front":
		return identityv1.KycDocType_KYC_DOC_TYPE_ID_FRONT
	case "selfie":
		return identityv1.KycDocType_KYC_DOC_TYPE_SELFIE
	default:
		return identityv1.KycDocType_KYC_DOC_TYPE_UNSPECIFIED
	}
}

func mapDocTypeOut(v identityv1.KycDocType) string {
	switch v {
	case identityv1.KycDocType_KYC_DOC_TYPE_ID_FRONT:
		return "id_front"
	case identityv1.KycDocType_KYC_DOC_TYPE_SELFIE:
		return "selfie"
	default:
		return "unspecified"
	}
}

func mapRegStateOut(v identityv1.RegistrationState) string {
	switch v {
	case identityv1.RegistrationState_REGISTRATION_STATE_STARTED:
		return "started"
	case identityv1.RegistrationState_REGISTRATION_STATE_CONTACT_VERIFIED:
		return "contact_verified"
	case identityv1.RegistrationState_REGISTRATION_STATE_CONSENTED:
		return "consented"
	case identityv1.RegistrationState_REGISTRATION_STATE_ACTIVATED:
		return "activated"
	case identityv1.RegistrationState_REGISTRATION_STATE_REJECTED:
		return "rejected"
	default:
		return "unspecified"
	}
}

func mapUploadStatusOut(v identityv1.KycUploadStatus) string {
	switch v {
	case identityv1.KycUploadStatus_KYC_UPLOAD_STATUS_PENDING:
		return "pending"
	case identityv1.KycUploadStatus_KYC_UPLOAD_STATUS_CONFIRMED:
		return "confirmed"
	default:
		return "unspecified"
	}
}
