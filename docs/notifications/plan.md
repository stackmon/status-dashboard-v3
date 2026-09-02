# Implementation Plan: Maintenance Email Notifications

Feature docs: [final_scope_email.md](final_scope_email.md) · [architecture.md](architecture.md)

The plan has two parts:

1. **Stages** — the big picture, in order.
2. **Tasks** — concrete checklists for each stage.

---

# Part 1. Stages

| # | Stage | What it delivers | Depends on |
|---|-------|------------------|------------|
| 1 | Database | Outbox table | — |
| 2 | Configuration | SMTP + notification settings | 1 |
| 3 | Storage layer | Go code to read/write the tables | 2 |
| 4 | Notification core | Recipients, templates, email sending | 3 |
| 5 | Transaction-safe writes | Shared-transaction DB methods | 3 |
| 6 | API triggers | Emails on create/patch | 4, 5 |
| 7 | Checker triggers | Emails on automatic transitions | 4, 5 |
| 8 | Delivery worker | Background sending across pods | 4 |
| 9 | Wiring | Start worker, inject into app | 6, 7, 8 |
| 10 | Tests | Unit + integration coverage | 9 |
| 11 | Operations | Metrics, retention, rollout | 10 |

**Suggested order:** 1 → 2 → 3 → 4 → 5 → (6 and 7) → 8 → 9 → 10 → 11.

The feature stays inert until Stage 6 and is always gated by `SD_NOTIFICATIONS_ENABLED`,
so every stage leaves the system working.

---

# Part 2. Tasks

## Stage 1 — Database

Files: `db/migrations/000008_notification.up.sql`, `db/migrations/000008_notification.down.sql`

- [ ] Create table `notification_outbox` (see architecture §4).
- [ ] Add index `idx_outbox_dispatch` (pending rows).
- [ ] Add index `idx_outbox_stale_processing` (stale leases).
- [ ] Add unique index `idx_outbox_dedup`.
- [ ] Write the down migration (drop the table).
- [ ] Verify: `make migrate-up` then `make migrate-down` runs clean.

Note: no separate `notification_log` table — delivery outcome lives on the outbox row.

## Stage 2 — Configuration

Files: `internal/conf/conf.go`, `internal/conf/conf_test.go`

- [ ] Add `SD_NOTIFICATIONS_ENABLED`.
- [ ] Add SMTP settings: host, port, from, user, password, TLS.
- [ ] Add timing settings:
  - `SD_SMTP_TIMEOUT` (e.g., 30s — SMTP connection/send timeout).
  - `SD_NOTIFICATIONS_LEASE_TIMEOUT` (e.g., 60s — must be > SMTP_TIMEOUT).
  - `SD_NOTIFICATIONS_MAX_ATTEMPTS` (e.g., 5 — max retries).
  - `SD_NOTIFICATIONS_BACKOFF_INTERVAL` (e.g., 5m minimum, with exponential cap).
- [ ] Add review-audience recipients: `SD_NOTIFICATIONS_SMOD_EMAIL` (fixed), `SD_NOTIFICATIONS_EMAILS_OPERATORS`, `SD_NOTIFICATIONS_EMAILS_ADMINS`.
- [ ] Validate: if enabled, SMTP host/port/from, lease timeout > SMTP timeout, and at least one review address are required.
- [ ] Mask SMTP secrets in logs.
- [ ] Verify: `go test ./internal/conf/...` passes.

## Stage 3 — Storage layer

Files: `internal/db/notification.go`, `internal/db/models.go`

- [ ] Add GORM model for the outbox.
- [ ] Enqueue method that accepts a shared transaction.
- [ ] Claim method: `FOR UPDATE SKIP LOCKED`, set `processing`, `attempts++`.
- [ ] Mark sent / failed method (updates status + last_error on the row).
- [ ] Lease recovery method (respects max attempts).
- [ ] Verify: unit tests for enqueue, claim, recovery, dedup conflict.

## Stage 4 — Notification core

Files: `internal/notification/` (`notification.go`, `resolver.go`, `renderer.go`, `smtp.go`, `templates/`)

- [ ] Define the three event kinds as typed constants (`pending_review`, `reviewed`, `status_changed`).
- [ ] Map kind by resulting maintenance status:
  - `kind='pending_review'` if `status='pending_review'`.
  - `kind='reviewed'` if `status='reviewed'`.
  - `kind='status_changed'` for all other statuses (`planned`, `in_progress`, `completed`, `cancelled`).
- [ ] Recipient rules by resulting status: review audience (SMOD fixed address + operator + admin lists) + `contact_email`, normalize + dedup.
- [ ] Build `dedup_key` as string `change_id : kind : recipient` (stored in JSONB, used in unique constraint).
- [ ] Build and serialize `payload` snapshot (JSONB): title, dates, old/new status, actor, incident link. This is stored in DB so worker never re-reads the incident.
- [ ] Change summary builder (title, dates, old/new status).
- [ ] Email templates in `internal/notification/templates/` (subject.txt, body.html, etc.).
  - Use Go `text/template` with payload snapshot as context.
  - Load and parse templates on worker startup (fail fast if invalid).
- [ ] Email renderer (subject, body, deep link from payload).
- [ ] SMTP sender using `github.com/wneessen/go-mail` (direct OTC SMTP endpoint, timeout + TLS).
- [ ] Promote `google/uuid` to a direct dependency.
- [ ] Verify: unit tests for kind mapping, recipient rules, dedup key, summary, payload serialization, and rendering.

## Stage 5 — Transaction-safe writes

Files: `internal/db/db.go`

Why: the three write paths use different transaction models today
(`SaveIncident` = Create, `ModifyIncident` = own transaction, `ModifyEventUpdate` = single update).

- [ ] Add `ModifyIncidentTx(tx, inc)`.
- [ ] Add `SaveIncidentTx(tx, inc)`.
- [ ] Add `ModifyEventUpdateTx(tx, update)`.
- [ ] Keep existing methods as thin wrappers (no behavior change).
- [ ] Verify: `make test` stays green; new methods covered.

## Stage 6 — API triggers

Files: `internal/api/v2/v2.go`, `internal/api/api.go`, `internal/api/routes.go`

- [ ] Wrap each maintenance write + enqueue in one transaction.
- [ ] Generate a fresh `change_id` per successful change.
- [ ] Enqueue only for maintenance, only on success, only when enabled.
- [ ] Map kind by resulting status: `pending_review`, `reviewed`, `status_changed`.
- [ ] Verify: no outbox row on 400/403/409/500; integration test for recipients.

## Stage 7 — Checker triggers

Files: `internal/checker/maintenance.go`, `internal/checker/checker.go`

- [ ] Enqueue kind `status_changed`, actor `checker`.
- [ ] Publish inside the winning optimistic-lock transaction only.
- [ ] On version conflict, publish nothing.
- [ ] One notification per real transition (no duplicates from backfill).
- [ ] Verify: integration test for transition and conflict paths.

## Stage 8 — Delivery worker

Files: `internal/notification/worker.go`

- [ ] Implement two wake-up paths:
  - **(Hot path)** Publisher signals in-process worker via channel after DB commit — sends immediately.
  - **(Safety path)** Low-frequency ticker (e.g., every 5 min) scans for due rows (retries + stale locks).
- [ ] Recover stuck rows: if `status='processing'` and `locked_at` is older than lease timeout, recover to `pending` (or `failed` if max attempts exhausted).
- [ ] Claim batch: `SELECT ... FOR UPDATE SKIP LOCKED` where `status='pending'` and `next_attempt_at <= now()`. Mark as `processing`, set `locked_by`/`locked_at`, increment `attempts`.
- [ ] Commit claim transaction, **then** send (never hold DB lock during SMTP).
- [ ] Per-row `recover()` isolation: each send runs in panic guard so one failure does not crash worker.
- [ ] On result, update outbox:
  - Success → `status = 'sent'`, `updated_at = now()`.
  - Failure with retries left → `status = 'pending'`, `next_attempt_at = now() + backoff()`, `last_error = err`.
  - Failure, no retries left → `status = 'failed'`, `last_error = err`.
- [ ] Backoff strategy: exponential with cap (e.g., 5min → 10min → 20min → 40min → max 2h).
- [ ] Bounded batch size (e.g., 50 rows) and concurrency (lease_timeout > SMTP_timeout + send time).
- [ ] Graceful shutdown: stop claiming new rows, finish in-flight sends, wait for graceful deadline.
- [ ] Verify: unit tests for backoff math, lease recovery, panic isolation, `FOR UPDATE SKIP LOCKED` no double-claim, signal path and ticker path.

## Stage 9 — Wiring

Files: `cmd/main.go`, `internal/app/app.go`

- [ ] Build worker with the existing `*db.DB` (shared pool, no new pool).
- [ ] Inject publisher into API and checker.
- [ ] Add worker to graceful shutdown.
- [ ] Note pod connection budget (`max_open_conns_per_pod * max_pods`).
- [ ] Verify: app boots enabled and disabled; clean shutdown.

## Stage 10 — Tests

Files: `internal/notification/*_test.go`, `tests/notifications_test.go`

- [ ] Unit: recipient rules, summary, renderer, worker retry/lease.
- [ ] Integration: each kind → expected recipients (review audience + creator vs creator only).
- [ ] Integration: checker transition emits one row to the creator.
- [ ] Failure path: SMTP down → outbox row marked failed with error.
- [ ] Failure path: DB error → no orphan email task.
- [ ] Dedup: same change cannot enqueue the same recipient twice.
- [ ] Verify: `make test`, `make test-acc`, `make lint` all pass.

## Stage 11 — Operations

Files: `internal/notification/metrics.go`, `internal/db/notification_ops.go`,
`internal/api/v2/notifications.go`, `internal/notification/worker.go`

Observability reads directly from the outbox row (the single source of truth). Two interfaces
expose it, plus retention and re-drive.

- [x] Prometheus `/metrics` (enabled only when notifications are on, dedicated registry):
  - Queue-depth gauges (pulled from the DB on each scrape): `notification_outbox_pending`,
    `_processing`, `_failed`, `_stale_processing`, `_retry_backlog`, `_oldest_pending_age_seconds`.
  - Worker series: `notification_sent_total{kind}`, `notification_failed_total{kind}`,
    `notification_attempts_total`, `notification_stale_recovered_total`,
    `notification_delivery_duration_seconds` (histogram).
  - Note: no per-`incident_id` label (cardinality); `kind` only.
- [x] Admin ops API (`/v2/notifications/…`, admin role): `GET /stats`, `GET /failed`,
  `POST /redrive` (reset `failed` → `pending`, optionally by id, then wake the worker).
- [x] Structured logs with `outbox_id`, `incident_id`, `recipient`, `kind`, `attempts`, `error`.
- [x] Retention (worker safety sweep): delete `status='sent'` older than 30 days in batches
  (`idx_outbox_retention`). `status='failed'` kept indefinitely (unfinished work, re-drivable).
- [x] Feature-flag rollout: gated by `SD_NOTIFICATIONS_ENABLED` (off by default; enable staging first).
- [x] Re-drive: via the `POST /v2/notifications/redrive` endpoint (not a manual SQL runbook).
- [x] Verify: failed deliveries queryable via `/stats` + `/failed`; re-drive tested end-to-end;
  `make test`, `make test-acc`, `make lint` all pass.
