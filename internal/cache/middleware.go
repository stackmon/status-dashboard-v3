package cache

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

// CachedResponse holds a captured HTTP response ready to be replayed.
type CachedResponse struct {
	status int
	header http.Header
	body   []byte
}

const defaultMaxHTTPCacheItems = 1000

// HTTPCache wraps Cache[CachedResponse] with request coalescing via singleflight
// to prevent the thundering herd problem on cache misses.
type HTTPCache struct {
	c   *Cache[CachedResponse]
	sfg singleflight.Group
}

// NewHTTPCache creates an HTTPCache with the given TTL.
func NewHTTPCache(ttl time.Duration) *HTTPCache {
	return &HTTPCache{c: New[CachedResponse](ttl, defaultMaxHTTPCacheItems)}
}

// Close stops the background janitor goroutine.
func (h *HTTPCache) Close() {
	h.c.Close()
}

// GinMiddleware returns a Gin middleware that caches successful GET responses.
// Concurrent requests for the same uncached key are coalesced: only one
// goroutine executes the downstream handler while others wait and receive the
// same result, eliminating thundering-herd bursts on cache expiry.
func GinMiddleware(h *HTTPCache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if h == nil {
			ctx.Next()
			return
		}
		if ctx.Request.Method != http.MethodGet {
			ctx.Next()
			return
		}

		key := ctx.Request.RequestURI

		if cached, ok := h.c.Get(key); ok {
			writeCached(ctx, cached, true)
			ctx.Abort()
			return
		}

		// Save original writer before singleflight so the leader can restore it.
		originalWriter := ctx.Writer
		isLeader := false

		val, _, _ := h.sfg.Do(key, func() (interface{}, error) {
			isLeader = true

			// Double-check: a previous singleflight call may have just populated the cache.
			if cached, ok := h.c.Get(key); ok {
				return cached, nil
			}

			// Use a drain recorder so the response is captured without being sent yet;
			// all coalesced goroutines (including the leader) will write it afterwards.
			rec := newDrainRecorder(ctx.Writer)
			ctx.Writer = rec
			defer func() { ctx.Writer = originalWriter }()
			ctx.Next()

			resp := CachedResponse{
				status: rec.status,
				header: rec.headers,
				body:   append([]byte(nil), rec.body.Bytes()...),
			}
			if rec.status >= 200 && rec.status < 300 {
				h.c.Set(key, resp)
			}
			return resp, nil
		})

		cached, ok := val.(CachedResponse)
		if !ok {
			return
		}
		// The leader's response was captured (not sent) inside singleflight.Do.
		// Write it now — same code path for leader and followers.
		// Followers are marked as HIT because they did not hit the backend.
		writeCached(ctx, cached, !isLeader)
		ctx.Abort()
	}
}

// Invalidator returns a middleware that invalidates the given cache on successful mutating requests.
func Invalidator(h *HTTPCache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if h == nil {
			ctx.Next()
			return
		}

		ctx.Next()

		if ctx.Request.Method != http.MethodGet && ctx.Writer.Status() < 400 {
			h.c.InvalidateAll()
		}
	}
}

// writeCached replays a captured response into ctx. isHit sets X-Cache: HIT.
func writeCached(ctx *gin.Context, cached CachedResponse, isHit bool) {
	for k, vals := range cached.header {
		ctx.Writer.Header()[k] = append([]string(nil), vals...)
	}
	if isHit {
		ctx.Writer.Header().Set("X-Cache", "HIT")
	}
	ctx.Writer.WriteHeader(cached.status)
	_, _ = ctx.Writer.Write(cached.body)
}

// drainRecorder captures a Gin response (status, headers, body) without
// forwarding writes to the underlying connection.
type drainRecorder struct {
	gin.ResponseWriter
	body    bytes.Buffer
	status  int
	headers http.Header
	written bool
}

func newDrainRecorder(w gin.ResponseWriter) *drainRecorder {
	return &drainRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
		// Clone current headers so that headers set by earlier middleware
		// (e.g. CORS) are visible to the handler and included in the capture.
		headers: w.Header().Clone(),
	}
}

func (r *drainRecorder) Header() http.Header { return r.headers }
func (r *drainRecorder) Status() int         { return r.status }
func (r *drainRecorder) Written() bool       { return r.written }
func (r *drainRecorder) Size() int           { return r.body.Len() }
func (r *drainRecorder) WriteHeaderNow()     {} // prevent premature flush
func (r *drainRecorder) Flush()              {} // prevent partial writes to conn

func (r *drainRecorder) WriteHeader(code int) {
	r.status = code
	r.written = true
}

func (r *drainRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.body.Write(b)
}

func (r *drainRecorder) WriteString(s string) (int, error) {
	r.written = true
	return r.body.WriteString(s)
}
