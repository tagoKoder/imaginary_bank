// bff\internal\api\rest\middleware\correlation.go
package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/tagoKoder/bff/internal/security"
)

const HeaderCorrelationID = "X-Correlation-Id"

func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid := r.Header.Get(HeaderCorrelationID)
		if cid == "" {
			cid = uuid.NewString()
		}

		w.Header().Set(HeaderCorrelationID, cid)

		ctx := security.WithCorrelationID(r.Context(), cid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
