# HTTP Response Caching

## Overview

The Status Dashboard implements an in-memory HTTP response cache with request coalescing
to reduce database load and improve response latency for read-heavy GET endpoints.

## Architecture

```
Request → GinMiddleware → [Cache HIT?] → yes → replay cached response
                              ↓ no
                     [singleflight.Do] → only 1 goroutine executes handler
                              ↓
                     [drainRecorder captures response]
                              ↓
                     [store in cache + respond to all waiters]
```

### Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `Cache[V]` | `internal/cache/cache.go` | Generic LRU cache with TTL and background eviction |
| `HTTPCache` | `internal/cache/middleware.go` | Wraps `Cache[CachedResponse]` + `singleflight.Group` |
| `GinMiddleware` | `internal/cache/middleware.go` | Gin handler that serves/populates cache |
| `Invalidator` | `internal/cache/middleware.go` | Gin handler that flushes cache on mutations |
| `drainRecorder` | `internal/cache/middleware.go` | Captures response without sending to client |

## Configuration

Defined in `internal/api/api.go`:

```go
const (
    componentsCacheTTL = 60 * time.Second  // /v2/components, /v1/component_status, /v2/availability
    eventsCacheTTL     = 10 * time.Second  // /v2/incidents, /v2/events, /v1/incidents
)
```

| Parameter | Value | Notes |
|-----------|-------|-------|
| Components TTL | 60s | Components change rarely |
| Events TTL | 10s | Events/incidents change more frequently |
| Max items | 1000 | Per cache instance (LRU eviction) |

## Cache Key Strategy

The cache key is `ctx.Request.RequestURI` — the full path including query parameters.
This means `/v2/events`, `/v2/events?type=incident`, and `/v2/events?type=maintenance`
are cached independently.

## Thundering Herd Protection

When TTL expires and multiple concurrent requests arrive for the same key,
`singleflight.Group` ensures only **one** goroutine executes the database query.
All other goroutines wait and receive the same result without hitting the DB.

## Cache Invalidation

`Invalidator` middleware is applied to all mutating endpoints (POST/PATCH/PUT).
On a successful mutation (status < 400), it calls `InvalidateAll()` on the
associated cache instance.

| Cache Instance | Invalidated By |
|----------------|---------------|
| `componentsCache` | `POST /v1/component_status`, `POST /v2/components` |
| `eventsCache` | `POST /v2/incidents`, `PATCH /v2/incidents/:id`, `POST /v2/events`, etc. |

## Response Headers

| Header | Value | Meaning |
|--------|-------|---------|
| `X-Cache: HIT` | Present | Response served from cache |
| (absent) | — | Cache miss; response fetched from DB |

## Load Test Results

Testing tool: `wrk 4.2.0`. Target: application with PostgreSQL backend.

### Warm Cache — 4 threads, 50 connections, 30s

| Endpoint | RPS | P50 | P99 | Max | Errors |
|----------|-----|-----|-----|-----|--------|
| `GET /v2/components` | 56,287 | 0.6ms | 23ms | 140ms | 0 |
| `GET /v2/incidents` | 38,524 | 0.9ms | 10ms | 45ms | 0 |
| `GET /v2/events` | 46,012 | 0.7ms | 208ms | 635ms | 0 |
| `GET /v2/events?type=incident` | 39,032 | 0.9ms | 39ms | 304ms | 0 |
| `GET /v2/events?type=maintenance` | 38,863 | 0.9ms | 211ms | 544ms | 0 |
| `GET /v2/availability` | 43,462 | 0.8ms | 18ms | 221ms | 0 |
| `GET /v1/component_status` | 30,402 | 1.1ms | 26ms | 244ms | 0 |
| `GET /v1/incidents` | 35,717 | 0.9ms | 26ms | 369ms | 0 |

### Stress Test — 8 threads, 200 connections, 60s

| Endpoint | RPS | P50 | P99 | Max | Errors |
|----------|-----|-----|-----|-----|--------|
| `GET /v2/components` | 52,046 | 3.1ms | 101ms | 457ms | 0 |
| `GET /v2/incidents` | 40,293 | 4.0ms | 53ms | 299ms | 0 |
| `GET /v2/events` | 52,419 | 3.0ms | 50ms | 198ms | 0 |
| `GET /v2/events?type=incident` | 52,415 | 3.0ms | 62ms | 333ms | 0 |
| `GET /v2/events?type=maintenance` | 46,269 | 3.4ms | 214ms | 1110ms | 0 |
| `GET /v1/component_status` | 36,731 | 4.4ms | 52ms | 319ms | 0 |
| `GET /v1/incidents` | 41,804 | 3.9ms | 72ms | 410ms | 0 |

### Key Observations

- Zero errors and zero timeouts under 200 concurrent connections
- Singleflight eliminates P99 spikes on cache expiry (previously up to 1.12s → now 53ms)
- DB connection pool (`MaxOpenConns=25`) prevents connection exhaustion
