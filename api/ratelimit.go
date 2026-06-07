package api

import (
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ipRateLimiter is a small, dependency-free token-bucket rate limiter keyed by
// client IP. It is used to throttle the unauthenticated auth endpoints
// (login / register) against online brute-force attacks.
//
// Design notes:
//   - Per-IP token bucket with lazy refill (no background goroutine).
//   - Idle buckets are evicted opportunistically so a flood of distinct source
//     IPs (e.g. spoofed X-Forwarded-For) cannot grow the map without bound.
//   - This is a throttle, not an authenticator. Behind a reverse proxy the
//     effective key is whatever gin's ClientIP() resolves; operators who
//     terminate TLS at a proxy should configure trusted proxies so ClientIP()
//     reflects the real peer rather than a spoofable header.
type ipRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rlBucket
	rate    float64 // tokens added per second
	burst   float64 // maximum tokens (and initial fill)
	lastGC  time.Time
}

type rlBucket struct {
	tokens float64
	last   time.Time
}

// newIPRateLimiter creates a limiter that allows bursts up to `burst` requests
// and then refills at `ratePerSec` tokens/second per client IP.
func newIPRateLimiter(ratePerSec, burst float64) *ipRateLimiter {
	return &ipRateLimiter{
		buckets: make(map[string]*rlBucket),
		rate:    ratePerSec,
		burst:   burst,
	}
}

// allow reports whether a request from key is permitted at time now, consuming
// one token when it is.
func (l *ipRateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Opportunistic GC: drop buckets idle for >10 minutes. Bounds memory even
	// under a spoofed-IP flood without needing a background goroutine.
	if l.lastGC.IsZero() {
		l.lastGC = now
	}
	if now.Sub(l.lastGC) > time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.last) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &rlBucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	// Refill based on elapsed time, capped at burst.
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateLimitMiddleware throttles requests per client IP, returning 429 when the
// caller exceeds the configured rate.
func rateLimitMiddleware(l *ipRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c.ClientIP(), time.Now()) {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down and try again in a minute.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
