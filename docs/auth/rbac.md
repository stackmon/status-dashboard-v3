# Role-Based Access Control (RBAC)

## Overview

The Status Dashboard implements RBAC for maintenance event management. Roles are extracted from the JWT `groups` claim and mapped to application permissions.

## Roles

Three roles are supported, with highest privilege taking precedence when a user has multiple roles:

| Role | Priority | Description |
|------|----------|-------------|
| **sd_admins** | Highest | Full access to all operations |
| **sd_operators** | Medium | Review and approve maintenance events |
| **sd_creators** | Lowest | Create and manage own maintenance events |

## Configuration

Roles are mapped from IdP group names via environment variables:

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `SD_ADMINS_GROUP` | `admin-group` | IdP group for admin role |
| `SD_OPERATORS_GROUP` | (none) | IdP group for operator role |
| `SD_CREATORS_GROUP` | (none) | IdP group for creator role |

## Permissions by Role

### sd_admins

- Unrestricted access to all maintenance operations
- Bypass all status-based restrictions
- Can transition events to any status
- Events created with status `planned` (bypass review)

### sd_operators

- View all maintenance events regardless of status
- Approve events: `pending_review` → `reviewed`
- Create events with status `planned` (bypass review)
- Cancel events in `pending_review` status
- Update events in `pending_review` status

### sd_creators

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
  │pending_review│ ◄── sd_creators can modify/cancel here
  └──────┬──────┘
         │ Operator approves
         ▼
  ┌─────────────┐
  │  reviewed   │
  └──────┬──────┘
         │ Checker auto-transitions
         ▼
  ┌─────────────┐
  │   planned   │ ◄── sd_operators/sd_admins start here
  └──────┬──────┘
         │
         ▼
    [active → completed]
```

## Field Visibility

Some fields are only visible to authenticated users:

| Field | Visibility |
|-------|------------|
| `creator` | Authenticated only |
| `contact_email` | Authenticated only |
| `version` | Authenticated only |

Events with status `pending_review` or `reviewed` are hidden from unauthenticated users.

## Error Responses

| HTTP Code | Condition |
|-----------|-----------|
| `401 Unauthorized` | Missing or invalid JWT token |
| `403 Forbidden` | Insufficient role permissions |
| `403 Forbidden` | Attempting to modify event you don't own (sd_creators) |
| `403 Forbidden` | Attempting to modify event not in `pending_review` (sd_creators) |
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
- `groups` → matched against configured role environment variables
