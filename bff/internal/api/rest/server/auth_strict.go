// bff\internal\api\rest\server\auth_strict.go
package server

import (
	"context"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
	"github.com/tagoKoder/bff/internal/api/rest/middleware"
	"github.com/tagoKoder/bff/internal/client/ports"
	"github.com/tagoKoder/bff/internal/security"
	"github.com/tagoKoder/bff/internal/util"
)

// GET /bff/login/start?redirect=/home
func (s *Server) StartWebLogin(
	ctx context.Context,
	req openapi.StartWebLoginRequestObject,
) (openapi.StartWebLoginResponseObject, error) {

	redir := ""
	if req.Params.Redirect != nil {
		redir = *req.Params.Redirect
	}
	redir = s.deps.Redirect.Safe(redir)

	out, err := s.deps.Clients.Identity.StartOidcLogin(ctx, ports.StartOidcLoginInput{
		Channel:            "web",
		RedirectAfterLogin: redir,
	})
	if err != nil {
		return openapi.StartWebLogin502JSONResponse(openapi.ErrorResponse{
			Code:    "UPSTREAM_ERROR",
			Message: "identity service unavailable",
		}), nil
	}

	// 302 Location: out.AuthorizationURL
	return openapi.StartWebLogin302Response{
		Headers: openapi.StartWebLogin302ResponseHeaders{
			Location: out.AuthorizationURL,
		},
	}, nil
}

// GET /bff/login/callback?code=...&state=...
func (s *Server) CompleteWebLogin(
	ctx context.Context,
	req openapi.CompleteWebLoginRequestObject,
) (openapi.CompleteWebLoginResponseObject, error) {

	// Tomamos request/response del contexto (inyectado por strict middleware)
	r, ok := middleware.GetHTTPRequest(ctx)
	if !ok || r == nil {
		return openapi.CompleteWebLogin502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http request context",
		}), nil
	}
	w, ok := middleware.GetHTTPResponseWriter(ctx)
	if !ok || w == nil {
		return openapi.CompleteWebLogin502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http response context",
		}), nil
	}

	code := req.Params.Code
	state := req.Params.State
	if code == "" || state == "" {
		return openapi.CompleteWebLogin400JSONResponse(openapi.ErrorResponse{
			Code:    "BAD_REQUEST",
			Message: "missing code/state",
		}), nil
	}

	out, err := s.deps.Clients.Identity.CompleteOidcLogin(ctx, ports.CompleteOidcLoginInput{
		Code:      code,
		State:     state,
		IP:        util.GetClientIP(r),
		UserAgent: r.UserAgent(),
		Channel:   "web",
	})
	if err != nil {
		return openapi.CompleteWebLogin401JSONResponse(openapi.ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "login rejected",
		}), nil
	}

	// Cookie de sesión + CSRF token
	if _, csrfErr := s.deps.CSRF.IssueForSession(w, out.SessionID); csrfErr != nil {
		return openapi.CompleteWebLogin502JSONResponse(openapi.ErrorResponse{Code: "INTERNAL", Message: "cannot issue csrf token"}), nil
	}
	s.deps.Cookies.Set(w, out.SessionID, out.SessionExpiresIn)

	// Evita cache
	w.Header().Set("Cache-Control", "no-store")

	return openapi.CompleteWebLogin302Response{
		Headers: openapi.CompleteWebLogin302ResponseHeaders{
			Location: s.deps.Redirect.Safe(out.RedirectAfterLogin),
		},
	}, nil
}

// POST /bff/session/logout
func (s *Server) LogoutWebSession(
	ctx context.Context,
	_ openapi.LogoutWebSessionRequestObject,
) (openapi.LogoutWebSessionResponseObject, error) {
	corrId := security.CorrelationID(ctx)

	r, ok := middleware.GetHTTPRequest(ctx)
	if !ok || r == nil {
		return openapi.LogoutWebSession502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http request context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}
	w, ok := middleware.GetHTTPResponseWriter(ctx)
	if !ok || w == nil {
		return openapi.LogoutWebSession502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http response context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	sid, ok := ctx.Value(middleware.CtxSession).(string)
	if ok && sid != "" {
		out, err := s.deps.Clients.Identity.LogoutSession(ctx, ports.LogoutSessionInput{SessionID: sid})
		if err != nil || !out.Success {
			s.deps.Cookies.Clear(w)
			s.deps.CSRF.Clear(w)
			return openapi.LogoutWebSession502JSONResponse(openapi.ErrorResponse{Code: "UPSTREAM_ERROR", Message: "session revocation not confirmed"}), nil
		}
	}

	s.deps.Cookies.Clear(w)
	s.deps.CSRF.Clear(w)

	return openapi.LogoutWebSession204Response{
		Headers: openapi.LogoutWebSession204ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}

// GET /api/v1/session/csrf
func (s *Server) GetWebCsrfToken(
	ctx context.Context,
	_ openapi.GetWebCsrfTokenRequestObject,
) (openapi.GetWebCsrfTokenResponseObject, error) {
	corrId := security.CorrelationID(ctx)

	r, ok := middleware.GetHTTPRequest(ctx)
	if !ok || r == nil {
		return openapi.GetWebCsrfToken403JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http request context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}
	w, ok := middleware.GetHTTPResponseWriter(ctx)
	if !ok || w == nil {
		return openapi.GetWebCsrfToken403JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http response context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	w.Header().Set("Cache-Control", "no-store")
	// 1) generar o recuperar el token CSRF de la respuesta
	sessionID, _ := ctx.Value(middleware.CtxSession).(string)
	tok, err := s.deps.CSRF.GetOrCreateTokenForSession(w, r, sessionID)
	if err != nil {
		return openapi.GetWebCsrfToken502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "cannot issue csrf token",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	if tok == "" {
		return openapi.GetWebCsrfToken502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "csrf token missing after issuance",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	return openapi.GetWebCsrfToken200JSONResponse{
		Body: openapi.CsrfTokenResponse{
			CsrfToken: tok,
		},
		Headers: openapi.GetWebCsrfToken200ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}

// POST /bff/session/refresh
func (s *Server) RefreshWebSession(
	ctx context.Context,
	req openapi.RefreshWebSessionRequestObject,
) (openapi.RefreshWebSessionResponseObject, error) {
	_ = req.Params.XCSRFToken // validado por middleware CSRF/OpenAPI
	corrId := security.CorrelationID(ctx)
	r, ok := middleware.GetHTTPRequest(ctx)
	if !ok || r == nil {
		return openapi.RefreshWebSession502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http request context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}
	w, ok := middleware.GetHTTPResponseWriter(ctx)
	if !ok || w == nil {
		return openapi.RefreshWebSession502JSONResponse(openapi.ErrorResponse{
			Code:    "INTERNAL",
			Message: "missing http response context",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	sid, ok := ctx.Value(middleware.CtxSession).(string)
	if !ok || sid == "" {
		return openapi.RefreshWebSession401JSONResponse(openapi.ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "missing session",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	out, err := s.deps.Clients.Identity.RefreshSession(ctx, ports.RefreshSessionInput{
		SessionID: sid,
		IP:        util.GetClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// higiene: si refresh falla, limpia para forzar login
		s.deps.Cookies.Clear(w)
		s.deps.CSRF.Clear(w)

		return openapi.RefreshWebSession401JSONResponse(openapi.ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "session refresh rejected",
			Details: &map[string]interface{}{"correlation_id": corrId},
		}), nil
	}

	// rota cookie y CSRF vinculado a la nueva session
	if _, csrfErr := s.deps.CSRF.IssueForSession(w, out.SessionID); csrfErr != nil {
		return openapi.RefreshWebSession502JSONResponse(openapi.ErrorResponse{Code: "INTERNAL", Message: "cannot issue csrf token"}), nil
	}
	s.deps.Cookies.Set(w, out.SessionID, out.SessionExpiresIn)
	w.Header().Set("Cache-Control", "no-store")

	return openapi.RefreshWebSession200JSONResponse{
		Body: openapi.RefreshSessionResponse{
			SessionExpiresIn: out.SessionExpiresIn,
		},
		Headers: openapi.RefreshWebSession200ResponseHeaders{
			XCorrelationId: corrId,
		},
	}, nil
}
