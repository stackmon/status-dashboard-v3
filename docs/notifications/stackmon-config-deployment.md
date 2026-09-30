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
`secret/data/statusdashboard/sd3-test`. The SMTP credentials live in a **separate** secret,
`secret/statusdashboard/smg` (KV v2 API path `secret/data/statusdashboard/smg`), so the template
gets a second `with secret` block.

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

The relay is the OTC Secure Mail Gateway
([docs](https://docs.otc.t-systems.com/secure-mail-gateway/umn/)). Host and port are fixed;
the credentials and the sender address are not in this repository and must not be invented.

| Value | Where | Source |
|---|---|---|
| `otc-de-out.mms.t-systems-service.com` | template literal | Gateway documentation |
| `25` (the only port open for mail acceptance) | template literal | Gateway documentation |
| SMTP login | `secret/statusdashboard/smg`, key `smtpuser` | Cloud Handling Support (already issued) |
| SMTP password | `secret/statusdashboard/smg`, key `smtppassword` | Cloud Handling Support (already issued) |
| Sender address `SD_SMTP_FROM` | template literal (not a secret) | See below |
| SMOD test recipient | `sd3-test`, key `notificationssmodemail` | Team decision |
| Operator test recipients | `sd3-test`, key `notificationsemailsoperators` | Team decision |
| Admin test recipients | `sd3-test`, key `notificationsemailsadmins` | Team decision |

> The exact key names inside `smg` must be confirmed by whoever owns the secret; adjust the
> template in Change 1 if they differ from `smtpuser` / `smtppassword`.

**Sender address.** `SD_SMTP_FROM` is the `MAIL FROM` / `From:` address, not a credential. The
gateway only accepts addresses permitted for the account (otherwise `550 sender address rejected`).
Use the SMTP login if it is an email address; otherwise obtain the allowed sender from Cloud
Handling Support. Do not invent one.

**Vault policy.** The Vault role `sd3` (`auth/kubernetes_otcinfra2`) must be allowed to `read`
`secret/data/statusdashboard/smg`. Without it the Vault agent init container fails with
`permission denied` and the pod does not start. This is changed in Vault, not in git.

Authentication is **mandatory** on this gateway — there is no IP-based alternative.

Also required: the image tag of a Status Dashboard build that contains the notification feature
(`quay.io/stackmon/status-dashboard-v3:sha-<commit>`). The tag currently referenced,
`sha-17d25aa`, predates the feature.

---

## Change 1 — `overlays/test/vault-agent.hcl`

Add to the `template` block. The non-secret SMTP settings and the notification recipients go inside
the existing `{{ with secret "secret/data/statusdashboard/sd3-test" }}` section:

```hcl
export SD_NOTIFICATIONS_ENABLED=true
export SD_SMTP_HOST=otc-de-out.mms.t-systems-service.com
export SD_SMTP_PORT=25
export SD_SMTP_FROM=<allowed-sender-address>
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

Then add a **second block** after the existing `{{- end }}`, for the credentials:

```hcl
{{ with secret "secret/data/statusdashboard/smg" -}}
export SD_SMTP_USER="{{ .Data.data.smtpuser }}"
export SD_SMTP_PASSWORD="{{ .Data.data.smtppassword }}"
{{- end }}
```

`SD_SMTP_TLS=true` makes STARTTLS mandatory, so the credentials are never sent over an
unencrypted connection. The gateway requires authentication, so both credential lines are
always present here; against a relay that authorises by IP they would be omitted entirely
rather than set to empty strings, since a non-empty user makes the sender negotiate AUTH.

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

- Vault key and policy changes (including read access to `secret/statusdashboard/smg`) —
  performed by whoever holds Vault access, not through git.
- `overlays/prod` — production stays on the current build until the test run succeeds.
- Ingress changes.

---

## Database migration — required before rollout

Migrations are **not** applied by the application, and the image contains neither the
`db/migrations` directory nor the `migrate` CLI. Migration `000008` (which creates
`notification_outbox`) must be applied to the `sd3-test` database out of band, by whoever
normally runs migrations for this environment.

The application refuses to start when notifications are enabled and the table is absent:

```
notification_outbox table is missing: apply the pending database migrations
```

This is deliberate. Without the check the pod would come up healthy and only fail on the
first maintenance change, turning a deployment mistake into a user-visible error.

Order therefore matters: **apply the migration first, then roll out the new image.**

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
kubectl -n sd3-test exec deploy/sd3-api -- nc -zv otc-de-out.mms.t-systems-service.com 25
kubectl -n sd3-test port-forward deploy/sd3-api 9090:9090
curl -s http://localhost:9090/metrics | grep notification_outbox
```

Queue snapshot over the API (requires an admin token):

```bash
curl -H "Authorization: Bearer $TOKEN" \
     https://api.test.status.otc-service.com/v2/notifications/stats
```

A healthy idle queue reports `pending: 0` and `failed: 0`. Rows stuck in `pending` with
rising `attempts` mean delivery is failing; list them and read the SMTP error:

```bash
curl -H "Authorization: Bearer $TOKEN" \
     "https://api.test.status.otc-service.com/v2/notifications/failed?status=pending"
```

---

## Rollback

Revert the image tag in `overlays/test/kustomization.yaml`, or set
`export SD_NOTIFICATIONS_ENABLED=false` in the template. With the feature off the application starts
without any SMTP setting, no worker runs and no mail is sent; queued rows stay in the database
untouched.
