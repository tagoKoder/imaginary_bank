// bff\internal\client\ports\identity.go
package ports

import "context"

type OidcUser struct {
	Name  string
	Email string
	Roles []string
}

type StartOidcLoginInput struct {
	Channel            string
	RedirectAfterLogin string
}
type StartOidcLoginOutput struct {
	AuthorizationURL string
	State            string
}

type CompleteOidcLoginInput struct {
	Code      string
	State     string
	IP        string
	UserAgent string
	Channel   string
}
type CompleteOidcLoginOutput struct {
	IdentityID         string
	SubjectIDOidc      string
	Provider           string
	User               OidcUser
	SessionID          string
	SessionExpiresIn   int64
	RedirectAfterLogin string
}

type RefreshSessionInput struct {
	SessionID string
	IP        string
	UserAgent string
}
type RefreshSessionOutput struct {
	SessionID            string
	SessionExpiresIn     int64
	AccessToken          string
	AccessTokenExpiresIn int64
}

type LogoutSessionInput struct {
	SessionID string
}
type LogoutSessionOutput struct {
	Success bool
}

type GetSessionInfoInput struct {
	SessionID string
	IP        string
	UserAgent string
}
type GetSessionInfoOutput struct {
	IdentityID           string
	SubjectIDOidc        string
	Provider             string
	User                 OidcUser
	CustomerID           string
	UserStatus           string // ACTIVE|LOCKED|DISABLED
	SessionExpiresIn     int64
	AccessToken          string
	AccessTokenExpiresIn int64
}

// ==========================
// Onboarding (Presigned S3)
// ==========================

type PresignedHeader struct {
	Name  string
	Value string
}

type PresignedUpload struct {
	DocType          string // id_front|selfie
	Bucket           string
	Key              string
	UploadURL        string
	Headers          []PresignedHeader
	MaxBytes         int64
	ContentType      string
	ExpiresInSeconds int64
}

type StartRegistrationInput struct {
	Channel             string
	NationalID          string
	NationalIDIssueDate string
	FingerprintCode     string

	MonthlyIncome  string
	OccupationType string // student|employee|self_employed|unemployed|retired

	Email string
	Phone string

	IdFrontContentType string
	SelfieContentType  string
}

type StartRegistrationOutput struct {
	RegistrationID   string
	State            string // started|...
	CreatedAtRFC3339 string
	Uploads          []PresignedUpload
}

type UploadedObject struct {
	DocType     string // id_front|selfie
	Bucket      string
	Key         string
	ETag        string
	SizeBytes   int64
	ContentType string
}

type KycObjectStatus struct {
	DocType string // id_front|selfie
	Status  string // pending|confirmed|...
	Bucket  string
	Key     string
	ETag    string
}

type ConfirmRegistrationKycInput struct {
	RegistrationID string
	Objects        []UploadedObject
	Channel        string
}

type ConfirmRegistrationKycOutput struct {
	RegistrationID     string
	State              string
	Statuses           []KycObjectStatus
	ConfirmedAtRFC3339 string
}

type ActivateRegistrationInput struct {
	RegistrationID string
	Channel        string
	FullName       string
	Tin            string
	BirthDate      string
	Country        string
	Email          string
	Phone          string
	AcceptedTerms  bool
}

type ActivatedAccount struct {
	AccountID   string
	Currency    string
	ProductType string
}

type ActivateRegistrationOutput struct {
	RegistrationId   string
	State            string
	CustomerId       string
	PrimaryAccountId string
	ActivationRef    string
	Accounts         []ActivatedAccount
	BonusJournalId   string
	CorrelationID    string
}

type IdentityPort interface {
	StartOidcLogin(ctx context.Context, in StartOidcLoginInput) (StartOidcLoginOutput, error)
	CompleteOidcLogin(ctx context.Context, in CompleteOidcLoginInput) (CompleteOidcLoginOutput, error)
	RefreshSession(ctx context.Context, in RefreshSessionInput) (RefreshSessionOutput, error)
	LogoutSession(ctx context.Context, in LogoutSessionInput) (LogoutSessionOutput, error)
	GetSessionInfo(ctx context.Context, in GetSessionInfoInput) (GetSessionInfoOutput, error)

	// Onboarding presigned
	StartRegistration(ctx context.Context, in StartRegistrationInput) (StartRegistrationOutput, error)
	ConfirmRegistrationKyc(ctx context.Context, in ConfirmRegistrationKycInput) (ConfirmRegistrationKycOutput, error)
	ActivateRegistration(ctx context.Context, in ActivateRegistrationInput) (ActivateRegistrationOutput, error)
}
