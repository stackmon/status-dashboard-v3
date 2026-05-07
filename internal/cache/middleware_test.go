package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "operations",
			run: func(t *testing.T) {
				c := New[string](time.Minute, 100)
				defer c.Close()
				c.Set("key", "value")

				value, ok := c.Get("key")
				require.True(t, ok)
				assert.Equal(t, "value", value)

				c.Invalidate("key")
				_, ok = c.Get("key")
				assert.False(t, ok)

				c.Set("one", "1")
				c.Set("two", "2")
				c.InvalidateAll()
				assert.Empty(t, c.items)
				assert.Equal(t, 0, c.order.Len())

				httpCache := NewHTTPCache(time.Second)
				defer httpCache.Close()
				require.NotNil(t, httpCache)
				assert.Equal(t, time.Second, httpCache.ttl)
			},
		},
		{
			name: "get returns false on expired entry",
			run: func(t *testing.T) {
				c := New[string](time.Minute, 100)
				defer c.Close()

				c.mu.Lock()
				e := &entry[string]{
					key:       "expired",
					value:     "stale",
					expiresAt: time.Now().Add(-time.Second),
				}
				c.items["expired"] = c.order.PushBack(e)
				c.mu.Unlock()

				_, ok := c.Get("expired")
				require.False(t, ok)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestGinMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		method        string
		path          string
		setupRouter   func(r *gin.Engine, cached *Cache[CachedResponse])
		requests      int
		expectedCode  int
		expectedBody  string
		expectedCache []string
	}{
		{
			name:   "caches successful GET response",
			method: http.MethodGet,
			path:   "/cacheable",
			setupRouter: func(r *gin.Engine, cached *Cache[CachedResponse]) {
				reads := 0
				r.GET("/cacheable", GinMiddleware(cached), func(c *gin.Context) {
					reads++
					c.Header("X-Single", "value")
					c.Writer.Header().Add("X-Multi", "first")
					c.Writer.Header().Add("X-Multi", "second")
					c.String(http.StatusAccepted, fmt.Sprintf("payload:%d", reads))
				})
			},
			requests:      2,
			expectedCode:  http.StatusAccepted,
			expectedBody:  "payload:1",
			expectedCache: []string{"", "HIT"},
		},
		{
			name:   "skips non-GET requests",
			method: http.MethodPost,
			path:   "/resource",
			setupRouter: func(r *gin.Engine, cached *Cache[CachedResponse]) {
				posts := 0
				r.POST("/resource", GinMiddleware(cached), func(c *gin.Context) {
					posts++
					c.String(http.StatusCreated, fmt.Sprintf("created:%d", posts))
				})
			},
			requests:      2,
			expectedCode:  http.StatusCreated,
			expectedBody:  "created:2",
			expectedCache: []string{"", ""},
		},
		{
			name:   "does not cache non-successful responses",
			method: http.MethodGet,
			path:   "/error",
			setupRouter: func(r *gin.Engine, cached *Cache[CachedResponse]) {
				reads := 0
				r.GET("/error", GinMiddleware(cached), func(c *gin.Context) {
					reads++
					c.String(http.StatusInternalServerError, fmt.Sprintf("error:%d", reads))
				})
			},
			requests:      2,
			expectedCode:  http.StatusInternalServerError,
			expectedBody:  "error:2",
			expectedCache: []string{"", ""},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cached := NewHTTPCache(time.Minute)
			defer cached.Close()
			router := gin.New()
			tc.setupRouter(router, cached)

			var lastResp *httptest.ResponseRecorder
			for i := 0; i < tc.requests; i++ {
				lastResp = performRequest(t, router, tc.method, tc.path)
				expectedCacheHeader := tc.expectedCache[i]
				assert.Equal(t, expectedCacheHeader, lastResp.Header().Get("X-Cache"))
			}

			require.Equal(t, tc.expectedCode, lastResp.Code)
			assert.Equal(t, tc.expectedBody, lastResp.Body.String())
		})
	}
}

func TestInvalidator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		setupRouter func(r *gin.Engine, cached *Cache[CachedResponse])
		actions     func(t *testing.T, router *gin.Engine)
	}{
		{
			name: "ignores failed mutations and GET requests",
			setupRouter: func(r *gin.Engine, cached *Cache[CachedResponse]) {
				reads := 0
				r.GET("/resource", GinMiddleware(cached), Invalidator(cached), func(c *gin.Context) {
					reads++
					c.String(http.StatusOK, fmt.Sprintf("resource:%d", reads))
				})
				r.POST("/resource", Invalidator(cached), func(c *gin.Context) {
					c.String(http.StatusBadRequest, "bad request")
				})
			},
			actions: func(t *testing.T, router *gin.Engine) {
				resp := performRequest(t, router, http.MethodGet, "/resource")
				require.Equal(t, http.StatusOK, resp.Code)

				resp = performRequest(t, router, http.MethodGet, "/resource")
				assert.Equal(t, "HIT", resp.Header().Get("X-Cache"))

				performRequest(t, router, http.MethodPost, "/resource")

				resp = performRequest(t, router, http.MethodGet, "/resource")
				assert.Equal(t, "HIT", resp.Header().Get("X-Cache"))
			},
		},
		{
			name: "invalidates on successful mutation",
			setupRouter: func(r *gin.Engine, cached *Cache[CachedResponse]) {
				reads := 0
				r.GET("/resource", GinMiddleware(cached), func(c *gin.Context) {
					reads++
					c.String(http.StatusOK, fmt.Sprintf("resource:%d", reads))
				})
				r.POST("/resource", Invalidator(cached), func(c *gin.Context) {
					c.Status(http.StatusCreated)
				})
			},
			actions: func(t *testing.T, router *gin.Engine) {
				performRequest(t, router, http.MethodGet, "/resource")
				performRequest(t, router, http.MethodPost, "/resource")

				resp := performRequest(t, router, http.MethodGet, "/resource")
				assert.Empty(t, resp.Header().Get("X-Cache"))
				assert.Equal(t, "resource:2", resp.Body.String())
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cached := NewHTTPCache(time.Minute)
			defer cached.Close()
			router := gin.New()
			tc.setupRouter(router, cached)
			tc.actions(t, router)
		})
	}
}

func performRequest(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}
