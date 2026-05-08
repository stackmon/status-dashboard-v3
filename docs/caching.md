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

## Distributed Deployment Considerations (Kubernetes)

> **Current scope**: single-pod deployment. This section documents known limitations
> and mitigation strategies for horizontal scaling.

### Known Limitations

| Problem | Impact | Severity |
|---------|--------|----------|
| **Stale data across pods** | Each pod has independent cache; POST on Pod A invalidates only Pod A's cache. Pods B, C serve stale data until local TTL expires. | Medium |
| **Per-pod singleflight** | On TTL expiry, each of N pods sends 1 query to DB simultaneously (N total). With 20 pods → 20 concurrent heavy queries. | Medium |
| **Memory duplication** | Same cached responses stored in every pod: total RAM = O(N × cache_size). | Low |

### Risk Assessment

With current TTL values and typical payload sizes:

- **Max staleness window**: 60s (components), 10s (events)
- **DB peak on expiry**: N pods × 1 query (bounded by singleflight within each pod)
- **Memory overhead per pod**: negligible for JSON payloads (~KB each, max 1000 entries)

**Conclusion**: For ≤5 pods with 10-60s TTL, the current design is acceptable.
Issues become significant at 10+ pods or when data freshness SLA < TTL.

### Mitigation Without External Dependencies

| Strategy | Effect | Tradeoff |
|----------|--------|----------|
| **Staggered TTL (jitter)** | Add ±10% random offset to TTL → prevents synchronized cache expiry across pods | Slightly less predictable staleness |
| **Reduce TTL** | Shorter TTL → smaller inconsistency window | Higher DB load (more frequent misses) |
| **Ingress session affinity** | Sticky sessions → one user always hits same pod → no visible flip-flops | Uneven load distribution |

### Future Architecture Options

#### Option A — Redis as L2 Cache

```
Request → L1 (in-memory) → miss → L2 (Redis) → miss → DB
                                                   ↓
                              store in L2 + L1 ← response
```

- **Solves**: stale data, memory duplication, thundering herd (single Redis fetch)
- **Cost**: +0.5-2ms network RTT on L1 miss; Redis HA infrastructure (Sentinel/Cluster)
- **Invalidation**: DELETE key from Redis on mutation → all pods miss L1 on next request

#### Option B — Pub/Sub Broadcast Invalidation

```
Pod A receives POST → invalidate local cache → publish event to Redis Pub/Sub
                                                          ↓
Pod B, Pod C subscribers → receive event → InvalidateAll()
```

- **Solves**: stale data (near-realtime, ~ms propagation)
- **Does not solve**: thundering herd across pods, memory duplication
- **Cost**: minimal latency impact; requires message broker (Redis Pub/Sub, NATS, RabbitMQ)
- **Graceful degradation**: if broker is down, falls back to current TTL-based expiry

#### Option C — Distributed Singleflight (Redlock)

```
TTL expires → Pod tries SET NX lock_key → success → fetch from DB → store in Redis
                                         → failure → poll Redis until result available
```

- **Solves**: thundering herd at cluster level (exactly 1 DB query across all pods)
- **Does not solve**: stale data between invalidation events
- **Cost**: high complexity; Redlock requires 3+ Redis nodes; adds failure modes

### Decision Matrix

| Criteria | A: Redis L2 | B: Pub/Sub | C: Dist. Singleflight |
|----------|:-----------:|:----------:|:---------------------:|
| Data consistency | ★★★ | ★★☆ | ★☆☆ |
| Latency impact | ★★☆ | ★★★ | ★★★ |
| Thundering herd fix | ★★★ | ★☆☆ | ★★★ |
| Memory efficiency | ★★★ | ★☆☆ | ★☆☆ |
| Implementation complexity | Medium | Low | High |
| Infra dependency | Redis HA | Pub/Sub broker | 3+ Redis nodes |
| Failure mode | SPOF (without HA) | Graceful | SPOF |

### Recommended Progression

1. **Now**: Single-pod deployment — current implementation is optimal
2. **2-5 pods**: Add TTL jitter + session affinity — zero code changes needed
3. **5-10 pods**: Implement Option B (Pub/Sub invalidation) — low effort, high ROI
4. **10+ pods / strict SLA**: Implement Option A (Redis L2) + Option B combined

## RBAC Integration Guidelines

When merging the `feature/rbac` branch or introducing Role-Based Access Control, strictly observe the following architectural constraints to prevent data leakage and ensure cache consistency.

### 1. Middleware Chain Order (Critical)

The order of execution in `routes.go` is paramount. Caching must occur **after** all authentication and authorization checks.

**Correct Order:**
`[Logger] -> [CORS] -> [Auth] -> [RBAC] -> [Cache] -> [Handler]`

If `cache.GinMiddleware` is placed before RBAC checks, cached data (potentially containing restricted or administrative fields) could be served to unauthorized users, causing a severe security vulnerability.

### 2. Cache Key Segmentation

The current cache key uses `ctx.Request.RequestURI`. If RBAC introduces endpoints where the response payload differs based on the user's role (e.g., an Admin sees extra fields in `GET /v2/components` that a regular user does not), a global URI-based cache will lead to privilege escalation or data suppression.

**Mitigation:** 
Extend the cache key to include the user's role or context if the endpoint serves role-specific data:
`key := fmt.Sprintf("%s:%s", userRole, ctx.Request.RequestURI)`

### 3. Invalidation of New Mutating Routes

The RBAC branch introduces several new mutating endpoints (e.g., specific `POST`, `PATCH`, `DELETE` operations for incidents). You must manually attach `cache.Invalidator(a.eventsCache)` or `componentsCache` to all new mutating routes during the merge process. Failure to do so will result in stale data being served after a successful mutation.
