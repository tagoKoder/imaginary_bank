// bff\internal\api\rest\middleware\trace.go
package middleware

import (
	"context"
	"net/http"

	"github.com/tagoKoder/bff/internal/util"
)

const (
	ctxTraceID    ctxKey = "trace_id"
	ctxSubject    ctxKey = "subject"
	ctxRoles      ctxKey = "roles"
	ctxCustomerID ctxKey = "customer_id"
)

func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := r.Header.Get("X-Trace-Id")
		if tid == "" {
			tid = util.NewTraceID()
		}
		w.Header().Set("X-Trace-Id", tid)
		ctx := context.WithValue(r.Context(), ctxTraceID, tid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTraceID(ctx context.Context) string {
	v, _ := ctx.Value(ctxTraceID).(string)
	return v
}

func SetAuth(ctx context.Context, subject string, roles []string, customerID string) context.Context {
	ctx = context.WithValue(ctx, ctxSubject, subject)
	ctx = context.WithValue(ctx, ctxRoles, roles)
	ctx = context.WithValue(ctx, ctxCustomerID, customerID)
	return ctx
}

func GetSubject(ctx context.Context) string {
	v, _ := ctx.Value(ctxSubject).(string)
	return v
}

func GetCustomerID(ctx context.Context) string {
	v, _ := ctx.Value(ctxCustomerID).(string)
	return v
}

func HasRole(ctx context.Context, role string) bool {
	roles, _ := ctx.Value(ctxRoles).([]string)
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
