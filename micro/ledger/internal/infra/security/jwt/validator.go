// micro\ledger\internal\infra\security\jwt\validator.go
package jwtvalidator

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tagoKoder/ledger/internal/infra/config"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingToken    = errors.New("missing bearer token")
	ErrInvalidToken    = errors.New("invalid token")
	ErrInvalidIssuer   = errors.New("invalid issuer")
	ErrInvalidAudience = errors.New("invalid audience")
)

type Claims struct {
	Subject    string
	CustomerID string
	Roles      []string
	MFA        bool

	Issuer    string
	Audience  []string
	ExpiresAt time.Time
	TokenUse  string
	ClientID  string
	Scopes    []string
}

type Validator struct {
	Issuer   string
	Audience string
	JWKSURL  string

	HTTPClient *http.Client
	CacheTTL   time.Duration

	mu             sync.Mutex
	cachedAt       time.Time
	keysByKID      map[string]*rsa.PublicKey
	allowedClients []string
	allowedScopes  []string
}

func New(issuer, audience, jwksURL string, cfg *config.Config) *Validator {
	return &Validator{
		Issuer:         issuer,
		Audience:       audience,
		JWKSURL:        jwksURL,
		HTTPClient:     &http.Client{Timeout: 3 * time.Second},
		CacheTTL:       10 * time.Minute,
		keysByKID:      map[string]*rsa.PublicKey{},
		allowedClients: cfg.LedgerSystemCreditClientIDs,
		allowedScopes:  cfg.LedgerSystemCreditScope,
	}
}

func (v *Validator) ValidateBearer(ctx context.Context, authzHeader string) (Claims, error) {
	raw := strings.TrimSpace(authzHeader)
	if raw == "" {
		return Claims{}, ErrMissingToken
	}
	if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		return Claims{}, ErrMissingToken
	}
	return v.ValidateToken(ctx, strings.TrimSpace(raw[len("bearer "):]))
}

func (v *Validator) ValidateToken(ctx context.Context, tokenStr string) (Claims, error) {
	if tokenStr == "" {
		return Claims{}, ErrMissingToken
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))
	var mc jwt.MapClaims

	keyFunc := func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, ErrInvalidToken
		}
		pub, err := v.getKeyByKID(ctx, kid)
		if err != nil {
			return nil, err
		}
		return pub, nil
	}

	tok, err := parser.ParseWithClaims(tokenStr, &mc, keyFunc)
	if err != nil || tok == nil || !tok.Valid {
		return Claims{}, ErrInvalidToken
	}

	iss, _ := mc["iss"].(string)
	if v.Issuer != "" && iss != v.Issuer {
		return Claims{}, ErrInvalidIssuer
	}

	// aud puede venir como string o array
	auds := extractAudience(mc["aud"])
	if v.Audience != "" && !contains(auds, v.Audience) {
		return Claims{}, ErrInvalidAudience
	}

	exp := extractTime(mc["exp"])
	if exp.IsZero() || !time.Now().UTC().Before(exp) {
		return Claims{}, ErrInvalidToken
	}

	sub, _ := mc["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return Claims{}, ErrInvalidToken
	}

	tokenUse, _ := mc["token_use"].(string)
	clientID, _ := mc["client_id"].(string)

	var scopes []string
	if rawScope, ok := mc["scope"].(string); ok {
		scopes = strings.Fields(rawScope)
	}

	roles := extractStringArray(mc["cognito:groups"])
	// customer_id: soporta "custom:customer_id" y "customer_id"
	cid, _ := mc["custom:customer_id"].(string)
	if cid == "" {
		cid, _ = mc["customer_id"].(string)
	}

	// MFA: Cognito access token suele traer "amr" (array) con "mfa" cuando aplica
	mfa := false
	for _, x := range extractStringArray(mc["amr"]) {
		if strings.EqualFold(x, "mfa") {
			mfa = true
			break
		}
	}

	return Claims{
		Subject:    sub,
		CustomerID: cid,
		Roles:      roles,
		MFA:        mfa,
		Issuer:     iss,
		Audience:   auds,
		ExpiresAt:  exp,
		TokenUse:   tokenUse,
		ClientID:   clientID,
		Scopes:     scopes,
	}, nil
}

func (v *Validator) getKeyByKID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now().UTC()
	if v.cachedAt.IsZero() || now.Sub(v.cachedAt) > v.CacheTTL || len(v.keysByKID) == 0 {
		keys, err := v.fetchJWKS(ctx)
		if err != nil {
			return nil, err
		}
		v.keysByKID = keys
		v.cachedAt = now
	}

	if k, ok := v.keysByKID[kid]; ok && k != nil {
		return k, nil
	}

	// refresh on miss
	keys, err := v.fetchJWKS(ctx)
	if err != nil {
		return nil, err
	}
	v.keysByKID = keys
	v.cachedAt = now

	k, ok := v.keysByKID[kid]
	if !ok || k == nil {
		return nil, fmt.Errorf("jwks kid not found: %s", kid)
	}
	return k, nil
}

type jwks struct {
	Keys []jwk `json:"keys"`
}
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

func (v *Validator) fetchJWKS(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	if v.JWKSURL == "" {
		return nil, errors.New("JWKS_URL is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.JWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jwks http %d", resp.StatusCode)
	}

	var data jwks
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	out := make(map[string]*rsa.PublicKey, len(data.Keys))
	for _, k := range data.Keys {
		if k.Kty != "RSA" || k.Kid == "" || k.N == "" || k.E == "" {
			continue
		}
		pub, err := rsaFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("jwks empty/invalid")
	}
	return out, nil
}

func rsaFromJWK(nB64URL, eB64URL string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(nB64URL)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(eB64URL)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nb)

	// e suele ser 65537 en bytes
	e := new(big.Int).SetBytes(eb).Int64()
	if e <= 0 || e > 1<<31-1 {
		return nil, errors.New("invalid exponent")
	}

	return &rsa.PublicKey{N: n, E: int(e)}, nil
}

func extractAudience(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func extractStringArray(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		return nil
	}
}

func extractTime(v any) time.Time {
	// jwt numeric date: float64 o json.Number
	switch t := v.(type) {
	case float64:
		return time.Unix(int64(t), 0).UTC()
	case json.Number:
		i, _ := t.Int64()
		if i > 0 {
			return time.Unix(i, 0).UTC()
		}
		return time.Time{}
	default:
		return time.Time{}
	}
}

func contains(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}

func (v *Validator) ValidateServiceBearer(
	ctx context.Context,
	authzHeader string,
) (Claims, error) {
	claims, err := v.ValidateBearer(ctx, authzHeader)
	if err != nil {
		return Claims{}, err
	}

	// Solo se aceptan access tokens de Cognito.
	if claims.TokenUse != "access" {
		return Claims{}, ErrInvalidToken
	}

	// Debe estar configurado el cliente de servicio autorizado.
	allowedClients := v.allowedClients

	clientAllowed := false
	for _, id := range allowedClients {
		if strings.TrimSpace(id) != "" &&
			strings.TrimSpace(id) == claims.ClientID {
			clientAllowed = true
			break
		}
	}
	if !clientAllowed {
		return Claims{}, ErrInvalidToken
	}

	allowedScopes := v.allowedScopes
	if len(allowedScopes) == 0 {
		return Claims{}, ErrInvalidToken
	}

	scopeAllowed := false
	for _, scope := range claims.Scopes {
		if contains(allowedScopes, scope) {
			scopeAllowed = true
			break
		}
	}
	if !scopeAllowed {
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}
