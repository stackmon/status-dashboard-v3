# Notification System -- Design Concept

## Overview

The notification system enables subscribers to receive real-time alerts when events
(incidents, maintenances, informational notices) are created or their status changes
on the Status Dashboard.

**Phase 1** delivers email notifications.
**Phase 2** extends the system to MS Teams via webhook (plugin architecture).

### Design Principles

1. **Subscriptions = Self-Service (Phase 1)**: End-users subscribe themselves via
   authenticated API and double opt-in email confirmation. No admin involvement
   required for basic email alerts.

2. **Policies = Admin-Managed Routing (Phase 2)**: Admins configure notification
   policies that route events to contact points (Teams webhooks, shared mailboxes)
   using label-based matching, timing controls, and grouping -- similar to Grafana.

3. **Subscriptions and Policies are independent paths**: They co-exist. A single
   event may trigger both self-service subscription emails AND policy-routed webhooks.

4. **Security-first**: No anonymous subscription creation (auth required, anti-bot).
   Double opt-in prevents abuse. Tokens are hashed at rest and never exposed via API.

5. **Signal depth is subscriber-controlled**: Each subscription specifies which signal
   types it receives (`created`, `transition`, `resolved`, `message`, `all`).

### Related Branches

| Branch | Status | Impact on Notifications |
|--------|--------|------------------------|
| `feature/rbac` | Testing | Admin-only endpoints will use `rbac.Service` roles; `contact_email` field in `incident` may pre-populate subscriber suggestions |
| `status-refactoring` | Concept | Nullable `incident_status.status` field means notifications must distinguish **status transitions** from **free-form messages** -- only transitions trigger alerts |

---

## Architecture

### High-Level Component Diagram

```
+------------------------------------------------------------------+
|                     Status Dashboard API                         |
|                                                                  |
|  +--------------+  +------------------+  +--------------------+  |
|  | v2 Handlers  |->| Notification     |->| Notifier Plugins   |  |
|  | (triggers)   |  | Service          |  |                    |  |
|  +--------------+  |                  |  |  +--------------+  |  |
|        |           |  - Load subs     |  |  |    Email     |  |  |
|        |           |  - Build message |  |  |   (SMTP)     |  |  |
|        v           |  - Poll outbox   |  |  +--------------+  |  |
|  +--------------+  |  - Log results   |  |  +--------------+  |  |
|  |  Database    |  +------------------+  |  |  MS Teams    |  |  |
|  |  (GORM)     |                         |  |  (webhook)   |  |  |
|  +--------------+                         |  +--------------+  | |
|        |                                  +--------------------+ |
|        v                                                         |
|  +------------------------------------+                          |
|  | notification_subscription (table)  |                          |
|  | notification_outbox (table)  <- NEW|                          |
|  | notification_log (table)           |                          |
|  +------------------------------------+                          |
+------------------------------------------------------------------+
```

### Plugin Interface

```
+-------------------------------------------------+
|           <<interface>> Notifier                 |
+-------------------------------------------------+
| + Send(ctx, recipient string, msg Message) err  |
| + Channel() string                              |
+-------------------------------------------------+
              ^                ^
              |                |
      +-------+------+  +-----+--------+
      |    Email     |  |   MS Teams   |
      |   Notifier   |  |   Notifier   |
      +--------------+  +--------------+
        Phase 1           Phase 2
```

---

## Notification Trigger Flow

### Sequence Diagram -- Event Creation

```
Client           v2 Handler        Database      Notification Svc    Email Notifier
  |                  |                |                 |                  |
  | POST /v2/events  |                |                 |                  |
  |----------------->|                |                 |                  |
  |                  | Save incident  |                 |                  |
  |                  |--------------->|                 |                  |
  |                  |       OK       |                 |                  |
  |                  |<---------------|                 |                  |
  |                  |                |                 |                  |
  |  201 Created     |  +-- async goroutine --+         |                  |
  |<-----------------|  |                     |         |                  |
  |                  |  | OnIncidentCreated() |         |                  |
  |                  |  |----------------------------- >|                  |
  |                  |  |                     |         |                  |
  |                  |  |                     | Load subscribers           |
  |                  |  |                     | for components             |
  |                  |  |                     |-------->|                  |
  |                  |  |                     | subs[]  |                  |
  |                  |  |                     |<--------|                  |
  |                  |  |                     |         |                  |
  |                  |  |                     | For each subscriber:       |
  |                  |  |                     |---Send(ctx, email, msg)--->|
  |                  |  |                     |         |                  |
  |                  |  |                     |         |   SMTP send      |
  |                  |  |                     |         |                  |
  |                  |  |                     | Write notification_log     |
  |                  |  |                     |-------->|                  |
  |                  |  +---------------------+         |                  |
```

### Sequence Diagram -- Status Transition (PATCH)

```
Client           v2 Handler        Database       Notification Svc
  |                  |                |                  |
  |PATCH /v2/events/42                |                  |
  |{"status":"fixing"}                |                  |
  |----------------->|                |                  |
  |                  | Update event   |                  |
  |                  |--------------->|                  |
  |                  |       OK       |                  |
  |                  |<---------------|                  |
  |    200 OK        |                |                  |
  |<-----------------|                |                  |
  |                  |  (async)       |                  |
  |                  |  OnIncidentUpdated(incident, status)
  |                  |---------------------------------->|
  |                  |                |                  |
  |                  |                |  dispatch to all |
  |                  |                |  active notifiers|
```

### Integration with `status-refactoring`

When the `status-refactoring` branch is merged, `incident_status.status` becomes nullable:

```
+--------------------------------------------------------------+
|              Notification Trigger Decision                    |
|                                                              |
|  POST /v2/events/:id/updates                                 |
|  +--------------------------------------------------------+  |
|  |  request.status == nil ?                                |  |
|  |                                                         |  |
|  |  YES (free-form message)      NO (status transition)    |  |
|  |   -> signal = "message"       -> signal = "transition"  |  |
|  |   -> notify only "message"    -> or "resolved" if       |  |
|  |      or "all" subscribers        terminal state         |  |
|  +--------------------------------------------------------+  |
|                                                              |
|  PATCH /v2/events/:id                                        |
|  +--------------------------------------------------------+  |
|  |  Always a status transition                             |  |
|  |   -> signal = "transition" or "resolved"                |  |
|  |   -> Always trigger notification                        |  |
|  +--------------------------------------------------------+  |
|                                                              |
|  POST /v2/events (new event)                                 |
|  +--------------------------------------------------------+  |
|  |  New event created                                      |  |
|  |   -> signal = "created"                                 |  |
|  |   -> Notify all matching subscribers                    |  |
|  +--------------------------------------------------------+  |
+--------------------------------------------------------------+
```

---

## Data Model

### Entity-Relationship Diagram

```
+----------------+       +-----------------------------------------+
|   component    |       |      notification_subscription          |
+----------------+  +---->-----------------------------------------+
| id (PK)        |<-+    | id (PK, SERIAL)                         |
| name           |       | email (VARCHAR 255, NOT NULL)           |
| ...            |       | component_id (FK -> component, CASCADE) |
+----------------+       | event_types (TEXT[])                    |
                         | notify_on (TEXT[], see defaults below)  |
                         | token_hash (VARCHAR 64, UNIQUE)         |
                         | confirmed (BOOL, DEFAULT false)         |
                         | active (BOOL, DEFAULT true)             |
                         | created_at (TIMESTAMPTZ)                |
                         | modified_at (TIMESTAMPTZ)               |
                         +-------------------+---------------------+
                                             | 1
                                             |
                                             | *
                         +-------------------v---------------------+
+----------------+       |      notification_log                   |
|   incident     |       +-----------------------------------------+
+----------------+  +---->                                         |
| id (PK)        |<-+    | id (PK, SERIAL)                         |
| text           |       | subscription_id (FK)                    |
| status         |       | incident_id (FK -> incident, NULL)      |
| type           |       | channel (VARCHAR 50)                    |
| components[]   |       | status ("sent" / "failed")              |
| ...            |       | error_message (TEXT, NULL)              |
+----------------+       | sent_at (TIMESTAMPTZ)                   |
                         +-----------------------------------------+

+-----------------------------------------+
|      notification_outbox                |
+-----------------------------------------+
| id (PK, SERIAL)                         |
| event_type (VARCHAR 50)                 |
| payload (JSONB)                         |
| status (VARCHAR 20, default "pending")  |
| attempts (INT, default 0)              |
| next_attempt_at (TIMESTAMPTZ)          |
| created_at (TIMESTAMPTZ)                |
+-----------------------------------------+

Notes:
  - notify_on DEFAULT: '{created,transition,resolved}'
  - component_id ON DELETE CASCADE (prevents orphaned -> global promotion)
  - token_hash: stored as SHA-256 hash, raw token sent only via email
  - Matching requires: active = true AND confirmed = true
```

### Subscription Matching Logic

```
  Incoming event (incident/maintenance/info)
         |
         v
  +-------------------------------------+
  |  Extract:                           |
  |    - component_ids from event       |
  |    - signal type: "created"         |
  |      or "transition" or "resolved"  |
  |      or "message"                   |
  +-----------------+-------------------+
                    |
                    v
  +-----------------------------------------------------------------+
  |  SELECT * FROM notification_subscription                         |
  |  WHERE active = true                                             |
  |    AND confirmed = true                                          |
  |    AND (component_id IS NULL OR component_id IN (:component_ids))|
  |    AND (event_types @> ARRAY[:event_type]                        |
  |         OR event_types IS NULL)                                  |
  |    AND (notify_on @> ARRAY[:signal]                              |
  |         OR 'all' = ANY(notify_on))                               |
  +-----------------------------------------------------------------+
                    |
                    v
  +-------------------------------------+
  |  Deduplicate by email               |
  |  (one notification per event)       |
  +-------------------------------------+
```

---

## Notification Events Matrix

Events that trigger notifications:

| Event Type | Trigger | Default | Configurable |
|------------|---------|---------|--------------|
| Incident | Created | Yes: `incident_created` | always |
| Incident | Status transition (analysing -> fixing, etc.) | Yes: `incident_updated` | always |
| Incident | Resolved / closed | Yes: `incident_resolved` | always |
| Incident | Free-form message added (status=NULL)* | No | opt-in via `notify_on` |
| Maintenance | Planned | Yes: `maintenance_planned` | always |
| Maintenance | In progress | Yes: `maintenance_started` | always |
| Maintenance | Completed / cancelled | Yes: `maintenance_completed` | always (maps to `resolved` signal) |
| Info | Planned | Yes: `info_planned` | always |
| Info | Active | Yes: `info_active` | always |
| Info | Completed | Yes: `info_completed` | always |

*Applies after `status-refactoring` merge. Until then, all status updates are transitions.

---

## Notification Policies & Routing

The system provides configurable routing and timing
per subscription. This gives subscribers full control over notification depth.

### Subscription Options (`notify_on`)

Each subscription declares which event signals it wants to receive:

```
+--------------------------------------------------------------+
|  notify_on field (TEXT[], per subscription)                   |
|                                                              |
|  Value          | Meaning                                    |
|  ---------------+--------------------------------------------+
|  "created"      | Event creation                             |
|  "transition"   | Status changes (non-terminal)              |
|  "resolved"     | Terminal states (resolved/completed/cancel) |
|  "message"      | Free-form messages only (status=NULL)      |
|  "all"          | Shortcut: everything                       |
+--------------------------------------------------------------+

Default: ["created", "transition", "resolved"]

Examples:
  ["created", "transition", "resolved"]  -- default (all status changes)
  ["all"]                                -- on-call engineer (+ free-form)
  ["created", "resolved"]                -- management (start/end only)
  ["message"]                            -- observer (free-form only)
```

### Notification Policy (Admin-configured, global)

Admins can define **routing rules** that match events by labels and route them
to specific contact points. This is configured via API.

```
+------------------------------------------------------------------+
|                  notification_policy (table)                      |
+------------------------------------------------------------------+
| id         | SERIAL PK                                           |
| name       | VARCHAR(100) -- human-readable label                |
| matchers   | JSONB -- label matching rules                       |
| contact_id | FK -> notification_contact_point                    |
| group_by   | TEXT[] -- group notifications by these fields        |
| group_wait | INTERVAL -- wait before sending first grouped notif |
| group_interval | INTERVAL -- min interval between group sends    |
| repeat_interval | INTERVAL -- re-send if still firing            |
| mute_timings   | TEXT[] -- references to mute timing windows     |
| active     | BOOL DEFAULT true                                    |
| priority   | INT -- evaluation order (lower = first)             |
+------------------------------------------------------------------+

Default policy (priority=9999, catch-all):
  matchers: {} (match everything)
  contact_id: -> default email contact point
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
```

### Routing Flow

```
  Event occurs (incident created, status changed, message posted)
         |
         v
  +------------------------------------------+
  |  Build event labels:                     |
  |    type: "incident"                      |
  |    severity: "critical" (from impact)    |
  |    component: "Object Storage"           |
  |    region: "eu-de"                       |
  |    signal: "transition" | "message"      |
  +--------------------+---------------------+
                       |
                       v
  +------------------------------------------+
  |  Evaluate policies (sorted by priority)  |
  |                                          |
  |  Policy 1: severity=critical             |
  |    -> contact: "Ops Team MS Teams"       |
  |    -> group_wait: 0s (immediate)         |
  |                                          |
  |  Policy 2: region=eu-de, type=maint      |
  |    -> contact: "EU Ops Team Email"       |
  |    -> group_wait: 5m                     |
  |                                          |
  |  Default: (catch-all)                    |
  |    -> contact: "General Subscribers"     |
  |    -> group_wait: 30s                    |
  +--------------------+---------------------+
                       |
                       v
  +------------------------------------------+
  |  Apply timing rules:                     |
  |                                          |
  |  1. Check mute_timings (suppress?)      |
  |  2. Group events by group_by fields     |
  |  3. Wait group_wait before first send   |
  |  4. Batch grouped events into one notif |
  |  5. Respect group_interval between sends|
  |  6. Re-notify after repeat_interval     |
  +------------------------------------------+
```

### Timing Parameters

```
Timeline for grouped notifications:

  Event A fires     Event B fires (same group)     Event C fires
      |                  |                              |
      |-- group_wait --->|                              |
      |                  | (batched with A)             |
      |                  |                              |
      +---- SEND (A+B) -+                              |
                         |                              |
                         |--- group_interval ---------->|
                         |                              |
                         |              (C arrives during interval, queued)
                         |                              |
                         +---- SEND (C) ---------------+
                                                       |
                                              repeat_interval...
                                                       |
                                              SEND (reminder if unresolved)
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `group_wait` | 30s | How long to wait for more events before sending first notification |
| `group_interval` | 5m | Minimum time between notifications for the same group |
| `repeat_interval` | 4h | Re-send notification if event is still active (unresolved) |

### Mute Timings

Suppress notifications during known maintenance windows or off-hours:

```
+------------------------------------------------------------------+
|                  notification_mute_timing (table)                |
+------------------------------------------------------------------+
| id        | SERIAL PK                                            |
| name      | VARCHAR(100) -- e.g., "Weekend EU", "Deploy Window"  |
| weekdays  | INT[] -- 0=Sun, 1=Mon, ..., 6=Sat                    |
| time_from | TIME -- e.g., 22:00                                  |
| time_to   | TIME -- e.g., 06:00                                  |
| months    | INT[] -- NULL = all months                           |
| active    | BOOL DEFAULT true                                    |
+------------------------------------------------------------------+

Example: suppress non-critical alerts on weekends
  name: "Weekend Quiet"
  weekdays: [0, 6]
  time_from: 00:00
  time_to: 23:59
```

### Contact Points

A contact point is a named destination (decoupled from individual subscriptions):

```
+------------------------------------------------------------------+
|                  notification_contact_point (table)              |
+------------------------------------------------------------------+
| id        | SERIAL PK                                            |
| name      | VARCHAR(100) -- "EU Ops Team", "Critical Teams"    |
| channel   | VARCHAR(50)  -- "email", "teams_webhook"             |
| config    | JSONB        -- channel-specific settings            |
| active    | BOOL DEFAULT true                                    |
+------------------------------------------------------------------+

config examples:
  email:         {"recipients": ["ops@example.com", "sre@example.com"]}
  teams_webhook: {"url": "https://outlook.office.com/webhook/..."}
```

---

## API Endpoints

### Subscription Management

```
+------------------------------------------------------------------+
|                                                                   |
|  AUTHENTICATED (any authorized user)                              |
|  -----------------------------------                              |
|                                                                   |
|  POST   /v2/subscriptions              Create a new subscription  |
|                                        (Triggers Double Opt-In)   |
|                                                                   |
|  PUBLIC (no auth required)                                        |
|  -------------------------                                        |
|                                                                   |
|  GET    /v2/subscriptions/confirm      Confirm email (activate)   |
|  DELETE /v2/subscriptions/unsubscribe  Self-service unsubscribe   |
|                                        (Requires Rate Limiting)   |
|                                                                   |
|  ADMIN ONLY (RBAC: Admin role)                                    |
|  -----------------------------                                    |
|                                                                   |
|  GET    /v2/subscriptions              List all subscriptions     |
|  DELETE /v2/subscriptions/:id          Remove subscription by ID  |
|                                                                   |
+------------------------------------------------------------------+
```

### Request / Response Examples

#### Create Subscription

```http
POST /v2/subscriptions
Content-Type: application/json

{
  "email": "ops-team@example.com",
  "component_id": 5,
  "event_types": ["incident", "maintenance"],
  "notify_on": ["created", "transition", "resolved"]
}
```

```http
HTTP/1.1 202 Accepted

{
  "id": 42,
  "message": "Confirmation email sent. Please check your inbox to activate."
}
```

> **Double Opt-In**: The subscription is created with `confirmed=false`.
> A confirmation email with a unique link is sent to the address.
> Only after clicking the link (`GET /v2/subscriptions/confirm?token=...`)
> does `confirmed` become `true` and notifications start flowing.
>
> **Security**: The unsubscribe token is a separate value, sent only inside
> notification emails (in the footer). It is stored as a SHA-256 hash in DB.
> Neither token is ever returned in API responses.

#### Create Subscription (full depth, on-call engineer)

```http
POST /v2/subscriptions
Content-Type: application/json

{
  "email": "oncall@example.com",
  "notify_on": ["all"]
}
```

#### Self-Service Unsubscribe

```http
DELETE /v2/subscriptions/unsubscribe?token=a1b2c3d4e5f6...
```

```http
HTTP/1.1 200 OK

{
  "message": "Successfully unsubscribed."
}
```

#### Confirm Email (Double Opt-In)

```http
GET /v2/subscriptions/confirm?token=<confirmation_token>
```

```http
HTTP/1.1 200 OK

{
  "message": "Email confirmed. Notifications are now active."
}
```

> Token is valid only once. After confirmation, the confirmation token
> is invalidated and the unsubscribe token takes over for lifecycle control.

---

## Configuration

### Environment Variables

```bash
# Enable/disable notification system
SD_NOTIFICATIONS_ENABLED=true

# SMTP settings
SD_SMTP_HOST=smtp.example.com
SD_SMTP_PORT=587
SD_SMTP_FROM=noreply@status.example.com
SD_SMTP_USER=apikey
SD_SMTP_PASSWORD=<secret>
SD_SMTP_TLS=true
```

### Configuration Structure

```
+-------------------------------------------+
|  Config                                   |
|  +-- DB                                   |
|  +-- Cache                                |
|  +-- Keycloak          (auth)             |
|  +-- RBAC              (feature/rbac)     |
|  +-- NotificationsEnabled (bool)  <-- NEW |
|  +-- SMTP              <-- NEW            |
|       +-- Host                            |
|       +-- Port (default: 587)             |
|       +-- From                            |
|       +-- User                            |
|       +-- Password                        |
|       +-- TLS                             |
+-------------------------------------------+
```

---

## Email Content

### Template Structure

Each email contains:

```
+-----------------------------------------------------------+
| +-------------------------------------------------------+ |
| | HEADER: Event Type Badge + Dashboard Name             ||
| +-------------------------------------------------------+ |
|                                                           |
| +-------------------------------------------------------+ |
| | BODY                                                   ||
| |                                                        ||
| | Event: [Title]                                         ||
| | Status: [detected -> analysing -> ...]                 ||
| | Impact: [minor / major / critical]                     ||
| | Components: [Component A, Component B]                 ||
| |                                                        ||
| | Latest Update:                                         ||
| | "[status update text from operator]"                   ||
| |                                                        ||
| +-------------------------------------------------------+ |
|                                                           |
| +-------------------------------------------------------+ |
| | FOOTER                                                 ||
| | [View on Dashboard]  |  [Unsubscribe]                  ||
| +-------------------------------------------------------+ |
+-----------------------------------------------------------+
```

### Subject Line Format

| Event | Subject |
|-------|---------|
| New incident | `[Incident] <title> -- <component>` |
| Status change | `[Update] <title> -- now <status>` |
| Resolved | `[Resolved] <title>` |
| Maintenance planned | `[Maintenance] <title> -- scheduled` |
| Maintenance started | `[Maintenance] <title> -- in progress` |
| Maintenance completed | `[Maintenance] <title> -- completed` |

---

## Async Dispatch & Error Handling

To prevent data loss if the server crashes after returning HTTP 2xx but before notifications are sent, the system uses the **Transactional Outbox** pattern.

```
API Handler (DB Tx)
       |
       +---> Updates Incident
       |
       +---> Writes to notification_outbox
       |
    Tx Commit -> Returns HTTP 2xx

... (Async Background Poller / Worker) ...

Read from notification_outbox
       |
       +----------+----------+
                  |
       +----------v----------+
       | For each subscriber |
       | +-----------------+ |
       | | notifier.Send   | |
       | +--------+--------+ |
       |          |          |
       |    +-----v-----+    |
       |    | success?  |    |
       |    +-----+-----+    |
       |     yes/ | \no      |
       |        /   \        |
       |  log:sent  log:failed
       |            + error  |
       +----------+----------+
                  |
       Write notification_log
                  |
       Delete from outbox
```

Key design decisions:
- **Non-blocking**: HTTP response returns immediately after the DB transaction commits.
- **Zero Data Loss**: The notification intent is safely stored in the `notification_outbox` table.
- **Panic-safe Worker**: The background worker recovers from panics so a bad email template won't crash the server.
- **Retry policy** (Phase 1): No automatic retries. Failed deliveries are logged for manual review, outbox task is deleted.
- **Retry policy** (Phase 2): Exponential backoff with dead-letter queue (DLQ). **Note:** Grouping/routing in Phase 2 will require distributed coordination (e.g., Redis or DB advisory locks) to prevent race conditions across multiple instances.

---

## RBAC Integration

After `feature/rbac` is merged, the notification system integrates with role-based access:

```
+--------------------------------------------------------------+
|                     RBAC Role Matrix                         |
|                                                              |
|  Endpoint                       NoRole Creator Operator Admin|
|  ------------------------------ ------ ------- -------- -----|
|  POST /v2/subscriptions          No     Yes     Yes     Yes  |
|  DELETE .../unsubscribe?token=   Yes    Yes     Yes     Yes  |
|  GET /v2/subscriptions           No     No      No      Yes  |
|  DELETE /v2/subscriptions/:id    No     No      No      Yes  |
+--------------------------------------------------------------+
```

> **Note**: Until `feature/rbac` is merged, admin-only endpoints use the existing
> `AuthenticationMW` + `authGroup` mechanism with TODO markers for migration.

---

## Compatibility with `status-refactoring`

The `status-refactoring` branch introduces nullable `incident_status.status`:

```
+--------------------------------------------------------------+
|                                                              |
|  incident_status.status                                      |
|  +--------------------------+                                |
|  | NOT NULL (current)       | --> signal = "transition"      |
|  +--------------------------+                                |
|                                                              |
|  +--------------------------+                                |
|  | NULL (after refactor)    | --> signal = "message"         |
|  +--------------------------+                                |
|                                                              |
|  +--------------------------+                                |
|  | Non-NULL (after refact.) | --> signal = "transition"      |
|  +--------------------------+                                |
|                                                              |
+--------------------------------------------------------------+
```

**Implementation strategy**: The notification service classifies each update into
exactly one signal type based on the status field value:

```go
// classifySignal determines the notification signal for an update.
// "message" fires ONLY when Status is nil (free-form), never for transitions.
func classifySignal(update *db.IncidentStatus, isNew bool) string {
    if isNew {
        return "created"
    }
    if update.Status == nil {
        return "message"    // free-form only, opt-in subscribers
    }
    if isTerminal(*update.Status) {
        return "resolved"   // terminal state
    }
    return "transition"     // non-terminal status change
}

func isTerminal(status event.Status) bool {
    switch status {
    case event.IncidentResolved, event.MaintenanceCompleted,
         event.MaintenanceCancelled, event.InfoCompleted, event.InfoCancelled:
        return true
    }
    return false
}

func shouldNotify(signal string, sub *db.NotificationSubscription) bool {
    for _, n := range sub.NotifyOn {
        if n == "all" || n == signal {
            return true
        }
    }
    return false
}
```

This gives subscribers **full control**: by default `created + transition + resolved`
are sent. Only those who explicitly opt into `"message"` or `"all"` receive free-form
updates. A `"message"` subscriber does NOT receive transitions unless also subscribed.

---

## File Structure

```
internal/
+-- notification/
|   +-- notification.go          # Notifier interface, Message, EventType
|   +-- service.go               # Service: subscriber lookup, dispatch, logging
|   +-- service_test.go          # Unit tests with mock Notifier
|   +-- email/
|       +-- email.go             # EmailNotifier: SMTP implementation
|       +-- email_test.go        # Template rendering tests
|       +-- templates/
|           +-- incident_created.html
|           +-- incident_updated.html
|           +-- incident_resolved.html
|           +-- maintenance_planned.html
|           +-- maintenance_started.html
|           +-- maintenance_completed.html
+-- api/
|   +-- api.go                   # + notifySvc field
|   +-- routes.go                # + subscription routes
|   +-- v2/
|       +-- subscriptions.go     # Subscription handlers
+-- conf/
|   +-- conf.go                  # + SMTPConfig, NotificationsEnabled
+-- db/
    +-- models.go                # + NotificationSubscription, NotificationLog
    +-- notification.go          # DB operations for subscriptions/logs

db/migrations/
+-- 000007_notification_subscriptions.up.sql
+-- 000007_notification_subscriptions.down.sql
```

---

## Migration Plan (Database)

### Migration 000007 -- Notification Subscriptions & Outbox

```sql
-- UP
CREATE TABLE notification_outbox (
    id              SERIAL PRIMARY KEY,
    event_type      VARCHAR(50) NOT NULL,
    payload         JSONB NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_outbox_pending
    ON notification_outbox(next_attempt_at) WHERE status = 'pending';

CREATE TABLE notification_subscription (
    id            SERIAL PRIMARY KEY,
    email         VARCHAR(255) NOT NULL,
    component_id  INTEGER REFERENCES component(id) ON DELETE CASCADE,
    event_types   TEXT[],
    notify_on     TEXT[] NOT NULL DEFAULT '{created,transition,resolved}',
    token_hash    VARCHAR(64) NOT NULL UNIQUE,
    confirmed     BOOLEAN NOT NULL DEFAULT false,
    active        BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    modified_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_subscription_email_component
    ON notification_subscription(LOWER(email), COALESCE(component_id, 0));

CREATE INDEX idx_subscription_active
    ON notification_subscription(active) WHERE active = true AND confirmed = true;

CREATE TABLE notification_log (
    id              SERIAL PRIMARY KEY,
    subscription_id INTEGER REFERENCES notification_subscription(id) ON DELETE SET NULL,
    incident_id     INTEGER REFERENCES incident(id) ON DELETE SET NULL,
    channel         VARCHAR(50) NOT NULL,
    status          VARCHAR(20) NOT NULL,
    error_message   TEXT,
    sent_at         TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_notification_log_subscription
    ON notification_log(subscription_id);

CREATE INDEX idx_notification_log_sent_at
    ON notification_log(sent_at);
```

```sql
-- DOWN (000007)
DROP TABLE IF EXISTS notification_log;
DROP TABLE IF EXISTS notification_subscription;
DROP TABLE IF EXISTS notification_outbox;
```

### Migration 000008 -- Notification Policies & Routing (Phase 2)

```sql
-- UP
CREATE TABLE notification_contact_point (
    id        SERIAL PRIMARY KEY,
    name      VARCHAR(100) NOT NULL,
    channel   VARCHAR(50) NOT NULL DEFAULT 'email',
    config    JSONB NOT NULL DEFAULT '{}',
    active    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE notification_mute_timing (
    id        SERIAL PRIMARY KEY,
    name      VARCHAR(100) NOT NULL,
    weekdays  INT[],
    time_from TIME,
    time_to   TIME,
    months    INT[],
    active    BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE notification_policy (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    matchers        JSONB NOT NULL DEFAULT '{}',
    contact_id      INTEGER NOT NULL REFERENCES notification_contact_point(id),
    group_by        TEXT[],
    group_wait      INTERVAL NOT NULL DEFAULT '30 seconds',
    group_interval  INTERVAL NOT NULL DEFAULT '5 minutes',
    repeat_interval INTERVAL NOT NULL DEFAULT '4 hours',
    mute_timings    TEXT[],
    active          BOOLEAN NOT NULL DEFAULT true,
    priority        INTEGER NOT NULL DEFAULT 100
);

CREATE INDEX idx_policy_priority ON notification_policy(priority);
```

```sql
-- DOWN (000008)
DROP TABLE IF EXISTS notification_policy;
DROP TABLE IF EXISTS notification_mute_timing;
DROP TABLE IF EXISTS notification_contact_point;
```

---

## Phase 2 -- MS Teams (Webhook)

Uses the `notification_contact_point` table with channel-specific config:

```
notification_contact_point
+----------------------------------------------+
| id: 3                                        |
| name: "Platform Team MS Teams"               |
| channel: "teams_webhook"                     |
| config: {                                    |
|   "url": "https://outlook.office.com/w/..."  |
| }                                            |
+----------------------------------------------+
        |
        v
TeamsNotifier.Send()
-> POST to Incoming Webhook URL
-> Adaptive Card JSON payload
```

Routing is handled by `notification_policy` matchers:

```
notification_policy
+----------------------------------------------+
| name: "Critical to Teams"                    |
| matchers: {"severity": "critical"}           |
| contact_id: 3 (-> "Platform Team MS Teams")  |
| group_wait: 0s (immediate)                   |
+----------------------------------------------+
```

> **Note**: Phase 2 adds contact points, policies, and mute timings
> (migration 000008). Phase 1 only uses subscriptions + outbox (000007).

---

## Summary

| Aspect | Decision |
|--------|----------|
| Trigger mechanism | Synchronous call from handler, async dispatch (goroutine) |
| Blocking behavior | Non-blocking -- HTTP response returns before send completes |
| Error isolation | `recover()` wrapper, errors logged, never propagated to client |
| Storage | PostgreSQL tables for subscriptions + delivery logs + policies |
| Self-service | Token-based unsubscribe, no auth required |
| Admin management | RBAC Admin role (post-merge) / AuthGroup (pre-merge) |
| Extensibility | `Notifier` interface -- one implementation per channel |
| Channels | Phase 1: Email (SMTP); Phase 2: MS Teams (webhook) |
| Routing | Grafana-style: label matchers, grouping, timing intervals |
| Subscriber control | `notify_on` field: created, transition, resolved (default), message, all |
| Compatibility | Forward-compatible with nullable status (status-refactoring) |
| Dependencies | stdlib only (`net/smtp`, `html/template`, `embed`) |
