---
name: devops-platform
description: Guides infrastructure as code, environment management, secrets handling, monitoring and alerting, and incident response. Use when writing or reviewing Terraform or similar IaC, structuring environments (dev/staging/prod), storing or rotating secrets, defining alert thresholds, or running an incident from detection through postmortem. Use before making a risky infrastructure change like a DNS or configuration update.
category: Engineering
---

# DevOps and Platform Engineering

## Overview

DevOps and platform engineering is the discipline of making infrastructure reproducible, environments consistent, secrets safe, problems visible before users report them, and incidents handled with a repeatable process. This skill covers infrastructure as code, environment structure, secrets management, monitoring/alerting design, and incident response.

## When to Use

- Writing or reviewing infrastructure as code (Terraform, Pulumi, CloudFormation, etc.)
- Structuring or reconciling dev/staging/production environments
- Storing, injecting, or rotating secrets and credentials
- Defining what gets monitored and what triggers an alert
- Running or reviewing an incident response, including postmortem
- Before any change that is hard to reverse (DNS, IAM policy, production config)

## Workflow

### 1. Treat infrastructure as code, reviewed like application code

- Every environment's infrastructure is defined in version-controlled IaC, not created by hand in a console — a manual change is invisible to the next person and impossible to diff.
- IaC changes go through the same PR review as application code (see sdlc-practices): reviewer checks blast radius (what else does this affect), not just syntax.
- Run `plan`/`diff` output as part of the PR, not just at apply time — reviewers should see exactly what will change before it happens.
- Modules/stacks should be environment-parameterized, not copy-pasted per environment — copies drift silently; parameters keep environments comparably structured.

```hcl
# Good: parameterized module, one source of truth per environment
module "api_service" {
  source      = "../../modules/service"
  environment = var.environment          # "staging" | "production"
  instance_count = var.environment == "production" ? 3 : 1
  db_instance_class = var.environment == "production" ? "db.r6g.large" : "db.t4g.medium"
}
```

### 2. Keep environments consistent, with intentional differences only

- Dev, staging, and production should run the same infrastructure shape (same service topology, same database engine/version) — differences should be intentional (scale, data volume) and documented, not accidental drift.
- Staging should be reachable by the same deploy pipeline as production, not a separately hand-maintained environment — otherwise "works in staging" stops meaning anything.
- Seed data in non-production environments should resemble production data shape (volume, distribution) closely enough to catch performance issues before they reach production, without containing real user data.
- Tear down ephemeral environments (preview/PR environments) automatically on merge/close — orphaned environments cost money and become undocumented snowflakes.

### 3. Handle secrets so they are never in version control or logs

- Secrets (API keys, database credentials, signing keys) live in a secrets manager (Vault, AWS Secrets Manager, cloud KMS-backed store), injected at runtime — never committed to the repo, never baked into a container image.
- `.env` files with real secrets are gitignored by default; only `.env.example` with placeholder values is committed.
- Rotate secrets on a schedule and immediately on suspected exposure — a secret that's "probably fine" after a leak is a liability until proven rotated.
- Scope credentials to least privilege per service; a single shared admin credential used everywhere means one leak compromises everything.
- Scan for accidentally committed secrets in CI (pre-merge, not just periodically) — catching a leaked key in seconds is categorically cheaper than catching it after it's been scraped by a bot.

```bash
# Before committing, verify nothing sensitive is staged
git diff --cached | grep -iE "api[_-]?key|secret|password|token" 
```

### 4. Design monitoring and alerting around user impact

- Instrument the metrics that reflect user-visible health first: error rate, latency (p50/p95/p99), and saturation (CPU, memory, connection pool, queue depth) — the four golden signals.
- Alert on symptoms that require action, not every anomaly — an alert that fires and is routinely ignored trains the on-call to ignore alerts, which is worse than no alert.
- Every alert should have a runbook link: what does this alert mean, what's the likely cause, what's the first diagnostic step. An alert with no runbook forces the responder to re-derive context every single time.
- Set alert thresholds from historical baseline plus margin, not arbitrary round numbers — and revisit thresholds after any significant traffic or architecture change.
- Separate paging alerts (wake someone up, needs action now) from informational alerts (dashboard/ticket, review during business hours) — conflating them causes alert fatigue on the pager.

| Signal | Example metric | Alert style |
|---|---|---|
| Errors | 5xx rate, failed job rate | Page above threshold sustained for N minutes |
| Latency | p95/p99 request duration | Page on sustained regression vs. baseline |
| Saturation | CPU, memory, connection pool usage, queue depth | Page approaching capacity, not only at 100% |
| Traffic | Request rate, anomaly vs. expected pattern | Informational unless correlated with errors/latency |

### 5. Run incidents with a repeatable process, then write it down

Incident lifecycle:
1. **Detect** — alert fires or is reported; assign an incident commander immediately, even for a suspected minor issue.
2. **Mitigate** — stop the bleeding first (rollback, feature flag off, scale up, block traffic) before root-causing; restoring service comes before understanding why.
3. **Communicate** — status updates on a fixed cadence to stakeholders/users, even when the update is "still investigating."
4. **Resolve** — confirm the mitigation actually restored the metric that indicated the problem, not just that the obvious symptom stopped.
5. **Postmortem** — blameless write-up: timeline, impact, root cause, what worked, what didn't, and specific follow-up actions with owners and dates.

Postmortem quality bar: a postmortem that concludes "we'll be more careful" has failed — every action item should be a concrete, ownable change (a new alert, a new test, a new gate), not a resolution to try harder.

### 6. Prepare an undo path before any risky change

Before a DNS change, IAM policy change, load balancer config change, or similar hard-to-reverse infrastructure action:
- Record the exact current values (export the current DNS records, dump the current policy JSON, snapshot the current config) before changing anything.
- Confirm who/what depends on the current state, if known, before changing it.
- Have the restore command ready and tested in a non-production context where feasible, not written for the first time mid-incident.

```bash
# Before changing DNS, save current state for rollback
dig +short ANY example.com > dns-backup-$(date +%F).txt
aws route53 get-hosted-zone --id ZXXXXXXXX > route53-backup-$(date +%F).json
```

## Checklist

- [ ] Infrastructure changes exist as version-controlled IaC with a `plan`/`diff` reviewed before apply
- [ ] Environments share the same topology, with documented (not accidental) differences
- [ ] No secret exists in version control, logs, or container images; CI scans for accidental commits
- [ ] Alerts map to the four golden signals and each has a linked runbook
- [ ] Paging alerts are reserved for symptoms requiring immediate action
- [ ] Every incident gets a blameless postmortem with owned, concrete follow-up actions
- [ ] Any hard-to-reverse change has its prior state recorded before the change is made

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "It's a quick manual fix in the console, I'll add it to Terraform later" | "Later" rarely happens, and the manual change is now invisible drift that IaC plan will fight against. |
| "We'll just alert on everything to be safe" | Over-alerting causes alert fatigue, which is functionally identical to having no alerts. |
| "I remember what the DNS record was" | Incident-time memory is unreliable; write it down before, not after. |
| "The postmortem's done, we agreed to be more careful" | Without a concrete mechanism (test, gate, alert), the same incident class recurs. |

## Red Flags

- Environments that were "created once and hand-tuned" with no IaC behind them
- Secrets referenced directly in application config files checked into the repo
- Alert channels with dozens of unacknowledged, ignored notifications
- Postmortems with no dated owner on any action item
- A risky infrastructure change made without first exporting/recording the current state

## Verification

- [ ] `terraform plan` (or equivalent) run and reviewed before every apply, with no drift between state and reality
- [ ] A secret-scanning tool runs in CI and has caught at least one test case
- [ ] On-call runbooks are reachable from the alert itself, not filed separately and hard to find
- [ ] Most recent postmortem's action items were verified as completed, not just filed
