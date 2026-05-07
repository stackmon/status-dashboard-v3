package cache

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type CachedResponse struct {
	status int
	header http.Header
	body   []byte
}

// GinMiddleware returns a gin middleware that caches successful GET responses.
func GinMiddleware(c *Cache[CachedResponse]) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Method != http.MethodGet {
			ctx.Next()
			return
		}

		key := ctx.Request.RequestURI

		if cached, ok := c.Get(key); ok {
			for k, vals := range cached.header {
				ctx.Writer.Header()[k] = append([]string(nil), vals...)
			}
			ctx.Writer.Header().Set("X-Cache", "HIT")
			ctx.Writer.WriteHeader(cached.status)
			_, _ = ctx.Writer.Write(cached.body)
			ctx.Abort()
			return
		}

		w := &responseRecorder{ResponseWriter: ctx.Writer, body: &bytes.Buffer{}}
		ctx.Writer = w

		ctx.Next()

		if w.Status() >= 200 && w.Status() < 300 {
			c.Set(key, CachedResponse{
				status: w.Status(),
				header: w.Header().Clone(),
				body:   append([]byte(nil), w.body.Bytes()...),
			})
		}
	}
}

type responseRecorder struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteString(s string) (int, error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

// Invalidator returns a middleware that invalidates the given cache on mutating requests.
func Invalidator(c *Cache[CachedResponse]) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if ctx.Request.Method != http.MethodGet && ctx.Writer.Status() < 400 {
			c.InvalidateAll()
		}
	}
}

const defaultMaxHTTPCacheItems = 1000

// NewHTTPCache creates a cache instance for HTTP responses with the given TTL.
func NewHTTPCache(ttl time.Duration) *Cache[CachedResponse] {
	return New[CachedResponse](ttl, defaultMaxHTTPCacheItems)
}
