package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ---- Rate limiting (audit finding #3) ----
//
// In-memory fixed-window limiter: adequate for a single-instance control
// plane and better than nothing on day one. Redis-backed limiter replaces
// this when the API scales past one replica (tracked in PROGRESS.md).

type window struct {
	count int
	start time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*window
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{visitors: map[string]*window{}}
}

func clientIP(r *http.Request) string {
	// Never trust X-Forwarded-For unless behind a trusted proxy; gin's
	// ClientIP is configured conservatively by default.
	return r.RemoteAddr
}

func (s *Server) rateLimit(max int, per time.Duration) gin.HandlerFunc {
	rl := s.limiter
	if rl == nil {
		rl = newRateLimiter()
	}
	return func(c *gin.Context) {
		key := clientIP(c.Request)
		now := time.Now()

		rl.mu.Lock()
		w, ok := rl.visitors[key]
		if !ok || now.Sub(w.start) > per {
			// Opportunistic GC so the map can't be grown unboundedly by
			// spoofed-source floods.
			if len(rl.visitors) > 10_000 {
				for k, v := range rl.visitors {
					if now.Sub(v.start) > per {
						delete(rl.visitors, k)
					}
				}
			}
			rl.visitors[key] = &window{count: 1, start: now}
			ok = false
		} else {
			w.count++
		}
		count := w.count
		rl.mu.Unlock()

		if ok && count > max {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// bodyLimit caps request bodies before binding (audit finding #4).
func (s *Server) bodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// ---- Refresh token rotation (audit finding #2) ----

type refreshRecord struct {
	userID   string
	consumed bool
}

type refreshStore struct {
	mu      sync.Mutex
	active  map[string]*refreshRecord // jti -> record
}

func newRefreshStore() *refreshStore {
	return &refreshStore{active: map[string]*refreshRecord{}}
}

func newJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (r *refreshStore) issue(jti, userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active[jti] = &refreshRecord{userID: userID}
}

// consume returns the record only if jti exists AND was never consumed.
// A replayed (already-consumed) refresh token revokes the whole family —
// standard stolen-token-response policy.
func (r *refreshStore) consume(jti string) (*refreshRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.active[jti]
	if !ok {
		return nil, false
	}
	if rec.consumed {
		// Replay detected: burn every outstanding token for this user.
		for _, other := range r.active {
			if other.userID == rec.userID {
				other.consumed = true
			}
		}
		return nil, false
	}
	rec.consumed = true
	cp := *rec
	return &cp, true
}

func (r *refreshStore) revokeAllFor(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.active {
		if rec.userID == userID {
			rec.consumed = true
		}
	}
}
