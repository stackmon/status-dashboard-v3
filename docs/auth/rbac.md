# Role-Based Access Control (RBAC)

## Overview

The Status Dashboard implements RBAC for maintenance event management. Roles are extracted from the JWT `groups` claim and mapped to application permissions.

## Roles

Three application roles are supported, with highest privilege taking precedence when a user has multiple roles.
Role names in this document refer to abstract application roles. Each role is mapped from an IdP group
configured via the corresponding environment variable (e.g. `SD_RBAC_GROUP_ADMINS` → `admin` role).

| Role | Priority | Description |
|------|----------|-------------|
| `admin` | Highest | Full access to all operations; will gain additional system-level privileges in future releases |
| `operator` | Medium | Full CRUD access to all maintenance events (event admin) |
| `creator` | Lowest | Create and manage own maintenance events |

## Configuration

RBAC is always active — there is no disable toggle. `SD_RBAC_GROUP_ADMINS` is mandatory;
`SD_RBAC_GROUP_OPERATORS` and `SD_RBAC_GROUP_CREATORS` are optional (when omitted, no user
can match the corresponding role).

| Environment Variable | Required | Description |
|---------------------|----------|-------------|
| `SD_RBAC_GROUP_ADMINS` | **Yes** | IdP group name that maps to the `admin` role |
| `SD_RBAC_GROUP_OPERATORS` | No | IdP group name that maps to the `operator` role |
| `SD_RBAC_GROUP_CREATORS` | No | IdP group name that maps to the `creator` role |

## Permissions by Role

### `admin`

- Unrestricted access to all maintenance operations
- Bypass all status-based restrictions
- Can transition events to any status
- Events created with status `planned` (bypass review)

### `operator`

- Full CRUD access to all maintenance events (event admin)
- Create events with status `planned` (bypass review workflow)
- View all maintenance events regardless of status
- PATCH events from any status to any valid maintenance status
- Cancel events from any status

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
  └──────┬──────┘     operator/admin can approve, modify, or cancel
         │ operator/admin approves (or any PATCH by operator/admin)
         ▼
  ┌─────────────┐
  │  reviewed   │
  └──────┬──────┘
         │ Checker auto-transitions (no validation)
         ▼
  ┌─────────────┐
  │   planned   │ ◄── operator/admin bypass to here on creation
  └──────┬──────┘
         │ Checker (StartDate reached)
         ▼
  ┌─────────────┐
  │ in_progress │
  └──────┬──────┘
         │ Checker (EndDate reached)
         ▼
  ┌─────────────┐
  │  completed  │  (terminal)
  └─────────────┘

cancelled  ◄── admin: from any status
           ◄── operator: from any status
           ◄── creator: from pending_review (own event) only
```

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
| `401 Unauthorized` | Missing or invalid JWT token |
| `403 Forbidden` | Insufficient role permissions |
| `403 Forbidden` | Attempting to modify event you don't own (`creator` role) |
| `409 Conflict` | Status transition not allowed for current role/state |
| `409 Conflict` | Version mismatch (concurrent modification) |
| `409 Conflict` | Event no longer in expected status |

## JWT Token Structure

The API expects JWT tokens with the following claims:

```json
{
  "preferred_username": "user@example.com",
  "groups": ["sd_creators", "other-group"]
}
```

- `preferred_username` → stored as event creator
- `groups` → each value is matched against the configured `SD_RBAC_GROUP_*` environment variables to
  resolve the application role. For example, if `SD_RBAC_GROUP_CREATORS=sd_creators`, then a token
  containing `"sd_creators"` in `groups` grants the `creator` role.

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
