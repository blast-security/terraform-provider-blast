# Terraform Provider for Blast Security

The `blast-security/blast` provider manages [Blast Security](https://blast.security) through the Blast External API.

> **Preview.** Resource names and arguments may change before 1.0.

* Documentation: [docs.blast.security](https://docs.blast.security/guides/blast-platform/api/terraform-provider) and [`docs/`](docs/)
* Registry: [registry.terraform.io/providers/blast-security/blast](https://registry.terraform.io/providers/blast-security/blast)

## Usage

```hcl
terraform {
  required_providers {
    blast = {
      source  = "blast-security/blast"
      version = "~> 0.1"
    }
  }
}

provider "blast" {} # reads BLAST_CLIENT_ID / BLAST_CLIENT_SECRET
```

Create the client ID and secret in Blast under **Settings → Access Keys**.

## What it manages

| Capability | Terraform |
|---|---|
| Create exclusion rule | `resource "blast_exclusion"` (create, update, delete, import) |
| Get exclusion rule(s) | `data "blast_exclusion"`, `data "blast_exclusions"` |
| Get guardrails | `data "blast_guardrail"`, `data "blast_guardrails"` |
| Create custom guardrail (deploy catalog guardrails as a control) | `resource "blast_guardrail_deployment"` |
| Get security controls | `data "blast_security_control"` (incl. policy JSON), `data "blast_security_controls"` |
| Change security controls (SCP/RCP) | `resource "blast_security_control_change"` |

The two task-backed resources support `delivery_mode = "task"` (default) or `"open_pr"`. **Direct deploy is out of scope** because the API doesn't expose it.
For full docs, see [`docs/`](docs/). For a working config covering every resource and data source, see [`examples/complete`](examples/complete/main.tf).

## Developing

Requires Go 1.23+ and Terraform 1.5+ (or OpenTofu).

```sh
make build           # -> bin/terraform-provider-blast
make test            # unit tests
make e2e             # full lifecycle against the in-memory mock API
```

Point Terraform at the local binary with `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/blast-security/blast" = "/ABSOLUTE/PATH/TO/terraform-provider-blast/bin"
  }
  direct {}
}
```

Then:

```sh
export BLAST_CLIENT_ID=...  BLAST_CLIENT_SECRET=...
cd examples/smoke-readonly && terraform plan     # read-only, makes no changes
```

## Try it without a tenant

`tools/mockserver` is an in-memory fake of the API and token endpoint.

```sh
make mock   # terminal 1
export BLAST_API_URL=http://127.0.0.1:8089 BLAST_TOKEN_URL=http://127.0.0.1:8089/auth/v1/token
export BLAST_CLIENT_ID=test BLAST_CLIENT_SECRET=test
cd examples/complete && terraform apply
```

`make e2e` runs the full lifecycle against the mock: create, an idempotent re-plan, switching task to open_pr in place, an in-place update,
a forced replacement, the 409 conflict, an import round-trip and destroy.

## Design notes

* **Auth:** OAuth client credentials. The token is cached and refreshed 60 seconds before `expires_in` (or before the JWT `exp` if `expires_in` is missing).
* **Projects:** the provider-level `project_id` is sent as `x-project-id`. Every resource and data source can override it.
* **Enums:** you can write labels ("Account ID") or raw keys ("ACCOUNT_ID"). The provider treats them as equal, so they never show up as drift.
* **Exclusions:** `guardrail_catalog_ids` and `expiration_date` force replacement because PATCH can't change them.
  `units` is optional. When omitted, the API resolves the root org units.
* **Task-backed resources** never roll back the cloud on destroy. They delete the Blast task, or forget it with `on_destroy = "abandon"`.
  If opening the PR fails after the task was created, the resource is tainted, so the next apply deletes the task and tries again.
  Task inputs are immutable. Only `name` and switching `task` to `open_pr` are applied in place.

## Release

Tag `vX.Y.Z` and the workflow at `.github/workflows/release.yml` builds, signs and publishes with GoReleaser.
Publishing to the Terraform Registry needs:

1. A public GitHub repo named `terraform-provider-blast` under the `blast-security` org.
2. A GPG key: add the private key and passphrase as `GPG_PRIVATE_KEY` / `PASSPHRASE` secrets, and the public key in the Registry.
3. Signing in to registry.terraform.io with GitHub and publishing the repo.

## Contributing / support

Customers: contact Blast support. Issues and pull requests are welcome.

## License

[MPL-2.0](LICENSE)
