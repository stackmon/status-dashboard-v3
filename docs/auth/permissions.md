# Event Permissions & Status Transition Matrix

This document defines the complete permission model for all event types (`maintenance`, `incident`, `info`),
covering creation rules, PATCH status transitions, and automatic checker transitions.

---

## Role Hierarchy

```
Admin (50) > Operator (30) > Creator (10) > NoRole (0)
```

Roles are resolved from the JWT `groups` claim. When a user belongs to multiple groups, the highest
matching role wins. See [rbac.md](rbac.md) for configuration details.

---

## Maintenance Events

### Creation

| Role | Initial Status | Start Date Constraint | Notes |
|------|---------------|----------------------|-------|
| `sd_admins` | `planned` | No restriction — past or future allowed | Bypasses review workflow |
| `sd_operators` | `planned` | No restriction — past or future allowed | Bypasses review workflow |
| `sd_creators` | `pending_review` | No restriction — past or future allowed | Enters review workflow |
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

Legend: ✅ allowed · ❌ 409 Conflict · 🚫 403 Forbidden

#### sd_admins — unrestricted

Any → any maintenance status is permitted. No stored-status check is performed.

| Current status (FROM ↓) \ Target status (TO →) | `pending_review` | `reviewed` | `planned` | `in_progress` | `modified` | `completed` | `cancelled` |
|------------------------------------------------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `pending_review` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `reviewed` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `planned` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `in_progress` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `modified` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `completed` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `cancelled` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

#### sd_operators — restricted to `pending_review` source

Operators may only PATCH events that are currently in `pending_review`.
Any other stored status returns **409 Conflict**.

| Current status (FROM ↓) \ Target status (TO →) | `pending_review` | `reviewed` | `planned` | `in_progress` | `modified` | `completed` | `cancelled` |
|------------------------------------------------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `pending_review` | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| `reviewed` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `planned` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `in_progress` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `modified` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `completed` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `cancelled` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

#### sd_creators — restricted to own events in `pending_review`

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

#### Path A — Admin or Operator creates with past dates

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

#### Path B — Creator creates with past dates

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
[Creator POST]──► pending_review ◄──┐
                      │ │            │ sd_creators: modify (own)
         Operator/    │ │ sd_creators/
         Admin        │ │ Operator/Admin → cancelled (terminal)
         approves     │ │
                      ▼ │
                  reviewed
                      │
               checker (auto)
                      ▼
[Operator/Admin POST]► planned ◄──────────────────────────────┐
                      │                                        │
               checker (StartDate)                    Admin can jump
                      ▼                               to any status
                  in_progress
                      │
               checker (EndDate)
                      ▼
                  completed (terminal)


cancelled ◄── Admin: from any status
cancelled ◄── Operator: from pending_review only
cancelled ◄── Creator: from pending_review (own event) only
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
| Authenticated (any role) | Visible | Visible | Visible | Visible |

---

## HTTP Error Reference

| Code | Trigger |
|------|---------|
| `400 Bad Request` | Invalid request body, missing required fields, invalid email format, `end_date` before `start_date` |
| `401 Unauthorized` | Missing or invalid JWT token |
| `403 Forbidden` | Insufficient role, or creator attempting to modify another user's event |
| `409 Conflict` | Status transition not permitted for current role/state, or version mismatch |
| `500 Internal Server Error` | Database or unexpected server error |
