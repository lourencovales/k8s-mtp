package middleware

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
)

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   int
	logger  *slog.Logger
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// TODO: background cleanup goroutine (sweeping full buckets)

func NewRateLimiter(cfg *config.Config, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{
		rate:    float64(cfg.RateRequestsPerMinute) / 60.0,
		burst:   cfg.RateBurst,
		logger:  logger,
		buckets: make(map[string]*bucket),
	}
}

func (rl *RateLimiter) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := UserFromContext(r.Context())
		if key == "" {
			key = r.RemoteAddr
		}

		// algo on display:
		// 1. lock the bucket
		// 2. check if we have the bucket by its key; if not, create it
		// 3. add tokens based on the rate and the time since last refill
		// 4. if the new amount is bigger than the burst amount, set it to burst
		// 5. set last refill to current time
		// 6. consume token
		// 7. check if amount of tokens is >= 0 (to avoid race conditions, do it before unlock)
		// 8. unlock the bucket
		rl.mu.Lock()
		userBucket, ok := rl.buckets[key]
		if !ok {
			userBucket = &bucket{tokens: float64(rl.burst), lastRefill: time.Now()}
			rl.buckets[key] = userBucket
		}
		now := time.Now()
		elapsed := now.Sub(userBucket.lastRefill).Seconds()
		userBucket.tokens += elapsed * rl.rate
		if userBucket.tokens > float64(rl.burst) {
			userBucket.tokens = float64(rl.burst)
		}
		userBucket.lastRefill = now
		userBucket.tokens -= 1
		allowed := userBucket.tokens >= 0
		rl.mu.Unlock()

		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", math.Ceil(1.0/rl.rate)))
			http.Error(w, "too many requests, no more tokens", http.StatusTooManyRequests)
		} else {
			next(w, r)
		}
	}
}
