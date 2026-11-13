// bff\internal\api\rest\v1\web\feature\session\handler.go
package session

import (
	"encoding/json"
	"net/http"

	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/config"
	"github.com/tagoKoder/bff/internal/security"
)

type Handler struct {
	cfg     config.Config
	clients *ports.Clients
	cookies *security.CookieManager
	csrf    *security.CSRFManager
}

func New(cfg config.Config, clients *ports.Clients, cookies *security.CookieManager, csrf *security.CSRFManager) *Handler {
	return &Handler{cfg: cfg, clients: clients, cookies: cookies, csrf: csrf}
}

// POST /bff/session/logout (manual, recomendado)
// - Intenta invalidar sesión upstream (Identity) si hay session cookie
// - Limpia cookies (session + csrf)
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sid, ok := h.cookies.ReadSessionID(r)
	if ok && sid != "" {
		_, _ = h.clients.Identity.LogoutSession(r.Context(), ports.LogoutSessionInput{SessionID: sid})
	}

	h.cookies.Clear(w)
	h.csrf.Clear(w)

	w.WriteHeader(http.StatusNoContent)
}

// GET /bff/session/csrf (manual)
// - Devuelve token actual; si no existe, lo crea
func (h *Handler) CSRFToken(w http.ResponseWriter, r *http.Request) {
	tok, err := h.csrf.GetOrCreateToken(w, r)
	if err != nil || tok == "" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"csrf_token": tok,
	})
}
