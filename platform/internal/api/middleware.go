package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/american-desi/platform/internal/logging"
	"github.com/google/uuid"
)

// requestIDMW assigns a UUID request id per request and stores it in context.
func requestIDMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", rid)
		ctx := logging.WithRequestID(r.Context(), rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMW writes one JSON log line per request.
func loggingMW(log *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &respWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)
			log.Info(r.Context(), "http", map[string]any{
				"method":   r.Method,
				"path":     r.URL.Path,
				"status":   rw.status,
				"bytes":    rw.bytes,
				"duration": time.Since(start).Milliseconds(),
				"remote":   clientIP(r),
			})
		})
	}
}

// recoverMW catches panics and converts them to 500s.
func recoverMW(log *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error(r.Context(), "panic", map[string]any{"err": rec})
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitMW enforces a simple in-memory token bucket per client IP.
// Good enough for a single-node deploy with Caddy in front.
type rateLimitMW struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rps     float64
	burst   int
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimitMW(rps float64) *rateLimitMW {
	if rps <= 0 {
		rps = 5
	}
	return &rateLimitMW{buckets: map[string]*bucket{}, rps: rps, burst: int(rps * 2)}
}

func (rl *rateLimitMW) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only throttle public endpoints; internal /admin is exempt.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r)
		rl.mu.Lock()
		b, ok := rl.buckets[ip]
		now := time.Now()
		if !ok {
			b = &bucket{tokens: float64(rl.burst), last: now}
			rl.buckets[ip] = b
		}
		elapsed := now.Sub(b.last).Seconds()
		b.tokens += elapsed * rl.rps
		if b.tokens > float64(rl.burst) {
			b.tokens = float64(rl.burst)
		}
		b.last = now
		if b.tokens < 1 {
			rl.mu.Unlock()
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
			return
		}
		b.tokens--
		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

type respWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *respWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *respWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if r.RemoteAddr == "" {
		return ""
	}
	// strip port
	if i := strings.LastIndex(r.RemoteAddr, ":"); i >= 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

// _ keep context alive
var _ context.Context = context.Background()
