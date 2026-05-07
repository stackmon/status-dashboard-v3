package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api"
	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
	"github.com/stackmon/otc-status-dashboard/internal/cache"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

func initTestsWithCache(t *testing.T) (r *gin.Engine, componentsCache, eventsCache *cache.Cache[cache.CachedResponse]) {
	t.Helper()

	d, err := db.New(&conf.Config{DB: databaseURL})
	require.NoError(t, err)

	componentsCache = cache.NewHTTPCache(5 * time.Second)
	eventsCache = cache.NewHTTPCache(5 * time.Second)

	gin.SetMode(gin.TestMode)
	r = gin.New()
	r.NoRoute(apiErrors.Return404)
	r.Use(api.ErrorHandle())

	logger, _ := zap.NewDevelopment()

	v1API := r.Group("v1")
	{
		v1API.GET("component_status", cache.GinMiddleware(componentsCache), v1.GetComponentsStatusHandler(d, logger))
		v1API.POST("component_status", cache.Invalidator(componentsCache), v1.PostComponentStatusHandler(d, logger))
		v1API.GET("incidents", cache.GinMiddleware(eventsCache), v1.GetIncidentsHandler(d, logger))
	}

	v2API := r.Group("v2")
	{
		v2API.GET("components", cache.GinMiddleware(componentsCache), v2.GetComponentsHandler(d, logger))
		v2API.GET("incidents", cache.GinMiddleware(eventsCache), v2.GetIncidentsHandler(d, logger))
		v2API.POST("incidents", api.ValidateComponentsMW(d, logger), cache.Invalidator(eventsCache), v2.PostIncidentHandler(d, logger))
	}

	return r, componentsCache, eventsCache
}

func TestCacheGETHitOnSecondRequest(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{"v2 components", "/v2/components"},
		{"v2 incidents", "/v2/incidents"},
		{"v1 component_status", "/v1/component_status"},
		{"v1 incidents", "/v1/incidents"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, cCache, eCache := initTestsWithCache(t)
			defer cCache.Close()
			defer eCache.Close()

			w1 := httptest.NewRecorder()
			req1, _ := http.NewRequest(http.MethodGet, tc.endpoint, nil)
			r.ServeHTTP(w1, req1)
			require.Equal(t, http.StatusOK, w1.Code)
			assert.Empty(t, w1.Header().Get("X-Cache"), "first GET must be a cache MISS")

			w2 := httptest.NewRecorder()
			req2, _ := http.NewRequest(http.MethodGet, tc.endpoint, nil)
			r.ServeHTTP(w2, req2)
			require.Equal(t, http.StatusOK, w2.Code)
			assert.Equal(t, "HIT", w2.Header().Get("X-Cache"), "second GET must be a cache HIT")
			assert.Equal(t, w1.Body.String(), w2.Body.String(), "cached response body must match original")
		})
	}
}

func TestCacheInvalidation(t *testing.T) {
	tests := []struct {
		name           string
		primeEndpoint  string
		mutateMethod   string
		mutateEndpoint string
		mutateBody     string
		checkEndpoint  string
		expectedCache  string
	}{
		{
			name:           "successful POST invalidates associated cache",
			primeEndpoint:  "/v2/incidents",
			mutateMethod:   http.MethodPost,
			mutateEndpoint: "/v2/incidents",
			mutateBody:     `{"title": "test", "impact": 1, "components": [1], "start_date": "2026-01-01T00:00:00Z", "system": false, "type": "incident"}`,
			checkEndpoint:  "/v2/incidents",
			expectedCache:  "",
		},
		{
			name:           "POST to components invalidates components cache",
			primeEndpoint:  "/v2/components",
			mutateMethod:   http.MethodPost,
			mutateEndpoint: "/v1/component_status",
			mutateBody:     `{"status": "degraded", "component_id": 1}`,
			checkEndpoint:  "/v2/components",
			expectedCache:  "",
		},
		{
			name:           "POST to incidents does NOT invalidate components cache",
			primeEndpoint:  "/v2/components",
			mutateMethod:   http.MethodPost,
			mutateEndpoint: "/v2/incidents",
			mutateBody:     `{"title": "test", "impact": 1, "components": [1], "start_date": "2026-01-01T00:00:00Z", "system": false, "type": "incident"}`,
			checkEndpoint:  "/v2/components",
			expectedCache:  "HIT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, cCache, eCache := initTestsWithCache(t)
			defer cCache.Close()
			defer eCache.Close()

			prime := httptest.NewRecorder()
			r.ServeHTTP(prime, makeGET(t, tc.primeEndpoint))
			require.Equal(t, http.StatusOK, prime.Code)

			warm := httptest.NewRecorder()
			r.ServeHTTP(warm, makeGET(t, tc.primeEndpoint))
			assert.Equal(t, "HIT", warm.Header().Get("X-Cache"))

			postReq, _ := http.NewRequest(tc.mutateMethod, tc.mutateEndpoint, bytes.NewReader([]byte(tc.mutateBody)))
			postReq.Header.Set("Content-Type", "application/json")
			postW := httptest.NewRecorder()
			r.ServeHTTP(postW, postReq)
			require.Less(t, postW.Code, 400, "mutation request must succeed")

			afterMut := httptest.NewRecorder()
			r.ServeHTTP(afterMut, makeGET(t, tc.checkEndpoint))
			assert.Equal(t, tc.expectedCache, afterMut.Header().Get("X-Cache"))
		})
	}
}

func makeGET(t *testing.T, path string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	require.NoError(t, err)
	return req
}
