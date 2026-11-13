// bff\internal\security\cookies.go
package security

import (
	"net/http"
	"time"

	"github.com/tagoKoder/bff/internal/config"
)

type CookieManager struct {
	secure     bool
	sameSite   http.SameSite
	cookieName string
}

func NewCookieManager(cfg config.Config) *CookieManager {
	ss := http.SameSiteLaxMode
	switch cfg.CookieSameSite {
	case "strict":
		ss = http.SameSiteStrictMode
	case "none":
		if cfg.CookieSecure {
			ss = http.SameSiteNoneMode
		}
	}
	name := "bff_sid"
	if cfg.CookieSecure {
		name = "__Host-bff_sid"
	}
	return &CookieManager{secure: cfg.CookieSecure, sameSite: ss, cookieName: name}
}
func (m *CookieManager) Name() string { return m.cookieName }
func (m *CookieManager) Set(w http.ResponseWriter, sid string, ttl int64) {
	if sid == "" || ttl <= 0 {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: m.cookieName, Value: sid, Path: "/", HttpOnly: true, Secure: m.secure, SameSite: m.sameSite, Expires: time.Now().Add(time.Duration(ttl) * time.Second), MaxAge: int(ttl)})
}
func (m *CookieManager) ReadSessionID(r *http.Request) (string, bool) {
	ck, err := r.Cookie(m.cookieName)
	if err != nil || ck.Value == "" {
		return "", false
	}
	return ck.Value, true
}
func (m *CookieManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: m.cookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.secure, SameSite: m.sameSite, MaxAge: -1, Expires: time.Unix(0, 0)})
}
func (m *CookieManager) WriteSessionID(w http.ResponseWriter, sid string, ttl int64) {
	m.Set(w, sid, ttl)
}
