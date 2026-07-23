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

Files: `db/migrations/000007_notification.up.sql`, `db/migrations/000007_notification.down.sql`

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
- [ ] Add review-audience recipients: `SD_NOTIFICATIONS_SMOD_EMAIL` (fixed), `SD_NOTIFICATIONS_EMAILS_OPERATORS`, `SD_NOTIFICATIONS_EMAILS_ADMINS`.
- [ ] Validate: if enabled, SMTP host/port/from and at least one review address are required.
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
- [ ] Recipient rules by resulting status: review audience (SMOD fixed address + operator + admin lists) + `contact_email`, normalize + dedup.
- [ ] Build `dedup_key` = `change_id : kind : recipient`.
- [ ] Change summary builder (title, dates, old/new status).
- [ ] Email templates + renderer (subject, body, deep link).
- [ ] SMTP sender using `github.com/wneessen/go-mail` (direct OTC SMTP endpoint, timeout + TLS).
- [ ] Promote `google/uuid` to a direct dependency.
- [ ] Verify: unit tests for recipient rules, dedup key, summary, rendering.

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

- [ ] Ticker loop: recover leases → claim batch → commit → send.
- [ ] Send outside the DB transaction.
- [ ] Per-row `recover()` isolation.
- [ ] On result, update outbox status + last_error on the row.
- [ ] Backoff with cap; move to `failed` at max attempts.
- [ ] Bounded batch size and concurrency (lease > SMTP timeout + wait).
- [ ] Graceful shutdown: stop claiming, drain in-flight.
- [ ] Verify: unit tests for backoff, lease limit, panic isolation, no double-claim.

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

Files: worker metrics/logging, runbook

- [ ] Metrics: queue depth (pending/processing), stale leases, sent/failed, retries, latency.
- [ ] Structured logs with `outbox_id` and `incident_id`.
- [ ] Retention job (delete sent rows/logs in batches; keep failed longer).
- [ ] Feature-flag rollout: off in prod, enable in staging first.
- [ ] Runbook: how to re-drive `status='failed'` rows.
- [ ] Verify: failed deliveries are queryable and re-drivable.
