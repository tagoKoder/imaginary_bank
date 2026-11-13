// bff/internal/api/rest/middleware/csrf.go
package middleware

import (
	"net/http"

	"github.com/tagoKoder/bff/internal/security"
)

// CSRF is enforced for state-changing operations requiring cookieAuth + csrfAuth.
// Token minting happens only in callback/refresh/get-csrf, never for arbitrary GETs.
func CSRF(m *security.CSRFManager, oas *OpenAPISecurity) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ri, ok := oas.Find(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			unsafe := r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete
			if unsafe && RequiresBothInSameRequirement(ri.Security, "cookieAuth", "csrfAuth") && !m.Validate(r) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
