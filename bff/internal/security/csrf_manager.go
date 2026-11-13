// bff\internal\security\csrf_manager.go
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tagoKoder/bff/internal/config"
)

// Uses signed double-submit CSRF tokens bound to a particular session ID.
// All BFF instances must use the same CSRF_SECRET, rotated via deployment policy.
type CSRFManager struct {
	secure            bool
	sameSite          http.SameSite
	cookieName        string
	sessionCookieName string
	headerName        string
	ttl               time.Duration
	secret            []byte
}

func NewCSRFManager(cfg config.Config) *CSRFManager {
	ss := http.SameSiteLaxMode
	switch cfg.CookieSameSite {
	case "strict":
		ss = http.SameSiteStrictMode
	case "none":
		if cfg.CookieSecure {
			ss = http.SameSiteNoneMode
		}
	}
	cookieName := "bff_csrf"
	sessionName := "bff_sid"
	if cfg.CookieSecure {
		cookieName = "__Host-bff_csrf"
		sessionName = "__Host-bff_sid"
	}
	secret := []byte(cfg.CSRFSecret)
	if len(secret) < 32 {
		if cfg.Env != "local" {
			panic("CSRF_SECRET must be at least 32 bytes in non-local deployments")
		}
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			panic(err)
		}
	}
	return &CSRFManager{secure: cfg.CookieSecure, sameSite: ss, cookieName: cookieName, sessionCookieName: sessionName, headerName: "X-CSRF-Token", ttl: 24 * time.Hour, secret: secret}
}

func (m *CSRFManager) sign(sid, nonce string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(sid))
	mac.Write([]byte("."))
	mac.Write([]byte(nonce))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *CSRFManager) valid(token, sid string) bool {
	if sid == "" {
		return false
	}
	fields := strings.Split(token, ".")
	if len(fields) != 2 {
		return false
	}
	nonceBytes, e1 := base64.RawURLEncoding.DecodeString(fields[0])
	sigBytes, e2 := base64.RawURLEncoding.DecodeString(fields[1])
	if e1 != nil || e2 != nil || len(nonceBytes) != 32 || len(sigBytes) != sha256.Size {
		return false
	}
	expected, _ := base64.RawURLEncoding.DecodeString(m.sign(sid, fields[0]))
	return subtle.ConstantTimeCompare(sigBytes, expected) == 1
}

func (m *CSRFManager) cookieToken(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: m.cookieName, Value: token, Path: "/", HttpOnly: true, Secure: m.secure, SameSite: m.sameSite, MaxAge: int(m.ttl.Seconds()), Expires: time.Now().Add(m.ttl)})
}

// IssueForSession is used on login and whenever Identity rotates the session.
func (m *CSRFManager) IssueForSession(w http.ResponseWriter, sid string) (string, error) {
	if sid == "" {
		return "", errors.New("missing session")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	nonce := base64.RawURLEncoding.EncodeToString(buf)
	token := nonce + "." + m.sign(sid, nonce)
	m.cookieToken(w, token)
	return token, nil
}

func (m *CSRFManager) sessionID(r *http.Request) string {
	ck, err := r.Cookie(m.sessionCookieName)
	if err != nil {
		return ""
	}
	return ck.Value
}

func (m *CSRFManager) ReadToken(r *http.Request) (string, bool) {
	ck, err := r.Cookie(m.cookieName)
	if err != nil || ck.Value == "" {
		return "", false
	}
	return ck.Value, true
}

// Returns a valid token also on the first request, without reading a Set-Cookie from r.
func (m *CSRFManager) GetOrCreateTokenForSession(w http.ResponseWriter, r *http.Request, sid string) (string, error) {
	if sid == "" {
		return "", errors.New("missing authenticated session")
	}
	if tok, ok := m.ReadToken(r); ok && m.valid(tok, sid) {
		return tok, nil
	}
	return m.IssueForSession(w, sid)
}

func (m *CSRFManager) GetOrCreateToken(w http.ResponseWriter, r *http.Request) (string, error) {
	sid := m.sessionID(r)
	return m.GetOrCreateTokenForSession(w, r, sid)
}

func (m *CSRFManager) EnsureToken(w http.ResponseWriter, r *http.Request) error {
	_, err := m.GetOrCreateToken(w, r)
	return err
}

func (m *CSRFManager) Validate(r *http.Request) bool {
	token, ok := m.ReadToken(r)
	if !ok || !m.valid(token, m.sessionID(r)) {
		return false
	}
	header := r.Header.Get(m.headerName)
	return header != "" && subtle.ConstantTimeCompare([]byte(token), []byte(header)) == 1
}

func (m *CSRFManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: m.cookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.secure, SameSite: m.sameSite, MaxAge: -1, Expires: time.Unix(0, 0)})
}
