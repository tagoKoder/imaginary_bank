// bff\internal\api\rest\middleware\ratelimit.go
package middleware

import (
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens int
	last   time.Time
}

func RateLimit(rps int) func(http.Handler) http.Handler {
	if rps < 1 {
		rps = 1
	}

	var mu sync.Mutex
	store := map[string]*bucket{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			b := store[ip]
			if b == nil {
				b = &bucket{tokens: rps, last: time.Now()}
				store[ip] = b
			}

			now := time.Now()
			elapsed := now.Sub(b.last).Seconds()
			if elapsed >= 1 {
				refill := int(elapsed) * rps
				b.tokens += refill
				if b.tokens > rps {
					b.tokens = rps
				}
				b.last = now
			}

			if b.tokens <= 0 {
				mu.Unlock()
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			b.tokens--
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}
