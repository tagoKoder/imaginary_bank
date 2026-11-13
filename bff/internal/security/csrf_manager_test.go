package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tagoKoder/bff/internal/config"
)

func newTestCSRF() *CSRFManager {
	return NewCSRFManager(config.Config{Env: "local", CSRFSecret: "0123456789abcdef0123456789abcdef"})
}
func TestCSRFBoundToSessionAndRotates(t *testing.T) {
	m := newTestCSRF()
	w := httptest.NewRecorder()
	tok, err := m.IssueForSession(w, "sid-one")
	if err != nil || tok == "" {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", nil)
	r.AddCookie(&http.Cookie{Name: "bff_sid", Value: "sid-one"})
	for _, c := range w.Result().Cookies() {
		r.AddCookie(c)
	}
	r.Header.Set("X-CSRF-Token", tok)
	if !m.Validate(r) {
		t.Fatal("valid token rejected")
	}
	r.Header.Set("X-CSRF-Token", "wrong")
	if m.Validate(r) {
		t.Fatal("wrong header accepted")
	}
	r.Header.Set("X-CSRF-Token", tok)
	rr := httptest.NewRequest(http.MethodPost, "/api/v1/payments", nil)
	rr.AddCookie(&http.Cookie{Name: "bff_sid", Value: "sid-two"})
	for _, c := range w.Result().Cookies() {
		rr.AddCookie(c)
	}
	rr.Header.Set("X-CSRF-Token", tok)
	if m.Validate(rr) {
		t.Fatal("old csrf token accepted for new session")
	}
}
func TestCSRFGetFirstToken(t *testing.T) {
	m := newTestCSRF()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/session/csrf", nil)
	r.AddCookie(&http.Cookie{Name: "bff_sid", Value: "sid-one"})
	w := httptest.NewRecorder()
	tok, err := m.GetOrCreateToken(w, r)
	if err != nil || tok == "" {
		t.Fatalf("cannot issue token: %v", err)
	}
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("Set-Cookie absent")
	}
}
