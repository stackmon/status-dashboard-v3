# Final Scope: Maintenance Email Notifications

## Purpose

This document defines the implementation scope for maintenance email notifications.

Scope is intentionally limited to **email notifications only** for maintenance events.

## In Scope

1. Notification channel: SMTP email only.
2. Event type: maintenance only.
3. Trigger sources:
- Maintenance creation (`POST /v2/events`, type `maintenance`).
- Maintenance field and status changes via API (`PATCH /v2/events/:eventID`).
- Maintenance update text changes (`PATCH /v2/events/:eventID/updates/:updateID`).
- Automatic checker transitions (`reviewed -> planned`, `planned -> in_progress`, `in_progress -> completed`).
4. Recipient routing based on RBAC role model and maintenance ownership/use case.
5. Reliable asynchronous delivery via transactional outbox.
6. Delivery logging and failure visibility.

## RBAC Basis

Role model and permissions are inherited from:
- `docs/auth/rbac.md`
- `docs/auth/permissions.md`

No hardcoding to a single IdP group is allowed. Routing must rely on resolved application roles (`admin`, `operator`, `creator`) and configured role email lists.

## Recipient Rules

### Rule A: Pending Review Creation Alert

When a maintenance is created in `pending_review`:

1. Notify review audience for roles that can act on review flow:
- `operator`
- `admin`
2. Notify maintenance squad email (`contact_email`) if provided.

### Rule B: Any Change to Created Maintenance

For any maintenance change (title, description, dates, status, or update text), notify maintenance
squad email (`contact_email`).

This covers user story: maintenance creator wants notification when changes are made for created maintenance.

### Rule C: Checker-Originated Changes

Automatic checker transitions are considered valid maintenance changes and must produce notifications through the same email flow.

## Event-to-Email Matrix

1. `maintenance_created_pending_review`
- Trigger: create maintenance and resulting status is `pending_review`.
- Recipients: operator/admin role lists + `contact_email`.

2. `maintenance_created_planned`
- Trigger: create maintenance and resulting status is `planned`.
- Recipients: `contact_email`.

3. `maintenance_changed`
- Trigger: successful PATCH that changes maintenance title, description, dates, or status.
- Recipients: `contact_email`.

4. `maintenance_update_text_changed`
- Trigger: successful PATCH on maintenance update text.
- Recipients: `contact_email`.

5. `maintenance_status_changed_by_checker`
- Trigger: checker persisted status transition for maintenance.
- Recipients: `contact_email`.

## Email Contract (Minimum)

Each email must include:

1. Maintenance title.
2. Event ID.
3. Current status.
4. Short change summary (what changed).
5. Actor:
- user id (`preferred_username`) for user-initiated updates, or
- `checker` for automatic transitions.
6. Timestamp of change in UTC.
7. Direct link to maintenance details page.

## Data and Delivery Model

### Transactional Outbox

1. Notification intent is written in DB in the same transaction as maintenance change persistence.
2. Background worker pulls pending outbox rows and sends email asynchronously.
3. Delivery outcome is written to notification log.

### Reliability Requirements

1. API response must not wait for SMTP send.
2. No notification intent loss after successful business transaction commit.
3. Concurrency-safe worker processing with best-effort duplicate prevention across restarts and retries.
4. Delivery semantics are at-least-once; a rare duplicate is possible if SMTP accepts an email
	immediately before the sending pod terminates.

## System Design Principles

This section defines the architecture plan for email notifications in this product.

1. Decoupled producer/consumer model:
- API/checker write notification intents.
- Background worker performs delivery.

2. Durable queue semantics:
- Outbox acts as durable queue in primary DB.
- Delivery state transitions are explicit (`pending`, `processing`, `sent`, `failed`).

3. At-least-once delivery with best-effort duplicate prevention:
- Worker may retry the same item.
- Atomic claiming prevents concurrent processing by different pods.
- A duplicate remains possible if SMTP accepts an email before the pod records success.

4. Backoff and bounded retries:
- Retry strategy should use progressive delay.
- Maximum attempts must be finite, then move to failed state with reason.

5. Recipient deduplication:
- Same email must not receive duplicate notifications for one event change.
- Dedup must be applied after role-based and event-based recipient aggregation.

6. Clear event classification:
- Notification type must be deterministic from domain action.
- Subject and summary are derived from a stable event taxonomy.

7. Operational observability first:
- Metrics for queue depth, success/failure, retries, and latency.
- Structured logs with outbox id and event id for incident correlation.

8. Failure isolation:
- Notification failures must not fail business API requests after commit.
- Worker failures are isolated and recoverable on restart.

9. Secure-by-default configuration:
- SMTP credentials are never logged in plain text.
- Invalid notification config fails fast at startup.

## Configuration

Required notification config groups:

1. Feature toggle:
- `SD_NOTIFICATIONS_ENABLED`

2. SMTP:
- `SD_SMTP_HOST`
- `SD_SMTP_PORT`
- `SD_SMTP_FROM`
- `SD_SMTP_USER`
- `SD_SMTP_PASSWORD`
- `SD_SMTP_TLS`

3. Role email recipients:
- `SD_NOTIFICATIONS_EMAILS_ADMINS`
- `SD_NOTIFICATIONS_EMAILS_OPERATORS`

4. Source event recipient:
- Maintenance `contact_email` field (already present in event payload/model).

## Security and Compliance Constraints

1. Do not expose SMTP secrets in logs.
2. Keep auditability: delivery log must contain status and failure reason.
3. Keep RBAC boundaries unchanged; notification routing must not grant new API permissions.

## Acceptance Criteria

1. Creating maintenance in `pending_review` sends email to operator/admin audiences and squad email.
2. Any maintenance field, status, or update text change sends email to squad email.
3. Checker-driven transitions send email to squad email.
4. Email body includes link and short change summary.
5. Failed sends are logged and visible for operations.
6. Notification handling is limited to SMTP email for maintenance events.

## Implementation Plan

Implementation is split into incremental phases so each step can be delivered and verified independently.

### Phase 1: Design Lock and Contracts

1. Freeze event taxonomy:
- `maintenance_created_pending_review`
- `maintenance_created_planned`
- `maintenance_changed`
- `maintenance_update_text_changed`
- `maintenance_status_changed_by_checker`
2. Freeze recipient resolution rules:
- role-based review audience (`admin`, `operator`) for pending review creation.
- `contact_email` for all maintenance changes.
3. Freeze email payload contract:
- event id, title, actor, timestamp, previous status, new status, change summary, deep link.
4. Add trace field contract for observability:
- outbox id, event id, delivery status, failure reason.

### Phase 2: Schema and Migration

1. Add outbox table for notification intents.
2. Add notification log table for delivery outcomes.
3. Add indexes for pending queue fetch and time-based retries.
4. Add migration down scripts.
5. Validate migration rollback safety on local DB.

### Phase 3: Configuration and Validation

1. Extend configuration with:
- `SD_NOTIFICATIONS_ENABLED`
- `SD_SMTP_HOST`, `SD_SMTP_PORT`, `SD_SMTP_FROM`, `SD_SMTP_USER`, `SD_SMTP_PASSWORD`, `SD_SMTP_TLS`
- `SD_NOTIFICATIONS_EMAILS_ADMINS`, `SD_NOTIFICATIONS_EMAILS_OPERATORS`
2. Add startup validation:
- when notifications enabled, SMTP config must be valid.
- at least one recipient path must be available for pending review flow.
3. Mask SMTP secrets in logs.

### Phase 4: Notification Module

1. Create internal notification package with:
- event payload model
- recipient resolver
- email renderer
- smtp sender
- outbox publisher and consumer abstractions
2. Implement deterministic change summary builder for:
- title, description, start date, end date, and status changes
- update text changes
- checker-originated transitions
3. Add recipient deduplication and email normalization.

### Phase 5: Producer Integration in Write Paths

1. On maintenance create, publish outbox intent in same transaction as event persistence.
2. On maintenance field or status patch, publish outbox intent in same transaction.
3. On maintenance update text patch, publish outbox intent in same transaction.
4. Ensure no outbox record is created for rejected requests (400/403/409/500).

### Phase 6: Checker Integration

1. When checker persists maintenance status transition, publish outbox intent in same transaction.
2. Mark actor as `checker`.
3. Ensure backfilled internal statuses do not generate duplicate emails in a single checker cycle.

### Phase 7: Worker and Delivery Pipeline

1. Implement background worker:
- poll pending outbox rows
- lock row (`FOR UPDATE SKIP LOCKED` semantics)
- send email
- write delivery log
- mark row as sent or failed
2. Implement bounded retry strategy:
- exponential backoff
- max attempts
3. Implement graceful startup/shutdown lifecycle integration.

### Phase 8: Testing and Verification

1. Unit tests:
- recipient resolution by role
- change summary formatting
- email rendering
- retry/backoff behavior
2. Integration tests:
- create pending review maintenance => role + squad recipients
- patch status => squad recipient
- patch update text => squad recipient
- checker transition => squad recipient
3. Failure-path tests:
- SMTP failure => failed log entry
- DB error => no orphan side effects

### Phase 9: Operations and Rollout

1. Introduce feature-flagged rollout:
- disabled by default in production
- enable in staging first
2. Add operational metrics:
- queue depth
- send success/failure counters
- retry counters
- delivery latency
3. Add runbook for re-drive of failed deliveries.

## User Stories

### Story 1: Creator Gets Change Notifications

As a maintenance creator,
I want my squad email to receive notifications when my maintenance changes,
so that the team is aware of progress and approvals.

Acceptance criteria:
1. Given a maintenance with `contact_email`, when status changes, then an email is sent to `contact_email`.
2. Given a maintenance with `contact_email`, when update text changes, then an email is sent to `contact_email`.
3. Email contains link to maintenance and short change summary.

### Story 2: Review Audience Is Notified About Pending Review

As a reviewer in privileged maintenance roles,
I want to receive email when a maintenance enters pending review,
so that I can approve or reject it quickly.

Acceptance criteria:
1. Given a maintenance is created with initial status `pending_review`, role recipient lists for `operator` and `admin` are notified.
2. Recipient routing is role-based, not tied to a specific IdP group name.
3. Email contains maintenance link and creator identity if available.

### Story 3: Checker Changes Produce Notifications

As a maintenance stakeholder,
I want checker-driven status transitions to generate notifications,
so that automatic lifecycle changes are visible.

Acceptance criteria:
1. When checker moves `reviewed` to `planned`, email is sent to `contact_email`.
2. When checker moves `planned` to `in_progress`, email is sent to `contact_email`.
3. When checker moves `in_progress` to `completed`, email is sent to `contact_email`.
4. Actor in email is `checker`.

### Story 4: Delivery Reliability

As an operator,
I want notifications to be delivered asynchronously and reliably,
so that API latency stays low and delivery intent is not lost.

Acceptance criteria:
1. API success does not wait for SMTP response.
2. Notification intent is persisted atomically with maintenance change.
3. Delivery attempts and failures are persisted in notification logs.

### Story 5: Observability and Supportability

As a support engineer,
I want visibility into notification processing,
so that I can diagnose and recover from failures quickly.

Acceptance criteria:
1. Failed deliveries contain error reason in logs.
2. Metrics expose sent, failed, retry, and queue depth signals.
3. Failed outbox entries can be identified for manual re-drive.

### Story 6: Security of Notification Configuration

As a security-conscious platform owner,
I want SMTP secrets protected,
so that credentials are not leaked through logs or responses.

Acceptance criteria:
1. SMTP credentials are never printed in plaintext logs.
2. Notification endpoints and processing expose no secret values in API responses.
3. Enabling notifications with invalid SMTP configuration fails fast at startup.
