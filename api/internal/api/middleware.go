package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/evil-shown/snow-radar/api/internal/store"
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
//
// Session state is persisted through store.Store (jti rows), so rotation and
// revocation survive process restarts. Consume is atomic at the DB level;
// replay burns the whole family for that user.

type sessionManager struct {
	st store.Store
}

func newSessionManager(st store.Store) *sessionManager {
	return &sessionManager{st: st}
}

func (m *sessionManager) issue(jti, userID string) error {
	return m.st.SaveRefreshToken(jti, userID)
}

func (m *sessionManager) consume(jti string) (string, error) {
	return m.st.ConsumeRefreshToken(jti)
}

func (m *sessionManager) revokeAllFor(userID string) error {
	return m.st.RevokeAllRefreshTokens(userID)
}
