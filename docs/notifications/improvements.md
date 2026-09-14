# Notifications — Development Roadmap

Improvement proposals for the maintenance email notification feature. Items are recorded
with their reasoning so the decisions do not have to be rediscovered later. Section 1 is
done; everything after it is open.

Related: [architecture.md](architecture.md), [configuration.md](configuration.md).

---

## 1. Recipient addressing — **implemented**

Both the allow-list and the token-derived creator address are in place:

- `SD_NOTIFICATIONS_ALLOWED_DOMAINS` restricts the domains accepted in `contact_email`,
  rejecting others with `400` and naming the permitted ones. An empty value keeps the
  previous behaviour.
- `SD_NOTIFICATIONS_EXCLUDED_EMAILS` drops specific addresses from every recipient list,
  so an exclusion cannot be bypassed via `contact_email`.
- `contact_email` is now optional. When omitted, the verified `email` claim of the
  creator's token is used instead.

Deliberately **not** done: a separate `incident.creator_email` column. One column holds
the resolved address, since the two sources are alternatives rather than two independent
recipients. Tokens without an `email` claim (HMAC, service-to-service) simply leave the
address empty, which narrows the audience but never blocks the request.

Open follow-up: `contact_email` still cannot be changed after creation, so a wrong
address stays wrong for the lifetime of the maintenance. Allowing it in
`PatchIncidentData` would fix that.

---

## 2. Config hardening beyond SMTP

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

Extend the existing `TestLoadConf_IgnoresBareEnvNames` (it already covers `USER` and
`PASSWORD`) with `HOSTNAME` and `PORT`, asserting the defaults are used.

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

## 6. Delivery throughput

**Priority: low**

Two related inefficiencies, neither affecting correctness.

**A new connection per message.** `DialAndSendWithContext` opens and closes an SMTP
session for every recipient, so a queue of 50 messages performs 50 TCP and TLS
handshakes. Corporate relays often rate-limit connections per source address and may
temporarily block a sender that reconnects too eagerly. `go-mail` supports
`DialWithContext` followed by several `Send` calls on one session.

**Single-threaded sending.** The worker sends one message at a time, so throughput is
capped at one email per round-trip. A small bounded pool (3–5 senders) would remove the
ceiling. This became straightforward only after claiming moved to one row per lease —
with batch claiming, concurrency would have widened the duplicate window described in
[architecture.md](architecture.md) §5.

Both are worth doing only if the queue is observed to lag: at the current volume
(~41 maintenances in 2 months) neither is measurable.

---

## Suggested order

| Order | Section | Type | Rationale |
|---|---|---|---|
| 1 | §2 Config hardening | Code | Latent production bug in any container |
| 2 | §3 Distribution lists | Config | No code, immediate operational relief |
| 3 | §4 Ops API gaps | Code | Diagnosability |
| 4 | Editable `contact_email` (§1 follow-up) | Code | A wrong address is currently permanent |
| 5 | §5 Implicit TLS | Code | Only when a relay demands it |
| 6 | §6 Delivery throughput | Code | Only if the queue is seen to lag |
