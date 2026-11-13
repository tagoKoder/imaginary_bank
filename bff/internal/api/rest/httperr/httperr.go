package httperr

import (
	"encoding/json"
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type AppError struct {
	HTTPStatus int
	PublicCode string
	PublicMsg  string
	PublicDet  map[string]any
	Cause      error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.PublicCode + ": " + e.Cause.Error()
	}
	return e.PublicCode
}
func (e *AppError) Unwrap() error { return e.Cause }
func New(code int, slug, msg string) *AppError {
	return &AppError{HTTPStatus: code, PublicCode: slug, PublicMsg: msg}
}
func (e *AppError) WithCause(err error) *AppError          { e.Cause = err; return e }
func (e *AppError) WithDetails(d map[string]any) *AppError { e.PublicDet = d; return e }
func BadRequest() *AppError                                { return New(400, "BAD_REQUEST", "Invalid request") }
func Unauthorized() *AppError                              { return New(401, "UNAUTHORIZED", "Unauthorized") }
func Forbidden() *AppError                                 { return New(403, "FORBIDDEN", "Forbidden") }
func NotFound() *AppError                                  { return New(404, "NOT_FOUND", "Not found") }
func Conflict() *AppError                                  { return New(409, "CONFLICT", "Conflict") }
func Upstream() *AppError                                  { return New(502, "UPSTREAM_ERROR", "Upstream service error") }
func Internal() *AppError                                  { return New(500, "INTERNAL", "Internal server error") }
func MapGrpc(err error) *AppError {
	st, ok := status.FromError(err)
	if !ok {
		return Internal().WithCause(err)
	}
	switch st.Code() {
	case codes.InvalidArgument, codes.OutOfRange:
		return BadRequest().WithCause(err)
	case codes.FailedPrecondition, codes.AlreadyExists, codes.Aborted:
		return Conflict().WithCause(err)
	case codes.Unauthenticated:
		return Unauthorized().WithCause(err)
	case codes.PermissionDenied:
		return Forbidden().WithCause(err)
	case codes.NotFound:
		return NotFound().WithCause(err)
	case codes.ResourceExhausted:
		return New(429, "RATE_LIMITED", "Too many requests").WithCause(err)
	case codes.Unavailable:
		return New(503, "SERVICE_UNAVAILABLE", "Service temporarily unavailable").WithCause(err)
	case codes.DeadlineExceeded:
		return New(504, "UPSTREAM_TIMEOUT", "Upstream deadline exceeded").WithCause(err)
	default:
		return Internal().WithCause(err)
	}
}
func Normalize(err error) *AppError {
	if err == nil {
		return nil
	}
	var app *AppError
	if errors.As(err, &app) {
		return app
	}
	if _, ok := status.FromError(err); ok {
		return MapGrpc(err)
	}
	return Internal().WithCause(err)
}
func Write(w http.ResponseWriter, r *http.Request, err error, debug bool) {
	app := Normalize(err)
	if app == nil {
		return
	}
	resp := ErrorResponse{Code: app.PublicCode, Message: app.PublicMsg}
	if debug && app.PublicDet != nil {
		resp.Details = app.PublicDet
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(app.HTTPStatus)
	_ = json.NewEncoder(w).Encode(resp)
}
