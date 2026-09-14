# Agent Instructions: Enable Notifications in `stackmon-config` (test)

Task description for an agent working in the **`stackmon-config`** repository. Prepared from the
Status Dashboard side; the agent editing `stackmon-config` has no context about this service, so
everything it needs is stated explicitly below.

---

## Goal

Deploy the Status Dashboard build that includes maintenance email notifications to the **sd3-test**
environment, and wire it to the corporate SMTP relay.

## Repository and branch

- Repository: `opentelekomcloud-infra/stackmon-config`
- Branch: `sd-test-branch`
- Scope: `kustomize/sd3/api/` only. Do **not** touch `overlays/prod`, `kustomize/sd3-ch` or
  `kustomize/sdb`.

## How this deployment works

Environment variables are **not** declared in the Deployment manifest. A Vault agent init container
renders a shell file and the application sources it at startup:

```yaml
args: ['source /secrets/sd3-api-env && "/usr/src/app/app"']
```

The file is produced by the template in `overlays/test/vault-agent.hcl` from the Vault secret
`secret/data/statusdashboard/sd3-test`.

Consequence: adding a setting requires **two** steps — a key in Vault, and an `export` line in the
template. A key without a template line has no effect; a template line without a key renders empty.

## Relevant files

| File | Purpose |
|---|---|
| `kustomize/sd3/api/overlays/test/vault-agent.hcl` | Renders env vars from Vault |
| `kustomize/sd3/api/overlays/test/kustomization.yaml` | Image tag, namespace `sd3-test`, ingress |
| `kustomize/sd3/api/base/deployment.yaml` | Container ports, resources, vault init container |

---

## Prerequisites (must be supplied before starting)

These values come from the mail team and are not in this repository. Do not invent them.

| Value | Vault key |
|---|---|
| Relay hostname | `smtphost` |
| Relay port (`587` expected) | `smtpport` |
| Allowed sender address | `smtpfrom` |
| SMTP login, only if the relay requires AUTH | `smtpuser` |
| SMTP password, only if the relay requires AUTH | `smtppassword` |
| SMOD test recipient | `notificationssmodemail` |
| Operator test recipients | `notificationsemailsoperators` |
| Admin test recipients | `notificationsemailsadmins` |

Also required: the image tag of a Status Dashboard build that contains the notification feature
(`quay.io/stackmon/status-dashboard-v3:sha-<commit>`). The tag currently referenced,
`sha-17d25aa`, predates the feature.

**Stop and ask** if the relay port is `465`: that is implicit TLS, which the application does not
support yet. Deployment must wait for a code change.

---

## Change 1 — `overlays/test/vault-agent.hcl`

Add to the `template` block, inside the existing `{{ with secret ... }}` section:

```hcl
export SD_NOTIFICATIONS_ENABLED=true
export SD_SMTP_HOST={{ .Data.data.smtphost }}
export SD_SMTP_PORT={{ .Data.data.smtpport }}
export SD_SMTP_FROM={{ .Data.data.smtpfrom }}
export SD_SMTP_TLS=true
export SD_SMTP_TIMEOUT=30s
export SD_NOTIFICATIONS_LEASE_TIMEOUT=60s
export SD_NOTIFICATIONS_MAX_ATTEMPTS=5
export SD_NOTIFICATIONS_BACKOFF_INTERVAL=5m
export SD_NOTIFICATIONS_SMOD_EMAIL="{{ .Data.data.notificationssmodemail }}"
export SD_NOTIFICATIONS_EMAILS_OPERATORS="{{ .Data.data.notificationsemailsoperators }}"
export SD_NOTIFICATIONS_EMAILS_ADMINS="{{ .Data.data.notificationsemailsadmins }}"
export SD_METRICS_PORT=9090
```

Add the credential lines **only if** the relay requires authentication:

```hcl
export SD_SMTP_USER="{{ .Data.data.smtpuser }}"
export SD_SMTP_PASSWORD="{{ .Data.data.smtppassword }}"
```

If the relay authorises by IP, omit both lines entirely. Do not add them with empty values: the
application treats a non-empty user as "this server wants AUTH" and aborts the session against a
relay that offers none.

### Quoting rule — do not skip

The rendered file is executed with `source`, so any value containing a space, comma or shell
metacharacter must be quoted. Unquoted:

```bash
export SD_NOTIFICATIONS_EMAILS_ADMINS=a@x.com, b@x.com
# shell parses: export "a@x.com," then tries to run "b@x.com"
```

All comma-separated lists and the password must use `"{{ ... }}"`.

**Pre-existing issue worth fixing in the same PR:** `SD_RBAC_GROUPS_ADMINS`,
`SD_RBAC_GROUPS_OPERATORS` and `SD_RBAC_GROUPS_CREATORS` are currently unquoted. If the Vault value
holds a list such as `sd-admins, status-dashboard`, it is silently truncated. Quote them.

### Dead variable

`SD_AUTH_GROUP` is exported by the template but no longer read by the application. Safe to remove;
mention it in the PR description rather than removing it silently.

---

## Change 2 — `base/deployment.yaml`

Declare the metrics port next to the existing one:

```yaml
        ports:
        - containerPort: 8000
        - containerPort: 9090
          name: metrics
```

Do **not** add it to the Ingress. The endpoint exposes queue depth and failure counts and is meant
to stay reachable only inside the cluster.

---

## Change 3 — `overlays/test/kustomization.yaml`

Update the image tag:

```yaml
images:
  - name: sd3-api
    newName: quay.io/stackmon/status-dashboard-v3
    newTag: sha-<commit-with-notifications>
```

Leave namespace, ingress host and TLS secret unchanged.

---

## Out of scope

- Vault key creation — performed by whoever holds Vault access, not through git.
- `overlays/prod` — production stays on the current build until the test run succeeds.
- Ingress changes.
- Database migrations — applied by the application at startup.

---

## Verification after rollout

```bash
kubectl -n sd3-test get pods
kubectl -n sd3-test logs deploy/sd3-api | head -30
```

The application refuses to start on invalid notification settings, so a running pod already proves
the configuration parsed. Typical startup failures name the offending variable:

```
notifications enabled: SD_SMTP_HOST, SD_SMTP_PORT and SD_SMTP_FROM are required
SD_NOTIFICATIONS_EMAILS_OPERATORS contains an invalid address "ops at example.com"
SD_NOTIFICATIONS_LEASE_TIMEOUT (30s) must be greater than SD_SMTP_TIMEOUT (30s)
```

Check relay connectivity and the queue:

```bash
kubectl -n sd3-test exec deploy/sd3-api -- nc -zv <relay-host> 587
kubectl -n sd3-test port-forward deploy/sd3-api 9090:9090
curl -s http://localhost:9090/metrics | grep notification_outbox
```

Queue snapshot over the API (requires an admin token):

```bash
curl -H "Authorization: Bearer $TOKEN" \
     https://api.test.status.otc-service.com/v2/notifications/stats
```

A healthy idle queue reports `pending: 0` and `failed: 0`. Rows stuck in `pending` with rising
`attempts` mean delivery is failing — `GET /v2/notifications/failed` and the pod logs carry the SMTP
error.

---

## Rollback

Revert the image tag in `overlays/test/kustomization.yaml`, or set
`export SD_NOTIFICATIONS_ENABLED=false` in the template. With the feature off the application starts
without any SMTP setting, no worker runs and no mail is sent; queued rows stay in the database
untouched.
