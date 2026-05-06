# Event Permissions & Status Transition Matrix

This document defines the complete permission model for all event types (`maintenance`, `incident`, `info`),
covering creation rules, PATCH status transitions, and automatic checker transitions.

---

## Role Hierarchy

```
Admin (50) > Operator (30) > Creator (10) > NoRole (0)
```

Role names (`admin`, `operator`, `creator`) are abstract application roles resolved from IdP group
names configured via `SD_RBAC_GROUPS_ADMINS`, `SD_RBAC_GROUPS_OPERATORS`, and `SD_RBAC_GROUPS_CREATORS`
environment variables. See [rbac.md](rbac.md) for configuration details.

---

## Maintenance Events

### Creation

| Role | Initial Status | Start Date Constraint | Notes |
|------|---------------|----------------------|-------|
| `admin` | `planned` | No restriction — past or future allowed | Bypasses review workflow |
| `operator` | `planned` | No restriction — past or future allowed | Bypasses review workflow |
| `creator` | `pending_review` | No restriction — past or future allowed | Enters review workflow |
| Unauthenticated | — | — | 401 Unauthorized |

> **Retroactive maintenance**: All roles may create maintenance events with start/end dates in the past.
> This is intentional and supports the use case of documenting maintenance that was not recorded in time.
> See [Retroactive Maintenance Workflow](#retroactive-maintenance-workflow) below.

**Required fields at creation:**
- `title`
- `description`
- `contact_email` (RFC 5322 format)
- `start_date`
- `end_date` (must be after `start_date`)
- `components` (at least one)

---

### PATCH Status Transitions

The table below shows which target statuses are reachable from each stored status, per role.
**Rows = current (stored) status. Columns = requested (incoming) status.**

All roles are subject to the **state machine** — only valid transitions are permitted regardless
of role. The state machine defines the allowed status flow; roles determine **which events** a user
may modify, not which transitions are valid.

Legend: ✅ allowed · ❌ 400 Bad Request (invalid transition) · 🚫 403 Forbidden · ⛔ terminal state

#### State Machine (applies to all roles)

| Current status (FROM ↓) \ Target status (TO →) | `pending_review` | `reviewed` | `planned` | `in_progress` | `modified` | `completed` | `cancelled` |
|------------------------------------------------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `pending_review` | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| `reviewed` | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ |
| `planned` | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ |
| `in_progress` | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| `modified` | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ | ✅ |
| `completed` | ⛔ | ⛔ | ⛔ | ⛔ | ⛔ | ⛔ | ⛔ |
| `cancelled` | ⛔ | ⛔ | ⛔ | ⛔ | ⛔ | ⛔ | ⛔ |

> **Terminal states**: `completed` and `cancelled` have no outgoing transitions.
> Setting the same status again is rejected (400) except `pending_review → pending_review`
> which is explicitly allowed so creators can update the event text.

#### `admin` / `operator` — state machine enforced

Admin and operator pass the RBAC check unconditionally (any event, any stored status).
However, the state machine still restricts which transitions are valid.
The distinction from `admin` is that `admin` will gain additional system-level privileges in future
releases; `operator` scope is limited to event management.

**Effective matrix** = the state machine table above.

#### `creator` — restricted to own events in `pending_review`

Creators may only PATCH their **own** events (ownership check: `user_id == created_by`).
If the event belongs to another user: **403 Forbidden**.
If the stored status is not `pending_review`: **409 Conflict**.

| Current status (FROM ↓) \ Target status (TO →) | `pending_review` | `reviewed` | `planned` | `in_progress` | `modified` | `completed` | `cancelled` |
|------------------------------------------------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `pending_review` (own event) | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| `pending_review` (other's event) | 🚫 | 🚫 | 🚫 | 🚫 | 🚫 | 🚫 | 🚫 |
| any other status | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

---

### Automatic Transitions (Checker Goroutine)

The `checker` goroutine runs periodically and transitions maintenance events based on wall-clock time.
These transitions require no user action and are not subject to RBAC.

```
reviewed   ──────────────────────────────► planned
                  (checker, unconditional)

planned    ──[StartDate reached]──────────► in_progress
                  (checker, time-based)

in_progress ──[EndDate reached]───────────► completed
                  (checker, time-based)
```

If a `cancelled` status entry ever exists in the event's status history, the checker preserves
`cancelled` as the final status regardless of dates.

The checker also backfills missing intermediate statuses (e.g., if an event jumps directly to
`completed`, the checker adds `planned` and `in_progress` entries with correct timestamps).

---

### Retroactive Maintenance Workflow

Maintenance events may be created with start and/or end dates in the past. This supports the
operational scenario of documenting maintenance that was performed but not registered in time.

#### Path A — `admin` or `operator` creates with past dates

```
POST /v2/events  { start_date: <past>, end_date: <past> }
        │
        ▼ initial status: planned
        │
        ▼ next checker run
   calculateCurrentMntStatus: now > EndDate → completed
        │
        ▼ fixMntMissedStatuses(completed):
          - adds planned  entry (timestamp = StartDate)
          - adds in_progress entry (timestamp = StartDate)
          - adds completed entry (timestamp = EndDate)
```

Result: the event appears in history with correct timestamps, as if it had been recorded in real time.
No manual PATCH is required after creation.

#### Path B — `creator` creates with past dates

```
POST /v2/events → initial status: pending_review
        │
        │ (checker leaves pending_review unchanged)
        │
        ▼ Operator approves via PATCH → reviewed
        │
        ▼ checker: reviewed → planned → (EndDate in past) → completed
          fixMntMissedStatuses fills history with correct timestamps
```

---

### Full Maintenance Status Flow Diagram

```
                        State Machine (enforced for ALL roles)
                        ======================================

[creator POST]──► pending_review ──┬──► reviewed ──► planned ──► in_progress ──┬──► completed (terminal)
                      │ ▲          │                                  │ ▲       │
                      │ │          │                                  │ │       │
                      └─┘          │                                  ▼ │       │
                (self-transition   │                               modified ────┘
                 allowed)          │                                  │
                                   │                                  │
                                   ▼                                  ▼
[operator/admin POST]──► planned   └───────────────────────────────────────────► cancelled (terminal)
                                         (from any non-terminal status)

Legend:
  ──► = valid transition (state machine)
  All roles: admin, operator, creator are subject to the state machine
  RBAC restricts WHO can trigger transitions, not WHICH transitions exist
  Creator: can only operate on own events in pending_review status
  Admin/Operator: can operate on any event in any non-terminal status
```

---

## Incident Events

### Creation

| Role | Constraint |
|------|-----------|
| Any authenticated | `start_date` must NOT be in the future (incidents are present/past events) |
| Any authenticated | `end_date` must be omitted at creation |

### PATCH Status Transitions

Incidents do not use the RBAC `allowMaintenancePatch` path. Any authenticated user with a valid
role may patch incidents. Key rules:

- If the incident has an `end_date` (closed), only closed statuses (`resolved`, `reopened`, `changed`)
  are accepted, unless using `IncidentChanged` to update dates.
- `impact` changes require status `impact changed`.
- `start_date` cannot be changed on an open incident.
- Status `resolved` automatically sets `end_date` to `update_date`.
- Status `reopened` clears `end_date`.

---

## Informational Events

### Creation

No special role restrictions beyond authentication. `impact` must be `0`.

### PATCH Status Transitions

Allowed statuses: `planned`, `active`, `completed`, `cancelled`.
Any authenticated user may patch info events to any of these statuses.

---

## Visibility Rules

| Condition | `pending_review` / `reviewed` maintenance | `creator` field | `contact_email` field | `version` field |
|-----------|:-----------------------------------------:|:---------------:|:--------------------:|:---------------:|
| Unauthenticated | Hidden (404) | Hidden | Hidden | Hidden |
| Authenticated (any role) | Visible | Visible | Visible | Visible for maintenance events only |

---

## HTTP Error Reference

| Code | Trigger |
|------|---------|
| `400 Bad Request` | Invalid request body, missing required fields, invalid email format, `end_date` before `start_date`, invalid state transition (state machine violation), setting same status again |
| `401 Unauthorized` | Missing or invalid JWT token |
| `403 Forbidden` | Insufficient role, or creator attempting to modify another user's event |
| `409 Conflict` | Status transition not permitted for current role/state (creator outside `pending_review`), or version mismatch |
| `500 Internal Server Error` | Database or unexpected server error |
