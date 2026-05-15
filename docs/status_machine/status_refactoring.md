# Status Machine Refactoring

## Problem

The current architecture tightly couples the **message log** and the **event status**. Every record in `incident_status` must contain a status — a user cannot add an informational message without changing the event state.

```
Current flow:
User wants to write "Contacted the network team"
  → Must select a status (analysing? fixing? observing?)
  → Status changes even though no real transition occurred
  → Transition history is distorted
```

---

## Solution

Make the `status` field in the `incident_status` table **nullable**. Records without a status are free-form messages. Records with a status are FSM transitions.

```
New flow:
User writes "Contacted the network team"
  → Record is saved with status = NULL
  → Event status does NOT change

User writes "Root cause identified" + status = "fixing"
  → Record is saved with status = "fixing"
  → Event status is updated to "fixing"
```

---

## Architecture

### Data Model

#### Table `incident_status` (modification)

| Field | Type | Before | After |
|-------|------|--------|-------|
| id | SERIAL | PK | PK |
| incident_id | INT | FK, NOT NULL | FK, NOT NULL |
| **status** | **VARCHAR(50)** | **NOT NULL** | **NULL** |
| text | TEXT | NOT NULL | NOT NULL |
| timestamp | TIMESTAMPTZ | NOT NULL | NOT NULL |
| created_at | TIMESTAMPTZ | — | — |
| modified_at | TIMESTAMPTZ | — | — |
| deleted_at | TIMESTAMPTZ | — | — |

#### Table `incident` (no schema changes)

The `incident.status` field remains the **source of truth** for the current event state. It is not computed from the last record in `incident_status`.

### Go Model

```go
// Before
type IncidentStatus struct {
    ID         uint         `json:"-" gorm:"primaryKey;autoIncrement:true;"`
    IncidentID uint         `json:"-"`
    Status     event.Status `json:"status"`
    Text       string       `json:"text"`
    Timestamp  time.Time    `json:"timestamp"`
    ...
}

// After
type IncidentStatus struct {
    ID         uint          `json:"-" gorm:"primaryKey;autoIncrement:true;"`
    IncidentID uint          `json:"-"`
    Status     *event.Status `json:"status"`     // nullable — nil means free-form message
    Text       string        `json:"text"`
    Timestamp  time.Time     `json:"timestamp"`
    ...
}
```

---

## Business Logic

### Record Types

| `status` | Record Type | Description |
|----------|-------------|-------------|
| `nil` | Informational | Free-form user message, event status does not change |
| `"analysing"` | Transition | Status change + mandatory explanation |

### Operations

| Operation | Condition | Result |
|-----------|-----------|--------|
| Add message | `text` is required | Record with `status = NULL`, event status unchanged |
| Change status | `text` + `status` both required | Record with `status = <new>`, `event.status` is updated |
| Edit (free-form record) | `status IS NULL` | Can modify `text`, `timestamp` |
| Delete (free-form record) | `status IS NULL` | Allowed |
| Edit (status record) | `status IS NOT NULL` | Can modify **only `text`** |
| Delete (status record) | `status IS NOT NULL` | **Forbidden** |

### Concurrency and Data Integrity

To prevent race conditions during status transitions, the `incident.status` read-modify-write cycle must be wrapped in a database transaction with a `SELECT ... FOR UPDATE` row lock on the `incident` record.

### Terminal Events

Free-form messages (`status IS NULL`) can be added to terminal events (e.g., `resolved`, `completed`, `cancelled`) to support post-mortems and retrospective notes without reopening the event.

### FSM (unchanged)

Allowed status transitions remain the same:

```
Incident:     detected → analysing → fixing → observing → resolved
                                           → impact changed
Closed:       resolved → reopened → (open statuses)
                       → changed

Maintenance:  planned → in progress → completed
                                    → cancelled
                     → modified

Info:         planned → active → completed
                              → cancelled
```

---

## API

### Add Message (new endpoint)

```
POST /v2/events/:eventID/updates
```

```json
{
  "text": "Contacted the network team, awaiting response",
  "timestamp": "2025-05-20T12:15:00Z" // Optional. Defaults to time.Now().UTC()
}
```

Response: `201 Created` with the created record.

### Change Status (existing PATCH)

```
PATCH /v2/events/:eventID
```

```json
{
  "status": "fixing",
  "message": "Root cause identified — BGP flap on edge router",
  "update_date": "2025-05-20T12:45:00Z"
}
```

This endpoint is strictly for state transitions (and associated metadata updates). Both `status` and `message` are **mandatory**. The record is automatically added to the timeline with `status = "fixing"`.

### Edit Record (existing PATCH)

```
PATCH /v2/events/:eventID/updates/:updateID
```

```json
{
  "text": "Corrected message text"
}
```

Validation:
- `updateID` is the database primary key (`IncidentStatus.ID`), not the array index.
- If `status IS NOT NULL` → only `text` is accepted
- If `status IS NULL` → both `text` and `timestamp` are accepted

### Delete Record (new endpoint)

```
DELETE /v2/events/:eventID/updates/:updateID
```

Validation:
- `updateID` is the database primary key (`IncidentStatus.ID`), not the array index.
- If `status IS NOT NULL` → `409 Conflict`: "cannot delete status transition record"
- If `status IS NULL` → record is deleted, `204 No Content`

---

## Frontend Representation

A single `updates` array sorted by `timestamp`:

```json
{
  "updates": [
    { "id": 1, "status": "detected",  "text": "The incident is detected.",      "timestamp": "2025-05-20T10:00:00Z" },
    { "id": 2, "status": null,        "text": "Investigating root cause",        "timestamp": "2025-05-20T10:15:00Z" },
    { "id": 3, "status": null,        "text": "Network team engaged",            "timestamp": "2025-05-20T10:30:00Z" },
    { "id": 4, "status": "fixing",    "text": "Root cause found — BGP flap",     "timestamp": "2025-05-20T10:45:00Z" },
    { "id": 5, "status": null,        "text": "Fix applied, monitoring",         "timestamp": "2025-05-20T11:10:00Z" },
    { "id": 6, "status": "resolved",  "text": "All services recovered.",         "timestamp": "2025-05-20T11:30:00Z" }
  ]
}
```

Frontend renders based on the `status` value:
- `null` → plain message (neutral style)
- non-null → status badge (color, icon, highlight)

---

## Migration

### SQL

```sql
-- 000006_nullable_status.up.sql
ALTER TABLE incident_status ALTER COLUMN status DROP NOT NULL;

-- 000006_nullable_status.down.sql
-- Before rollback: remove records without status (if any)
DELETE FROM incident_status WHERE status IS NULL;
ALTER TABLE incident_status ALTER COLUMN status SET NOT NULL;
```

All existing records have `status IS NOT NULL` — migration is safe, no data loss.

### Backward Compatibility

- Existing clients that always pass `status` in PATCH — continue working without changes
- JSON response retains the same structure, `status` can now be `null`
- The `GET /v2/events` endpoint returns `updates` in the same format

---

## Future Extensions

### `author` Field (after merge of feature/rbac)

```sql
ALTER TABLE incident_status ADD COLUMN author VARCHAR(255) NULL;
```

Does not block the current implementation. Added as a separate migration when RBAC is ready for all event types.

---

## Affected Components

| File | Change |
|------|--------|
| `internal/db/models.go` | `Status event.Status` → `Status *event.Status` |
| `internal/api/v2/v2.go` | New POST handler for updates; DELETE handler; update all `IncidentStatus` instantiations to use pointer; `EventUpdateData.Status` → pointer |
| `internal/api/v2/validation.go` | Forbid deletion of status records |
| `internal/api/routes.go` | New routes: `POST .../updates`, `DELETE .../updates/:updateID` |
| `internal/api/middleware.go` | Add `DELETE` to allowed CORS methods |
| `internal/db/db.go` | Methods: `AddEventUpdate`, `DeleteEventUpdate`, use `SELECT FOR UPDATE` transaction for transitions |
| `internal/event/event.go` | No changes (FSM transitions remain the same) |
| `db/migrations/` | New migration: nullable status |
