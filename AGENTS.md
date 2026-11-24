# AGENTS.md: Blast Terraform Provider

Instructions for AI coding agents (Claude Code, Codex, Cursor, Copilot, Gemini CLI, ...) and humans maintaining
`terraform-provider-blast`. Read this whole file before changing code, releasing, or answering questions about the provider.
History, decisions, open items and the ownership checklist are in `HANDOFF.md`, an **internal** document shared with the
maintainers separately (it is git-ignored: this repo is public). If it's in your working copy, read it too.

## 1. What this is

| | |
|---|---|
| Provider address | `blast-security/blast` (Terraform Registry, public) |
| Repo | https://github.com/blast-security/terraform-provider-blast (public, MPL-2.0) |
| Registry page | https://registry.terraform.io/providers/blast-security/blast/latest |
| Customer docs | https://docs.blast.security/guides/blast-platform/api/terraform-provider (Mintlify, repo `blast-security/blast-docs`, deploy branch `develop`) |
| Built on | Blast **External API** (OpenAPI 1.5.0), base `https://api.blast.security/api/v1/external` |
| Framework | Go, `hashicorp/terraform-plugin-framework` v1.13 (protocol 6), validators v0.16 |
| Status | Preview (`v0.x`). Breaking changes are allowed before 1.0, but must be in CHANGELOG. |

## 2. What the provider manages

| Terraform | Type | API | Notes |
|---|---|---|---|
| `blast_exclusion` | resource | `/exclusions` CRUD | Stateful. Import: `<id>` or `<project_id>/<id>` |
| `blast_exclusion`, `blast_exclusions` | data | `GET /exclusions[/{id}]` | |
| `blast_guardrail`, `blast_guardrails` | data | `GET /guardrails[/{catalogId}]` | filters pass through as query params |
| `blast_security_control`, `blast_security_controls` | data | `GET /security-controls`, `/{id}/policy` | single lookup fetches policy JSON |
| `blast_guardrail_deployment` | resource | `POST /tasks` (+ `/tasks/{id}/terraform-pr`) | **Task-backed**, see 5.3 |
| `blast_security_control_change` | resource | `POST /tasks/control-changes` | **Task-backed**, SCP/RCP only |
| `blast_project` | resource | `/projects` CRUD | Tenant-scoped (no `x-project-id`) |
| `blast_project`, `blast_projects` | data | `GET /projects[/{id}]` | |
| `blast_cloud_units` | data | `GET /projects/organizations` | flattened org tree, client-side filters |

Not supported (by decision or API gap): direct deploy of tasks, programs, notification routing, exclusion requests,
threats, activities.

## 3. Repository layout

```
main.go                         provider server entrypoint (address registry.terraform.io/blast-security/blast)
internal/client/                thin HTTP client: OAuth token cache, Do(), pagination, typed API calls
  client.go                     auth + request plumbing (x-project-id, NoProject for tenant-scoped calls)
  api.go                        exclusions, guardrails, security controls, tasks
  projects.go                   projects + org units
internal/provider/              Terraform schemas and CRUD
  provider.go                   provider config + resource/data source registration
  common.go                     shared helpers: enum normalisation, list/set conversion, import ID parsing
  task_common.go                shared logic for task-backed resources (delivery_mode, PR, on_destroy)
  *_resource.go / *_data_sources.go
tools/mockserver/               in-memory fake of the API + token endpoint (used by e2e and local dev)
scripts/e2e_mock.sh             full lifecycle test against the mock (CI runs it)
scripts/auth_check.sh           debug OAuth credentials (prints status codes only)
scripts/check_enums.sh          print identifier/operator enums the live API accepts
scripts/probe_expiry_endpoint.sh  safe probe of an internal endpoint (diagnostics only, not used by the provider)
examples/                       complete, projects, provider, smoke-readonly (no writes), live-test (step-by-step writes)
docs/                           Terraform Registry docs (one file per resource/data source, registry format)
.github/workflows/              test.yml (PR/main), release.yml (tags v*), live-smoke.yml (manual/scheduled, real tenant)
```

## 4. Hard rules for agents

1. **Never commit secrets.** No client IDs/secrets, tokens, GPG keys or passphrases in code, examples, tests, docs or
   commit messages. Credentials only via env vars `BLAST_CLIENT_ID` / `BLAST_CLIENT_SECRET` or CI secrets.
2. **Never commit tenant data.** `*.tfstate*`, `.terraform/`, `dev.tfrc`, `bin/`, `dist/` are git-ignored. State files
   from live tests contain real account IDs and policy documents. Don't paste them into issues or PRs either.
3. **Never run write operations against a real tenant without explicit human approval.** Read-only (`examples/smoke-readonly`)
   is fine. `examples/live-test` creates real objects; only run with a named test SCP/account and always `destroy` after.
4. **Never put internal information in this public repo** (roadmap, customer names, internal endpoints' details,
   tenant IDs). Internal notes go in `HANDOFF.md` (git-ignored) or internal tools.
5. **Never invent API fields.** The source of truth is the OpenAPI spec (ask the API team for the latest; it's also at
   `https://docs.blast.security/api/external-api.openapi.json`, behind login). If a field isn't in the spec, don't use it.
6. **Use only `/api/v1/external/*` endpoints.** Internal app endpoints (`/api/v1/...` without `external`) are not a public
   contract and may reject API-key tokens or change without notice.
7. **Every change keeps these green:** `go vet ./...`, `go test ./...`, `scripts/e2e_mock.sh terraform`. If you add or change
   API behaviour, update `tools/mockserver` so e2e covers it.
8. **Every user-visible change updates:** `CHANGELOG.md`, the Registry doc in `docs/`, examples if relevant, and the
   Mintlify guide (section 9). README table if a resource/data source is added.
9. **Plans must be idempotent.** After `apply`, an immediate `plan` must say "No changes". Any perpetual diff is a bug.
10. **Don't hard-code API enums as validators** when the API already validates them (see 5.2). Only validate what the
   provider itself needs (e.g. `delivery_mode`).
11. **Don't change `go.mod`'s module path, the provider address, or resource type names** without a major-version plan.

## 5. Architecture and behaviour (read before editing)

### 5.1 Authentication and requests
- OAuth2 client credentials: `POST https://auth.blast.security/auth/v1/token`, **form body** (`grant_type`, `client_id`,
  `client_secret`). HTTP Basic auth is rejected by the server. Response: `access_token`, `expires_in`.
- Token is cached and refreshed 60 s before expiry (falls back to JWT `exp`, then 5 min). Mutex-protected.
- Every call: `Authorization: Bearer`, JSON. Errors become `*client.APIError` with status + body; `IsNotFound()` drives
  "resource gone, remove from state".
- `x-project-id`: provider-level `project_id` (or `BLAST_PROJECT_ID`) is the default; each resource/data source can
  override. **Project endpoints are tenant-scoped**: use `RequestOptions{NoProject: true}`; a unit test enforces this.
- Pagination: `page`/`size` (size 500), stops on `page.totalPages` or an empty page.
- Env vars: `BLAST_CLIENT_ID`, `BLAST_CLIENT_SECRET`, `BLAST_PROJECT_ID`, `BLAST_API_URL`, `BLAST_TOKEN_URL`.

### 5.2 Enums (labels vs raw keys)
The API accepts labels (`"Account ID"`) and raw keys (`"ACCOUNT_ID"`), and may return either. `canonicalEnum()` maps an API
value to a known label by **longest token-suffix match** (so `NOT_EQUALS` maps to "Not Equals", not "Equals"; `PLATFORM_TYPE_AWS`
and `AWS` map to "Amazon Web Services"). `keepEnum()` keeps the user's spelling when equivalent, so plans don't show drift.
Exclusion `identifier`/`operator` are **not** validated client-side; the label list in `common.go` is only for normalisation.
When the API adds an enum value, add its label to the list in `common.go` (optional, only improves normalisation).

### 5.3 Task-backed resources (`blast_guardrail_deployment`, `blast_security_control_change`)
- `create` = create a Blast task; if `delivery_mode = "open_pr"`, also `POST /tasks/{id}/terraform-pr`.
- The cloud changes only when the task is deployed in Blast or the PR is merged. Terraform does **not** wait.
- `read` refreshes `status` and `terraform_pr_url` only. Task inputs are immutable: changing them = new task (replace).
- In-place updates: `name`; `delivery_mode` task -> open_pr (opens the PR). open_pr -> task forces replace.
- `delete`: `on_destroy = "delete"` (default) deletes the task; `"abandon"` only forgets it. **Never rolls back the cloud.**
- If the task is created but the PR fails, state is saved and an error returned, so Terraform taints it and the next apply
  replaces it.
- Control changes: one open control-change task per control; the API returns 409 and `conflictHint()` explains it.
- `policy` is compared semantically (`jsonEqual`): formatting-only changes don't force replacement.
- **Known design debt:** a customer pointed out these are imperative (one-off tasks), not declarative state. Planned
  direction: replace with a stateful resource representing the security control (guardrails + units) that only creates a
  task on drift. Until then treat them as experimental.

### 5.4 `blast_exclusion`
- `guardrail_catalog_ids` and `expiration_date` **force replacement**, because `PATCH /exclusions/{id}` doesn't accept them.
  If the API adds them to PATCH, drop the `RequiresReplace` modifiers and send them in `UpdateExclusion` (top API ask).
- Exactly one of `sources` / `targets` (`ValidateConfig`). Removing all rules of one kind forces replacement (PATCH keeps
  omitted fields, so switching kinds in place would leave both).
- `units` is Optional+Computed; the response doesn't return units, so the provider keeps what it sent. Omitted means the
  API resolves root org units.
- `expiration_date` is `YYYY-MM-DD`; the API may return a timestamp, and the provider compares the date part.

### 5.5 `blast_project`
- `name` must be unique: checked at **plan** time via `GET /projects/name-available` (`ModifyPlan`).
- The default project ("All Cloud Units", `default = true`): any change fails at plan; delete is refused. Users should
  `terraform state rm` it.
- `unit_ids` = exact membership (sent as `unitIds` full replacement on update). Optional+Computed: omitted means unmanaged.
  Confirmed by the API team: `GET` returns exactly the unitIds sent (no OU expansion).
- `types` is a set in Terraform; the API takes a comma-separated **string** on write and returns an **array**.
- `status` is not accepted by `POST /projects` (API team decision): the provider creates, then `PUT`s `Inactive` if requested.
- `PUT /projects/{id}` is a partial update; the provider always sends name, description and status.

## 6. Local development

```sh
make build      # bin/terraform-provider-blast
make test       # go vet + unit tests
make e2e        # full lifecycle vs mock (needs terraform or: scripts/e2e_mock.sh tofu)
make mock       # mock API on 127.0.0.1:8089
```

Use the local build with a CLI config file (keep it out of git; `dev.tfrc` is ignored):
```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/blast-security/blast" = "/ABS/PATH/bin"
    "registry.opentofu.org/blast-security/blast" = "/ABS/PATH/bin"
  }
  direct {}
}
```
`export TF_CLI_CONFIG_FILE=$PWD/dev.tfrc`, then run `terraform plan` in an example (no `init` needed with dev_overrides).
Against the mock: `BLAST_API_URL=http://127.0.0.1:8089 BLAST_TOKEN_URL=http://127.0.0.1:8089/auth/v1/token BLAST_CLIENT_ID=test BLAST_CLIENT_SECRET=test`.

Gotchas seen in practice:
- **macOS kills a provider binary that was overwritten in place** (signature cache), showing "Unrecognized remote plugin
  message". Build to a new path or `rm` the old binary before copying.
- Remove quarantine on downloaded binaries: `xattr -dr com.apple.quarantine <dir>`.
- zsh doesn't treat `#` as a comment when pasting commands; don't paste lines with trailing comments.
- `go.sum` may be missing in older checkouts: run `go mod tidy` (CI also does).

## 7. Testing levels

| Level | Command | Touches real tenant | When |
|---|---|---|---|
| Unit | `go test ./...` | no | every change |
| Mock lifecycle | `scripts/e2e_mock.sh terraform` | no | every change (CI) |
| Read-only smoke | `examples/smoke-readonly`, `terraform plan` | read only | before every release (CI: live-smoke.yml) |
| Live writes | `examples/live-test`, steps 0-4 | **creates test objects** | before releasing changes to resources; human-approved |

`examples/live-test` steps (each off by default; always `destroy` after):
```sh
V='-var guardrail_catalog_id=<catalog id> -var security_control_name_contains=<TEST scp name>'
terraform apply $V -var step_project=true -var project_unit_name=<test account>    # 0 project
terraform apply $V -var step_exclusion=true                                         # 1 exclusion
terraform apply $V -var step_exclusion=true -var step_deployment=true               # 2 guardrail task
terraform apply $V ... -var step_control_change=true                                # 3 control-change task
terraform apply $V ... -var pr_mode=open_pr                                         # 4 open PRs (needs git integration)
terraform plan  $V <same vars>   # must be "No changes"
terraform destroy $V <same vars>
```
Exclusion identifier test: add `-var test_identifier="VPC ID" -var test_value=vpc-...` (operator defaults to `Like`).

## 8. Releasing

1. All checks green on `main`; CHANGELOG has the version section (move "Unreleased" to the version + date).
2. `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. `release.yml` runs GoReleaser: builds all platforms, signs `SHA256SUMS` with the GPG key (secrets `GPG_PRIVATE_KEY`,
   `PASSPHRASE`), creates the GitHub release with `manifest.json`.
4. The Terraform Registry picks up the release through its GitHub webhook, usually within minutes. If it doesn't, use
   **Resync** on the provider's Settings tab in the Registry. Verify on the Registry page and with `terraform init` in an
   empty folder using `source = "blast-security/blast"`.
5. Then merge the matching docs PR in `blast-docs` (Mintlify) and post release notes.

Versioning: semver. New resource/data source = minor (`0.x.0`); fixes/docs = patch. Docs pin `version = "~> 0.2"`; bump
that in docs/README when a release requires it, and only after the release is on the Registry.

## 9. Customer docs (Mintlify)

Page: `guides/blast-platform/api/terraform-provider` (sidebar: Guides > Blast Platform > Integrations > Terraform Provider).
Keep it in sync with `docs/` and CHANGELOG: capabilities table, argument tables, update/replace table, import, troubleshooting.
Work on a branch and open a PR to `develop`; merge after the provider release is live. Customers sign in to read docs.

## 10. Adding a new resource (checklist)

1. Confirm the endpoints exist in the External API spec (CRUD + 404 on missing + IDs stable).
2. Client: typed structs + methods in `internal/client/` (use `RequestOptions`, `listPaged` for lists).
3. Resource: schema with descriptions, plan modifiers (`UseStateForUnknown` for stable computed values, `RequiresReplace`
   only where the API can't update), Create/Read/Update/Delete/ImportState; Read removes state on 404.
4. Normalise API enums via `canonicalEnum`/`keepEnum` to avoid drift.
5. Register in `provider.go`.
6. Mock endpoints in `tools/mockserver` (return raw enum keys to exercise normalisation).
7. Extend `scripts/e2e_mock.sh` or add an example and verify: create, re-plan clean, update in place, import, destroy.
8. Registry doc `docs/resources/<name>.md` (+ data sources), example, CHANGELOG, README table, Mintlify guide.
9. Live test on the sandbox tenant before release.
