# Configuration Guide: Maintenance Email Notifications

How to configure, deploy and troubleshoot maintenance email notifications.

For the design and its reasoning see [architecture.md](architecture.md).

---

## Quick start

Notifications are **disabled by default**. A minimal working configuration:

```bash
SD_NOTIFICATIONS_ENABLED=true
SD_SMTP_HOST=smtp.example.com
SD_SMTP_PORT=587
SD_SMTP_FROM=status-dashboard@example.com
SD_SMTP_TLS=true
SD_NOTIFICATIONS_SMOD_EMAIL=smod@example.com
```

Everything else has a default. The application refuses to start if the configuration is incomplete
or malformed, so a successful startup means the settings are valid.

---

## Reference

### Feature switch

| Variable | Default | Description |
|----------|---------|-------------|
| `SD_NOTIFICATIONS_ENABLED` | `false` | Master on/off switch. When `false`, no SMTP setting is required, no worker runs and no metrics listener opens. |

### SMTP transport

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `SD_SMTP_HOST` | **yes** | — | Mail server hostname. |
| `SD_SMTP_PORT` | **yes** | — | Mail server port, `1`–`65535`. Typically `587` (STARTTLS) or `25`. |
| `SD_SMTP_FROM` | **yes** | — | Sender address. Must be a valid address **and** permitted for the account, or the relay rejects every message. |
| `SD_SMTP_USER` | no | — | SMTP login. **Omit entirely** when the relay authorises by IP. |
| `SD_SMTP_PASSWORD` | no | — | SMTP password. Store in a secret, never in a ConfigMap. |
| `SD_SMTP_TLS` | no | `false` | `true` requires STARTTLS; `false` uses it opportunistically. |
| `SD_SMTP_TIMEOUT` | no | `30s` | Connect + send timeout (Go duration). |

Required fields apply only when the feature is enabled.

### Recipients

| Variable | Default | Description |
|----------|---------|-------------|
| `SD_NOTIFICATIONS_SMOD_EMAIL` | — | Fixed SMOD team review address. |
| `SD_NOTIFICATIONS_EMAILS_OPERATORS` | — | Review addresses for the Operator role, comma-separated. |
| `SD_NOTIFICATIONS_EMAILS_ADMINS` | — | Review addresses for the Admin role, comma-separated. |

At least one of the three must be set. Addresses are trimmed, lowercased and deduplicated.

The creator recipient is **not** configured here: it is the `contact_email` supplied when the
maintenance is created.

### Delivery tuning

| Variable | Default | Description |
|----------|---------|-------------|
| `SD_NOTIFICATIONS_LEASE_TIMEOUT` | `60s` | How long a claimed row stays owned by a pod. **Must exceed `SD_SMTP_TIMEOUT`.** |
| `SD_NOTIFICATIONS_MAX_ATTEMPTS` | `5` | Attempts before a row becomes terminally `failed`. |
| `SD_NOTIFICATIONS_BACKOFF_INTERVAL` | `5m` | Base retry delay. Doubles per attempt, capped at 2h, spread by ±20%. |

### Observability

| Variable | Default | Description |
|----------|---------|-------------|
| `SD_METRICS_PORT` | `9090` | Port for the private `/metrics` listener. Must differ from `SD_PORT`. |

---

## Deployment notes

### Which SMTP account to use

Use a **service account**, never a personal one. Three common cases:

| Relay setup | What to configure |
|-------------|-------------------|
| Authorises by IP or subnet | Omit `SD_SMTP_USER` and `SD_SMTP_PASSWORD` entirely |
| Requires authentication | Service account, e.g. `svc-status-dashboard`, password from a secret |
| Functional mailbox | The mailbox login |

Do not set `SD_SMTP_USER=""` explicitly. An empty value behaves like an unset one, but the redundant
key invites confusion later.

Agree `SD_SMTP_FROM` together with the account: relays verify that the sender is entitled to the
address and answer `550 sender address rejected` otherwise.

### Distribution lists over individual addresses

Prefer one distribution list per role:

```yaml
SD_NOTIFICATIONS_SMOD_EMAIL: smod@example.com
SD_NOTIFICATIONS_EMAILS_OPERATORS: sd-operators@example.com
SD_NOTIFICATIONS_EMAILS_ADMINS: sd-admins@example.com
```

Membership then lives in the mail system and changes without a redeploy, instead of requiring a
config change and a pod restart for every staffing update.

### Metrics port

`/metrics` is served on its own listener so that queue depth and failure counts are not reachable
from the public API port. Keep it internal to the cluster:

```yaml
ports:
  - name: http
    containerPort: 8000
  - name: metrics
    containerPort: 9090   # scraped by Prometheus, not exposed through Ingress
```

### Secrets

`SD_SMTP_PASSWORD` belongs in a Kubernetes Secret. It is masked in application logs, but a ConfigMap
or a committed `.env` would expose it.

---

## Validation at startup

When the feature is enabled the application refuses to start unless:

- `SD_SMTP_HOST`, `SD_SMTP_PORT` and `SD_SMTP_FROM` are set;
- `SD_SMTP_PORT` is a number in `1`–`65535`;
- `SD_SMTP_FROM` parses as an email address;
- at least one review address is configured, and **every** configured review address parses;
- `SD_SMTP_TIMEOUT`, `SD_NOTIFICATIONS_LEASE_TIMEOUT` and `SD_NOTIFICATIONS_BACKOFF_INTERVAL` parse
  as Go durations;
- `SD_NOTIFICATIONS_LEASE_TIMEOUT` is greater than `SD_SMTP_TIMEOUT`;
- `SD_NOTIFICATIONS_MAX_ATTEMPTS` is a positive integer;
- `SD_METRICS_PORT` is in `1024`–`65535` and differs from `SD_PORT`.

Failing here is deliberate: a typo in a review address would otherwise stay invisible until the
first maintenance, then break every review notification.

---

## Verifying the setup

### Admin API

All three endpoints require the `admin` role.

```bash
curl -H "Authorization: Bearer $TOKEN" https://<host>/v2/notifications/stats
```

```json
{
  "pending": 0, "processing": 0, "sent": 42, "failed": 0,
  "stale_processing": 0, "retry_backlog": 0,
  "oldest_pending_age_seconds": 0
}
```

| Endpoint | Purpose |
|----------|---------|
| `GET /v2/notifications/stats` | Queue snapshot |
| `GET /v2/notifications/failed` | Most recent terminally failed rows |
| `POST /v2/notifications/redrive` | Reset failed rows to `pending` and wake the worker |

Re-drive everything, or selected rows:

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
     -d '{}'              https://<host>/v2/notifications/redrive
curl -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
     -d '{"ids":[12,13]}' https://<host>/v2/notifications/redrive
```

### Metrics worth alerting on

| Series | Signal |
|--------|--------|
| `notification_outbox_oldest_pending_age_seconds` | Rising steadily → delivery is stuck |
| `notification_outbox_failed` | Growing → recipients or relay misconfigured |
| `notification_outbox_stale_processing` | Non-zero → pods crashing mid-send |
| `notification_collector_errors_total` | Increasing → the gauges below it are unreliable |

---

## Local development

Use [Mailpit](https://mailpit.axllent.org/) as a catcher — it accepts everything and offers no
authentication:

```bash
podman run -d --rm --name status-dashboard-mailpit \
  -p 127.0.0.1:1025:1025 -p 127.0.0.1:8025:8025 \
  docker.io/axllent/mailpit:latest --verbose
```

```bash
SD_NOTIFICATIONS_ENABLED=true
SD_SMTP_HOST=127.0.0.1
SD_SMTP_PORT=1025
SD_SMTP_FROM=status-dashboard@local.test
SD_SMTP_TLS=false
# No SD_SMTP_USER — Mailpit offers no AUTH
SD_NOTIFICATIONS_SMOD_EMAIL=smod@local.test
SD_NOTIFICATIONS_EMAILS_OPERATORS=operators@local.test
SD_NOTIFICATIONS_EMAILS_ADMINS=admins@local.test
```

Inbox at `http://127.0.0.1:8025`; `--verbose` logs every SMTP session, which distinguishes "the app
never connected" from "the message was rejected".

---

## Troubleshooting

### Nothing arrives, and the outbox is empty

The change never reached the publisher. Confirm the event is a **maintenance** (other incident types
never notify) and that `SD_NOTIFICATIONS_ENABLED=true`.

### Rows stay `pending` with a rising `attempts`

Delivery is failing and being retried. Read `last_error`:

```sql
SELECT id, recipient, status, attempts, left(last_error, 80)
FROM notification_outbox ORDER BY id DESC LIMIT 20;
```

| `last_error` contains | Cause | Fix |
|-----------------------|-------|-----|
| `dial failed` / `connection refused` | Host, port or firewall | Check `SD_SMTP_HOST`/`SD_SMTP_PORT` and egress rules |
| `server does not support SMTP AUTH` | Credentials sent to a relay without AUTH | Unset `SD_SMTP_USER` |
| `550 sender address rejected` | `SD_SMTP_FROM` not allowed for the account | Align the sender with the account |
| `context deadline exceeded` | Relay too slow | Raise `SD_SMTP_TIMEOUT`, then `SD_NOTIFICATIONS_LEASE_TIMEOUT` above it |

### Rows go straight to `failed` on the first attempt

A permanent `5xx` rejection — usually an unknown recipient. Check the address, then `redrive` after
fixing it.

### Retries seem slow

By design: `5m → 10m → 20m → 40m → 80m`, capped at 2h and jittered by ±20%. `redrive` bypasses the
wait, but only for rows already in `failed`.

### Application will not start

The message names the offending variable, for example:

```
SD_NOTIFICATIONS_EMAILS_OPERATORS contains an invalid address "ops at example.com"
SD_NOTIFICATIONS_LEASE_TIMEOUT (30s) must be greater than SD_SMTP_TIMEOUT (30s)
SD_METRICS_PORT must differ from SD_PORT
```

### Metrics are unreachable

They are on `SD_METRICS_PORT` (default `9090`), not on the API port, and only when notifications are
enabled.
