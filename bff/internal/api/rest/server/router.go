// bff\internal\api\rest\server\router.go
package server

import (
	"log"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	openapi "github.com/tagoKoder/bff/internal/api/rest/gen/openapi"
	"github.com/tagoKoder/bff/internal/api/rest/httperr"
	"github.com/tagoKoder/bff/internal/api/rest/middleware"
)

type RouterDeps struct {
	Server   *Server
	HSTS     bool
	DebugErr bool
}

func NewRouter(deps RouterDeps, swagger *openapi3.T, swaggerValidator func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// Resolver OpenAPI para seguridad por operación
	oas, err := middleware.NewOpenAPISecurity(swagger)
	if err != nil {
		panic(err)
	}

	// 1) Context HTTP disponible desde el inicio
	r.Use(middleware.WithHTTPMiddleware)

	// 2) Correlation-ID estándar
	r.Use(middleware.CorrelationID)

	// 3) baseline/hardening
	r.Use(chimw.RealIP)
	r.Use(middleware.Recover())
	r.Use(middleware.RouteTemplate(oas))
	r.Use(middleware.AccessLog())
	r.Use(middleware.SecurityHeaders(deps.HSTS))

	// 4) Schema validation (si lo pasas)
	if swaggerValidator != nil {
		r.Use(swaggerValidator)
	}

	// 5) Rate limiting
	r.Use(middleware.RateLimit(deps.Server.deps.Config.RateLimitRPS))

	// CSRF must validate the original request cookies before automatic session refresh.
	r.Use(middleware.CSRF(deps.Server.deps.CSRF, oas))

	// AuthSession can rotate SID and CSRF cookies together when refreshing.
	r.Use(middleware.AuthSession(middleware.AuthDeps{
		Identity: deps.Server.deps.Clients.Identity,
		Cookies:  deps.Server.deps.Cookies,
		CSRF:     deps.Server.deps.CSRF,
	}, oas))

	// 8) OpenAPI STRICT handler
	strict := openapi.NewStrictHandlerWithOptions(
		deps.Server,
		nil,
		openapi.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				// Log interno con correlation-id (no lo devuelvas al cliente como “detalle”)
				cid := r.Header.Get("X-Correlation-Id")
				log.Printf("request_error cid=%s route=%s err=%v", cid, r.URL.Path, err)

				// Sanitizado: BAD_REQUEST (sin err.Error al cliente)
				httperr.Write(w, r, httperr.BadRequest().WithCause(err), deps.DebugErr)
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				cid := r.Header.Get("X-Correlation-Id")
				log.Printf("handler_error cid=%s route=%s err=%v", cid, r.URL.Path, err)

				// Sanitizado + mapeo (gRPC -> HTTP si aplica)
				httperr.Write(w, r, err, deps.DebugErr)
			},
		},
	)

	openapi.HandlerFromMux(strict, r)
	return r
}
