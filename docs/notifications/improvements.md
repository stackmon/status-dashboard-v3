# Notifications — Development Roadmap

Open improvements for the maintenance email notification feature. Nothing listed here is
implemented; sections are ordered by priority, and each records the reasoning so the
decision does not have to be rediscovered later.

Related: [architecture.md](architecture.md), [configuration.md](configuration.md).

---

## 1. Frontend: prefill `contact_email`

**Priority: high (the backend fallback is unreachable without it)**

Lives in the separate [StatusDashboard-V3](https://github.com/stackmon/StatusDashboard-V3)
repository.

### Problem

The backend accepts a maintenance without `contact_email` and falls back to the `email`
claim of the creator's token. The create form still marks the field as required and blocks
submission while it is empty:

```ts
if (type === EventType.Maintenance && !value) {
  setValContactEmail("Contact Email is required for maintenance.");
}
```

So the fallback can never trigger through the UI. Users keep typing an address by hand,
with the same risk of a typo that cannot be corrected afterwards.

### Recommendation

Prefill the field from the signed-in user instead of relaxing the requirement.
`NewForm.tsx` already calls `useAuth()`, and `profile.email` is present in the ID token,
so no extra request is needed:

```ts
const userEmail = useAuth().user?.profile.email;
useEffect(() => {
  if (userEmail && !contactEmail) {
    _setContactEmail(userEmail);
  }
}, [userEmail]);
```

Keep the field required and editable. Making it optional would match the backend fallback,
but the user would no longer see where the notifications are going, and "send to the team
mailbox instead of me" is a routine request that must stay one edit away.

### Note

The `email` claim reaches the **access token** only through a mapper on the client
(`Add to access token`); the frontend reads it from the ID token, where it is present by
default. The two paths are independent, so the frontend prefill works even where the
backend fallback does not — and vice versa.

---

## 2. Review audience via distribution lists

**Priority: medium (operational, no code change)**

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

## 3. Editable `contact_email`

**Priority: medium**

### Problem

`contact_email` is set once at creation and cannot be changed afterwards — the field does
not exist in `PatchIncidentData`. Only the syntax is validated, so `user@gmial.com` is
accepted and every notification for that maintenance is delivered to a stranger, or
nowhere, for the entire lifecycle of the event. The only repair is a manual `UPDATE` in
the database.

### Proposal

Add the field to `PatchIncidentData` and apply the same checks as on creation: address
syntax plus the `SD_NOTIFICATIONS_ALLOWED_DOMAINS` allow-list.

Restrict the change to the roles that may already patch the maintenance. Note that the
creator's own permission is derived from `created_by`, so a creator editing their own
event keeps working without extra rules.

### Trade-off

Changing the address mid-flight means rows already queued keep the old recipient, since
the payload is a snapshot. That is acceptable: the alternative — rewriting pending rows —
would blur the audit trail for no practical gain.

---

## 4. Ops API: `503` when the feature is disabled

**Priority: low**

With `SD_NOTIFICATIONS_ENABLED=false` the three admin endpoints still respond `200` with
zeroed statistics, so an operator cannot tell "nothing to send" from "feature switched
off". Both look like a perfectly healthy empty queue.

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
session for every recipient, so a queue of 50 messages performs 50 TCP and TLS handshakes.
Corporate relays often rate-limit connections per source address and may temporarily block
a sender that reconnects too eagerly. `go-mail` supports `DialWithContext` followed by
several `Send` calls on one session.

**Single-threaded sending.** The worker sends one message at a time, so throughput is
capped at one email per round-trip. A small bounded pool (3–5 senders) would remove the
ceiling. This became straightforward only after claiming moved to one row per lease — with
batch claiming, concurrency would have widened the duplicate window described in
[architecture.md](architecture.md) §5.

Both are worth doing only if the queue is observed to lag: at the current volume
(~41 maintenances in 2 months) neither is measurable.
