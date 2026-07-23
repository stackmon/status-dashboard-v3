# Final Scope: Maintenance Email Notifications

## Purpose

This document defines the implementation scope for maintenance email notifications.

Scope is intentionally limited to **email notifications only** for maintenance events.

## In Scope

1. Notification channel: SMTP email only.
2. Event type: maintenance only.
3. Trigger sources:
- Maintenance creation (`POST /v2/events`, type `maintenance`).
- Maintenance status changes via API (`PATCH /v2/events/:eventID`).
- Automatic checker transitions (`planned`, `in_progress`, `completed`).
4. Recipient routing based on the resulting maintenance status.
5. Reliable asynchronous delivery via transactional outbox.
6. Failure visibility on the outbox record.

## Recipients

There are two recipient audiences:

1. **Review audience** — the RBAC roles that can review and approve maintenance: **Operator** and
   **Admin** (see [../auth/rbac.md](../auth/rbac.md), [../auth/permissions.md](../auth/permissions.md)).
   Their addresses are predefined per-role email lists in configuration
   (`SD_NOTIFICATIONS_EMAILS_OPERATORS`, `SD_NOTIFICATIONS_EMAILS_ADMINS`), not derived from the
   request token. Operationally this is the SMOD team.
2. **Creator** — the maintenance contact address supplied in the create request
   (`contact_email`) and stored in `incident.contact_email` (for example `creator@com.com`).

## Recipient Rules

Recipients are determined by the **resulting maintenance status**:

| Resulting status | Review audience (Operator + Admin) | Creator |
|------------------|:----------------------------------:|:-------:|
| `pending_review` | ✅ | ✅ |
| `reviewed` | ✅ | ✅ |
| `planned` | ❌ | ✅ |
| `in_progress` | ❌ | ✅ |
| `completed` | ❌ | ✅ |
| `cancelled` | ❌ | ✅ |

- Review states (`pending_review`, `reviewed`) notify the review audience (operators and admins) and
  the creator.
- Lifecycle states (`planned`, `in_progress`, `completed`, `cancelled`) notify the creator only.
- The rule is identical whether the status changed via the API or via the checker.

## Event-to-Email Matrix

1. `maintenance_pending_review`
- Trigger: maintenance created with resulting status `pending_review`.
- Recipients: review audience (Operator + Admin) + creator.

2. `maintenance_reviewed`
- Trigger: maintenance moved to `reviewed` (approved).
- Recipients: review audience (Operator + Admin) + creator.

3. `maintenance_status_changed`
- Trigger: maintenance moved to `planned`, `in_progress`, `completed`, or `cancelled`
  (via API or checker).
- Recipients: creator.

## Email Contract (Minimum)

Each email must include:

1. Maintenance title.
2. Event ID.
3. Current status.
4. Short change summary (what changed).
5. Actor:
- user id (`preferred_username`) for user-initiated changes, or
- `checker` for automatic transitions.
6. Timestamp of change in UTC.
7. Direct link to maintenance details page.

## Data and Delivery Model

### Transactional Outbox

**Why an outbox instead of sending directly:** sending email inside the request would force the API
to wait on the mail server and could lose a notification if the process restarts mid-send. Recording
the email task in a table first decouples the two concerns — the API stays fast, and a committed
maintenance change always has a durable email task that survives restarts.

1. The email task is written to `notification_outbox` in the same transaction as the maintenance
   change.
2. A background worker pulls pending rows and sends email asynchronously.
3. The delivery outcome (`sent` / `failed` + error) is recorded on the outbox row itself.
   No separate log table is used.

### Reliability Requirements

1. API response must not wait for SMTP send.
2. No email task loss after a successful business transaction commit.
3. Concurrency-safe worker processing across multiple pods.
4. Delivery semantics are at-least-once; a rare duplicate is possible if SMTP accepts an email
   immediately before the sending pod terminates.

## System Design Principles

1. Decoupled producer/consumer model:
- API/checker write email tasks.
- Background worker performs delivery.

2. Durable queue semantics:
- Outbox acts as a durable queue in the primary DB.
- State transitions are explicit (`pending`, `processing`, `sent`, `failed`).

3. At-least-once delivery with best-effort duplicate prevention:
- Worker may retry the same item.
- Atomic claiming prevents concurrent processing by different pods.

4. Backoff and bounded retries:
- Progressive delay between attempts.
- Finite maximum attempts, then `failed` with a reason.

5. Recipient handling:
- One outbox row per recipient, so a failed delivery to one address is retried on its own without
  re-sending to the others.
- Addresses normalized and deduplicated per change, so no recipient is emailed twice for the same
  change.

6. Clear event classification:
- Notification type is deterministic from the resulting maintenance status.

7. Operational observability:
- Metrics for queue depth, success/failure, retries, and latency.
- Structured logs with outbox id and event id.

8. Failure isolation:
- Notification failures never fail business API requests.

9. Secure-by-default configuration:
- SMTP credentials are never logged in plain text.
- Invalid notification config fails fast at startup.

## Configuration

1. Feature toggle:
- `SD_NOTIFICATIONS_ENABLED`

2. SMTP:
- `SD_SMTP_HOST`
- `SD_SMTP_PORT`
- `SD_SMTP_FROM`
- `SD_SMTP_USER`
- `SD_SMTP_PASSWORD`
- `SD_SMTP_TLS`

> **Transport:** the backend connects **directly** to the OTC (Open Telekom Cloud) SMTP endpoint.
> No external mail gateway or HTTP mail API is used. Email is composed and sent with the maintained
> `github.com/wneessen/go-mail` library, not bare `net/smtp`.

3. Review audience recipients (RBAC roles that approve maintenance):
- `SD_NOTIFICATIONS_EMAILS_OPERATORS`
- `SD_NOTIFICATIONS_EMAILS_ADMINS`

4. Creator recipient:
- Not configured — taken from the maintenance `contact_email` field stored in the DB.

## Security and Compliance Constraints

1. Do not expose SMTP secrets in logs.
2. Keep auditability: the outbox row must retain final status and failure reason.
3. Notification routing must not grant new API permissions.

## Acceptance Criteria

1. Creating a maintenance in `pending_review` sends email to the review audience (operators and
   admins) and the creator.
2. Moving a maintenance to `reviewed` sends email to the review audience (operators and admins) and
   the creator.
3. Moving a maintenance to `planned`, `in_progress`, `completed`, or `cancelled` sends email to the
   creator only.
4. Checker-driven transitions follow the same rules as API-driven ones.
5. Email body includes a link and a short change summary.
6. Failed sends are recorded on the outbox row and visible for operations.
7. Notification handling is limited to SMTP email for maintenance events.

## User Stories

### Story 1: Creator Gets Change Notifications

As a maintenance creator,
I want the contact address to receive an email when the maintenance changes,
so that the team is aware of progress and approvals.

Acceptance criteria:
1. When the maintenance status changes, an email is sent to `contact_email`.
2. The email contains a link to the maintenance and a short change summary.

### Story 2: Review Audience Is Notified About Review States

As an operator or admin (review audience),
I want an email when a maintenance needs review or is approved,
so that I can act on it quickly.

Acceptance criteria:
1. When a maintenance is created in `pending_review`, the operator and admin review lists are notified.
2. When a maintenance is moved to `reviewed`, the operator and admin review lists are notified.
3. The review addresses come from `SD_NOTIFICATIONS_EMAILS_OPERATORS` and `SD_NOTIFICATIONS_EMAILS_ADMINS`.

### Story 3: Checker Changes Produce Notifications

As a maintenance stakeholder,
I want checker-driven status transitions to generate notifications,
so that automatic lifecycle changes are visible.

Acceptance criteria:
1. When the checker moves a maintenance to `planned`, `in_progress`, or `completed`,
   an email is sent to the creator.
2. The actor in the email is `checker`.

### Story 4: Delivery Reliability

As an operator,
I want notifications to be delivered asynchronously and reliably,
so that API latency stays low and no email task is lost.

Acceptance criteria:
1. API success does not wait for the SMTP response.
2. The email task is persisted atomically with the maintenance change.
3. Delivery outcome and failures are persisted on the outbox row.

### Story 5: Observability and Supportability

As a support engineer,
I want visibility into notification processing,
so that I can diagnose and recover from failures quickly.

Acceptance criteria:
1. Failed deliveries retain an error reason on the outbox row.
2. Metrics expose sent, failed, retry, and queue depth signals.
3. Failed outbox rows can be identified for manual re-drive.

### Story 6: Security of Notification Configuration

As a security-conscious platform owner,
I want SMTP secrets protected,
so that credentials are not leaked through logs or responses.

Acceptance criteria:
1. SMTP credentials are never printed in plaintext logs.
2. Notification processing exposes no secret values in API responses.
3. Enabling notifications with invalid SMTP configuration fails fast at startup.

## Implementation Plan

The step-by-step build order is maintained separately in [plan.md](plan.md).
