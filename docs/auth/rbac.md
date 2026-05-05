# Role-Based Access Control (RBAC)

## Overview

The Status Dashboard implements RBAC for maintenance event management. Roles are extracted from the JWT `groups` claim and mapped to application permissions.

## Roles

Three application roles are supported, with highest privilege taking precedence when a user has multiple roles.
Role names in this document refer to abstract application roles. Each role is mapped from an IdP group
configured via the corresponding environment variable (e.g. `SD_RBAC_GROUPS_ADMINS` → `admin` role).

| Role | Priority | Description |
|------|----------|-------------|
| `admin` | Highest | Full access to all operations; will gain additional system-level privileges in future releases |
| `operator` | Medium | Full CRUD access to all maintenance events (event admin) |
| `creator` | Lowest | Create and manage own maintenance events |

## Configuration

RBAC is always active — there is no disable toggle. `SD_RBAC_GROUPS_ADMINS` is mandatory;
`SD_RBAC_GROUPS_OPERATORS` and `SD_RBAC_GROUPS_CREATORS` are optional (when omitted, no user
can match the corresponding role).

Each variable accepts either a single group name or a **comma-separated list** of group names.
All listed groups are mapped to the same role. Group names are matched case-sensitively after
stripping a leading `/` (Keycloak sends groups with a `/` prefix by default).

| Environment Variable | Required | Description |
|---------------------|----------|-------------|
| `SD_RBAC_GROUPS_ADMINS` | **Yes** | Group name(s) that map to the `admin` role |
| `SD_RBAC_GROUPS_OPERATORS` | No | Group name(s) that map to the `operator` role |
| `SD_RBAC_GROUPS_CREATORS` | No | Group name(s) that map to the `creator` role |

**Example** — mapping multiple Keycloak groups to the `admin` role:
```
SD_RBAC_GROUPS_ADMINS=sd-admins,status-dashboard
```
A token containing either `/sd-admins` or `/status-dashboard` in its `groups` claim will be
granted the `admin` role.

## Permissions by Role

### `admin`

- Unrestricted access to all maintenance operations
- Can PATCH any event regardless of ownership or stored status
- Transitions are subject to the **state machine** (only valid status flows are permitted)
- Events created with status `planned` (bypass review)

### `operator`

- Full CRUD access to all maintenance events (event admin)
- Create events with status `planned` (bypass review workflow)
- View all maintenance events regardless of status
- PATCH events from any stored status — state machine enforces valid transitions
- Cancel events from any non-terminal status

### `creator`

- Create maintenance events (status: `pending_review`)
- Modify **own** events only when status is `pending_review`
- Cancel **own** events only when status is `pending_review`
- Cannot modify events after approval (`reviewed`, `planned`, etc.)

## Maintenance Status Workflow

```
Creator creates event
        │
        ▼
  ┌─────────────┐
  │pending_review│ ◄── creator can modify/cancel (own events only)
  └──────┬──────┘     operator/admin can approve or cancel
         │ operator/admin: → reviewed
         ▼
  ┌─────────────┐
  │  reviewed   │
  └──────┬──────┘
         │ operator/admin: → planned (or checker auto-transition)
         ▼
  ┌─────────────┐
  │   planned   │ ◄── operator/admin bypass to here on creation
  └──────┬──────┘
         │ operator/admin: → in_progress (or checker: StartDate reached)
         ▼
  ┌─────────────┐
  │ in_progress │
  └──────┬──┬───┘
         │  │ operator/admin: → modified
         │  ▼
         │ ┌─────────────┐
         │ │  modified   │──► in_progress (loop back)
         │ └──────┬──────┘
         │        │
         ▼        ▼
  ┌─────────────┐
  │  completed  │  (terminal)
  └─────────────┘

cancelled  ◄── from any non-terminal status (all roles subject to RBAC)
           ◄── admin/operator: from pending_review, reviewed, planned, in_progress, modified
           ◄── creator: from pending_review (own event) only
```

> **State machine enforcement**: All transitions are validated by the state machine regardless of role.
> Admin/operator pass the RBAC check unconditionally but the state machine still prevents invalid
> transitions (e.g., `completed → planned` is rejected with 400 Bad Request).
> Terminal states (`completed`, `cancelled`) have no outgoing transitions.

> **Retroactive maintenance**: events may be created with dates in the past. The checker will
> automatically transition them to `completed` and backfill intermediate status history with
> correct timestamps. See [permissions.md](permissions.md) for the full workflow.

For the complete status transition matrix and per-role rules, see [permissions.md](permissions.md).

## Field Visibility

Some fields are only visible to authenticated users:

| Field | Visibility |
|-------|------------|
| `creator` | Authenticated only |
| `contact_email` | Authenticated only |
| `version` | Authenticated only (maintenance events only) |

Events with status `pending_review` or `reviewed` are hidden from unauthenticated users.

## Error Responses

| HTTP Code | Condition |
|-----------|-----------|
| `400 Bad Request` | Invalid state transition (state machine violation), setting same status again |
| `401 Unauthorized` | Missing or invalid JWT token |
| `403 Forbidden` | Insufficient role permissions |
| `403 Forbidden` | Attempting to modify event you don't own (`creator` role) |
| `409 Conflict` | Creator attempting to modify event outside `pending_review` status |
| `409 Conflict` | Version mismatch (concurrent modification) |

## JWT Token Structure

The API expects JWT tokens with the following claims:

```json
{
  "preferred_username": "user@example.com",
  "groups": ["sd_creators", "other-group"]
}
```

- `preferred_username` → stored as event creator
- `groups` → each value is matched against the configured `SD_RBAC_GROUPS_*` environment variables to
  resolve the application role. Leading `/` prefix is stripped before comparison (Keycloak sends
  groups as `/group-name`). For example, if `SD_RBAC_GROUPS_ADMINS=sd-admins,status-dashboard`, then
  a token containing either `/sd-admins` or `/status-dashboard` (or without `/`) in `groups` grants
  the `admin` role.

## Dual-IdP Authentication

The application supports two simultaneous identity providers:

| Provider | JWT Algorithm | Key Source | Use Case |
|----------|-------------|------------|----------|
| **Keycloak (RSA)** | RS256 | JWKS endpoint → public key | Production SSO |
| **Local (HMAC)** | HS256 / HS384 / HS512 | `SD_SECRET_KEY` env var | Dev, tests, service-to-service |

`parseToken` dispatches on `token.Method`: `*jwt.SigningMethodHMAC` → secret key,
`*jwt.SigningMethodRSA` → Keycloak public key. At least one provider must be configured;
`conf.Validate()` fails otherwise.

### Security Hardening

- **Minimum secret key length**: `SD_SECRET_KEY` must be ≥ 32 characters (HMAC-SHA256 requirement).
- **No bypasses**: `SD_AUTHENTICATION_DISABLED` and `SD_RBAC_DISABLED` toggles have been removed.

### Audit Logging

All authentication events are logged in structured SIEM-ready format:

```json
{
  "event": "auth_audit",
  "action": "token_validation",
  "result": "success",
  "idp_type": "keycloak",
  "username": "user@example.com"
}
```

Fields: `event`, `action` (`token_validation` / `authorization`), `result` (`success` / `failure` / `denied`),
`idp_type` (`local_hmac` / `keycloak` / `unknown`), `username`, `reason` (omitted when empty).
