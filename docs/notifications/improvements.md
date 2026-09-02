# Notifications — Development Roadmap

Improvement proposals for the maintenance email notification feature. Nothing here is
implemented; this document records the reasoning so the decisions do not have to be
rediscovered later.

Related: [architecture.md](architecture.md), [final_scope_email.md](final_scope_email.md),
[plan.md](plan.md).

---

## 1. Recipient allow-list for `contact_email`

**Priority: high (security)**

### Problem

`contact_email` comes straight from the create request and is only checked for syntax:

```go
if _, err := mail.ParseAddress(incData.ContactEmail); err != nil {
    return apiErrors.ErrMaintenanceContactEmailInvalid
}
```

Any user holding the `creator` role can therefore make the dashboard send mail to an
arbitrary external address. Two consequences:

- **Abuse.** Messages leave from a trusted corporate domain with corporate branding,
  which is a ready-made phishing vector. The outbox even retries them for us.
- **Typos.** `user@gmial.com` is syntactically valid, so the mail leaves the building
  and lands with a stranger. Maintenance titles and schedules are not public data.

### Proposal

Add a domain allow-list applied at request validation time:

```
SD_NOTIFICATIONS_ALLOWED_DOMAINS=company.com,t-systems.com
```

- Empty value keeps current behaviour, so existing installations are unaffected.
- Compare the domain part case-insensitively, after the existing `mail.ParseAddress`.
- Reject with `400` and a message naming the allowed domains — the user must be able to
  fix the input without reading the deployment manifest.

Validate the variable itself at startup (each entry a plausible domain), consistent with
how the review-audience lists are already checked in `validateReviewAudience`.

### Trade-offs

Installations that legitimately notify external partners must list those domains
explicitly. That is the intended cost: the allow-list turns an implicit capability into
an explicit, auditable decision.

---

## 2. Trusted `creator_email` from the JWT

**Priority: medium**

### Problem

The creator's address is whatever was typed into the form. Nothing ties a notification to
the identity that actually created the maintenance:

- The person who created the window may never be notified about it.
- `created_by` (the Keycloak `preferred_username`) and `contact_email` can point at
  unrelated people, and nothing detects the mismatch.
- A typo silently redirects every notification for that maintenance.

The OIDC scope already requests `email` ([../../internal/api/auth/auth.go](../../internal/api/auth/auth.go)),
but the middleware only extracts `preferred_username` and `groups`, so the verified
address is discarded.

### Proposal

Treat the token as the source of truth and the form field as an optional addition:

| Field | Source | Role |
|---|---|---|
| `creator_email` | `email` claim | Trusted, verified by Keycloak, always notified |
| `contact_email` | request body, optional | Additional address, subject to the allow-list |

Steps:

1. Extract the `email` claim in `setUserIDFromClaims` alongside `preferred_username`.
2. Add an `incident.creator_email` column; populate it on create.
3. Pass both addresses into `notification.Change`.

`Resolver.Recipients` already normalizes and deduplicates, so when the two fields match,
only one message is produced. No resolver changes are required.

### Trade-offs

Local HMAC tokens (dev, service-to-service) carry no `email` claim, so `creator_email`
must stay nullable and the feature must degrade to `contact_email` alone. Keep
`contact_email` optional rather than removing it: "notify the team mailbox, not me" is a
legitimate and common request.

---

## 3. Review audience via distribution lists

**Priority: low (operational, no code change)**

### Problem

`SD_NOTIFICATIONS_EMAILS_OPERATORS` and `SD_NOTIFICATIONS_EMAILS_ADMINS` hold individual
addresses, so every staffing change requires a config change and a pod restart. The same
membership information already exists in Keycloak groups, duplicated by hand.

### Proposal

Point each variable at one distribution list instead of a list of people:

```yaml
SD_NOTIFICATIONS_SMOD_EMAIL: smod@company.com
SD_NOTIFICATIONS_EMAILS_OPERATORS: sd-operators@company.com
SD_NOTIFICATIONS_EMAILS_ADMINS: sd-admins@company.com
```

Membership then lives in the mail system, owned by the people who already own the groups.
The application keeps three stable addresses that change once every few years.

### Alternative considered: Keycloak Admin API

Resolving group members at send time looks natural — the groups are already there — but it
requires a service account with user-read permissions, pagination handling, a cache with
invalidation, and a defined behaviour when Keycloak is unreachable mid-delivery. That
inserts a distributed dependency into the mail path to buy what a distribution list
provides for free. Not recommended.

---

## 4. Operations API gaps

**Priority: low**

### Queue is not fully visible

`GET /v2/notifications/failed` only lists rows in the `failed` state. Rows stuck in
`pending` with a growing `attempts` count — the common symptom of a misconfigured relay —
are invisible over HTTP and require direct SQL access.

**Proposal:** accept `?status=` and `?limit=` on the same endpoint, defaulting to `failed`
to preserve current behaviour.

### Disabled feature is indistinguishable from an empty queue

With `SD_NOTIFICATIONS_ENABLED=false` the three admin endpoints still respond `200` with
zeroed statistics, so an operator cannot tell "nothing to send" from "feature switched
off".

**Proposal:** return `503` with an explicit body when the feature is disabled.

---

## 5. SMTP transport: implicit TLS (port 465)

**Priority: low, becomes blocking if a relay requires SMTPS**

`SD_SMTP_TLS=true` maps to `mail.TLSMandatory`, which is *mandatory STARTTLS* on a plain
port (587 or 25). Relays that expect TLS negotiated at connection time (SMTPS, port 465)
are not supported — the handshake never happens and the connection fails.

**Proposal:** add `SD_SMTP_TLS_MODE` with values `starttls` (default), `implicit`
(`mail.WithSSL()`), and `none`, deprecating the boolean. Keep the boolean working for one
release to avoid breaking deployments.

---

## 6. Config hardening beyond SMTP

**Priority: medium**

`SMTPConfig` and `Notifications.Enabled` no longer carry `envconfig` tags, because
envconfig falls back to the bare tag name when the prefixed variable is unset — a tag of
`"USER"` silently inherited the shell's `$USER` and enabled SMTP AUTH against a server
that offers none.

The same trap remains in `Config`:

| Field | Tag | Risk |
|---|---|---|
| `Hostname` | `HOSTNAME` | **Always set in containers** — without `SD_HOSTNAME` the app adopts the pod name |
| `Port` | `PORT` | Set by several PaaS platforms (Cloud Run injects `PORT=8080`) |
| `DB`, `Cache` | `DB`, `CACHE` | Plausible in some shells |

**Proposal:** drop the tags on these single-word fields as well. `mergeConfigs` already
falls back to the field name via `envKeyPart`, and the field names produce identical keys
(`SD_HOSTNAME`, `SD_PORT`), so the change is behaviour-preserving except for removing the
unintended fallback.

Add a regression test in the shape of `TestLoadConf_IgnoresBareEnvNames`, which sets
`HOSTNAME`/`PORT` in the environment and asserts the defaults are used.

---

## Suggested order

| # | Item | Type | Rationale |
|---|---|---|---|
| 1 | Allow-list for `contact_email` | Code | Closes an abuse vector |
| 2 | Config hardening (`Hostname`, `Port`) | Code | Latent production bug in any container |
| 3 | `creator_email` from JWT | Code + migration | Correctness of addressing |
| 4 | Distribution lists | Config | No code, immediate operational relief |
| 5 | Ops API gaps | Code | Diagnosability |
| 6 | Implicit TLS | Code | Only when a relay demands it |
