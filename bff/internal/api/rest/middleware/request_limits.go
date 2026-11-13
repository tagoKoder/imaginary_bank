// bff\internal\api\rest\middleware\request_limits.go
package middleware

import (
	"net/http"
	"time"
)

func MaxBodyBytes(max int64) func(http.Handler) http.Handler {
	if max <= 0 {
		max = 1 << 20 // 1MB default
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	if d <= 0 {
		d = 10 * time.Second
	}
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, "timeout")
	}
}

func NoStore() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			next.ServeHTTP(w, r)
		})
	}
}
