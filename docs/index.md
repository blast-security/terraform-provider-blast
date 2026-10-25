---
page_title: "Provider: Blast Security"
description: |-
  Manage Blast Security exclusions, guardrail deployments and security-control changes.
---

# Blast Security Provider

Manages [Blast Security](https://blast.security) through the Blast External API (v1).

## Authentication

The provider uses OAuth 2.0 client credentials against `https://auth.blast.security/auth/v1/token`.
Create a client ID and secret in Blast, then set them as environment variables (recommended) or provider arguments.

```hcl
provider "blast" {}   # reads BLAST_CLIENT_ID / BLAST_CLIENT_SECRET
```

## Argument Reference

| Argument | Env var | Description |
|---|---|---|
| `client_id` | `BLAST_CLIENT_ID` | OAuth client ID. |
| `client_secret` | `BLAST_CLIENT_SECRET` | OAuth client secret (sensitive). |
| `project_id` | `BLAST_PROJECT_ID` | Default Blast project (sent as `x-project-id`). Omit for "All Cloud Units". Every resource and data source accepts its own `project_id` override. |
| `api_url` | `BLAST_API_URL` | Defaults to `https://api.blast.security`. |
| `token_url` | `BLAST_TOKEN_URL` | Defaults to `https://auth.blast.security/auth/v1/token`. |

## How task-backed resources work

`blast_guardrail_deployment` and `blast_security_control_change` don't change your cloud directly. They create a **Blast task**.
With `delivery_mode = "open_pr"` they also ask Blast to open a Terraform PR/MR in the git integration connected to the tenant.
The cloud changes when the task is deployed (in Blast, or by merging the PR). `status` and `terraform_pr_url` are refreshed on every plan.
Destroying the resource deletes the task (or forgets it with `on_destroy = "abandon"`). It never rolls back a change that was already deployed.
