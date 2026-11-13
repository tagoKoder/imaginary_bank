// bff/internal/api/rest/middleware/auth_session.go

package middleware

import (
	"context"
	"net/http"

	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/security"
	"github.com/tagoKoder/bff/internal/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	CtxSubject   ctxKey = "subject"
	CtxRoles     ctxKey = "roles"
	CtxSession   ctxKey = "session_id"
	CtxCustomer  ctxKey = "customer_id"
	CtxUserState ctxKey = "user_status"
	CtxIdentity  ctxKey = "identity_id"
)

type AuthDeps struct {
	Identity ports.IdentityPort
	Cookies  *security.CookieManager
	CSRF     *security.CSRFManager
}

func AuthSession(deps AuthDeps, oas *OpenAPISecurity) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ri, ok := oas.Find(r)
			if !ok || !RequiresScheme(ri.Security, "cookieAuth") {
				next.ServeHTTP(w, r)
				return
			}
			sid, ok := deps.Cookies.ReadSessionID(r)
			if !ok || sid == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			ctx := r.Context()
			ctx = security.WithSessionID(ctx, sid)
			ctx = security.WithClientIP(ctx, util.GetClientIP(r))
			ctx = security.WithUserAgent(ctx, r.UserAgent())
			ctx = security.WithRouteTemplate(ctx, ri.RouteTemplate)
			if ik := r.Header.Get("Idempotency-Key"); ik != "" {
				ctx = security.WithIdempotencyKey(ctx, ik)
			}
			// Refresh and logout are the authoritative session lifecycle operations.
			// Pre-refreshing here would rotate twice, or revoke the wrong SID.
			if ri.OperationID == "refreshWebSession" || ri.OperationID == "logoutWebSession" {
				ctx = context.WithValue(ctx, CtxSession, sid)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			ip, ua := util.GetClientIP(r), r.UserAgent()
			info, finalSID, token, err := getSessionInfoWithRefresh(ctx, deps, sid, ip, ua, w)
			if err != nil {
				return
			} // status already written
			if info.UserStatus != "" && info.UserStatus != "ACTIVE" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if info.SubjectIDOidc == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			ctx = security.WithSessionID(ctx, finalSID)
			ctx = security.WithAccessToken(ctx, token)
			ctx = context.WithValue(ctx, CtxSubject, info.SubjectIDOidc)
			ctx = context.WithValue(ctx, CtxRoles, info.User.Roles)
			ctx = context.WithValue(ctx, CtxSession, finalSID)
			ctx = context.WithValue(ctx, CtxCustomer, info.CustomerID)
			ctx = context.WithValue(ctx, CtxUserState, info.UserStatus)
			ctx = context.WithValue(ctx, CtxIdentity, info.IdentityID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func getSessionInfoWithRefresh(ctx context.Context, deps AuthDeps, sid, ip, ua string, w http.ResponseWriter) (ports.GetSessionInfoOutput, string, string, error) {
	info, err := deps.Identity.GetSessionInfo(ctx, ports.GetSessionInfoInput{SessionID: sid, IP: ip, UserAgent: ua})
	if err == nil && info.AccessToken != "" && info.AccessTokenExpiresIn > 30 {
		return info, sid, info.AccessToken, nil
	}
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			w.WriteHeader(http.StatusBadGateway)
			return ports.GetSessionInfoOutput{}, "", "", err
		}
		if st.Code() != codes.Unauthenticated || st.Message() != "SESSION_EXPIRED" {
			switch st.Code() {
			case codes.Unauthenticated, codes.NotFound, codes.FailedPrecondition:
				deps.Cookies.Clear(w)
				deps.CSRF.Clear(w)
				w.WriteHeader(http.StatusUnauthorized)
			case codes.PermissionDenied:
				w.WriteHeader(http.StatusForbidden)
			case codes.Unavailable, codes.DeadlineExceeded:
				w.WriteHeader(http.StatusServiceUnavailable)
			default:
				w.WriteHeader(http.StatusBadGateway)
			}
			return ports.GetSessionInfoOutput{}, "", "", err
		}
	}
	// Refresh only when Identity indicates an expired session, or a token is near expiry.
	ref, rerr := deps.Identity.RefreshSession(ctx, ports.RefreshSessionInput{SessionID: sid, IP: ip, UserAgent: ua})
	if rerr != nil {
		st, ok := status.FromError(rerr)
		if ok && (st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded) {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			deps.Cookies.Clear(w)
			deps.CSRF.Clear(w)
			w.WriteHeader(http.StatusUnauthorized)
		}
		return ports.GetSessionInfoOutput{}, "", "", rerr
	}
	if ref.SessionID == "" || ref.AccessToken == "" || ref.SessionExpiresIn <= 0 {
		w.WriteHeader(http.StatusBadGateway)
		return ports.GetSessionInfoOutput{}, "", "", status.Error(codes.Internal, "invalid refresh response")
	}
	info2, e2 := deps.Identity.GetSessionInfo(ctx, ports.GetSessionInfoInput{SessionID: ref.SessionID, IP: ip, UserAgent: ua})
	if e2 != nil {
		w.WriteHeader(http.StatusBadGateway)
		return ports.GetSessionInfoOutput{}, "", "", e2
	}
	if info2.SubjectIDOidc == "" {
		w.WriteHeader(http.StatusBadGateway)
		return ports.GetSessionInfoOutput{}, "", "", status.Error(codes.Internal, "invalid session info")
	}
	// Both cookies must be emitted before next handler starts its response.
	if _, e := deps.CSRF.IssueForSession(w, ref.SessionID); e != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return ports.GetSessionInfoOutput{}, "", "", e
	}
	deps.Cookies.Set(w, ref.SessionID, ref.SessionExpiresIn)
	w.Header().Set("Cache-Control", "no-store")
	return info2, ref.SessionID, ref.AccessToken, nil
}
