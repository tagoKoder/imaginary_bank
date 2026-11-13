// bff\internal\api\rest\middleware\oapi_validator.go
package middleware

import (
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

func OapiValidator(swagger *openapi3.T) func(http.Handler) http.Handler {
	return nethttpmiddleware.OapiRequestValidator(swagger)
}
