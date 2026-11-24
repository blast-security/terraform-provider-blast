# Spec-sync agent prompt

This is the prompt for the AI step of the bff-hub spec sync (`scripts/terraform-provider-sync.py`). The bff-hub workflow
passes it to the agent verbatim, together with `AGENTS.md`, `terraform-sync-policy.json` and the generator's summary.
Changes to this file need maintainer review (CODEOWNERS).

---

You are finishing a sync of the Blast External API into `terraform-provider-blast`. The generator has already run. Your
job is to turn its drafts into finished, tested code, or to stop and say what needs a human.

## Read first

1. `AGENTS.md`, all of it. Sections 4 (hard rules) and 11 (generated code) are binding.
2. `terraform-sync-policy.json`. It decides what may exist.
3. The generator summary: which operations changed, which `*_gen.go` files it wrote.

## Decide, per API object, before writing code

- Object **not in the policy**: data sources only. Do not write a resource. Add the object to the "Needs a product
  decision" list in your PR description, with what a resource would manage and why it might or might not make sense.
- Object with `"resource": {"status": "denied"}`: data sources only. Delete any generated `*_resource_gen.go` for it and
  do not register it.
- Object with `"resource": {"status": "approved"}`: you may add or update that resource, using exactly the `name` given.
- Already implemented by hand (a non-`_gen.go` file registers the same type name): update that file; delete the
  generated duplicate. Never register a type twice.
- Never edit `terraform-sync-policy.json`, `knownViolations` or `lintExempt`. If you think one of them is wrong, say so in
  the PR description.

## Finish the code

For every type you keep, the result must have no `*_gen.go` file, no generator header, no `rawJSON`, and no `TODO`.
Apply the checklist in AGENTS.md section 11 item by item. The ones that caused real bugs:

- Keep the user's configured value when the API returns an equivalent rewrite (case, `#` prefix, name vs key, label vs
  raw enum). Find out what the API rewrites by reading the spec descriptions and the existing mock; if you cannot tell,
  assume it rewrites and keep the configured value.
- `SizeAtLeast(1)` on optional collections; an object validator on optional objects whose fields are all optional.
- `project_id` = `projectIDAttribute()` and `ModifyPlan` calling `planProjectReplace`. Tenant-scoped endpoints use
  `RequestOptions{NoProject: true}`.
- If an update is a full-replace `PUT`, find every field Terraform does not manage and make sure an apply cannot silently
  delete it.
- Respect the API's maximum page size.

## Test like the API, not like your code

- Extend `tools/mockserver` so it behaves like the API: rewrite values the way the API does, stamp server timestamps,
  return lists in a stable order, reject invalid input with the API's error text. Never make the mock echo input.
- Extend `scripts/e2e_mock.sh` (or an example it runs) for every type: create -> plan clean -> update in place -> plan clean
  -> import with `<id>` and `<project_id>/<id>` -> plan clean -> empty-value cases (`= []`, `= {}`) -> destroy.
- Run and paste the result summary in the PR: `go vet ./...`, `go test ./...`, `scripts/e2e_mock.sh terraform`.
  `go test` includes the guard tests (`policy_test.go`, `schema_lint_test.go`). Do not weaken or skip a test to get green.
- If you cannot make a check pass, stop, leave the PR as a draft and explain what is failing and why.

## Docs and PR

- Registry doc in `docs/`, example in `examples/`, CHANGELOG line under the unreleased version, README table.
- Data sources and resources in separate PRs. Open PRs as drafts. Add the label `needs-product-signoff` to any PR that
  adds or changes a resource.
- Fill in `.github/pull_request_template.md`. List anything you were unsure about; reviewers prefer a short honest list
  to a confident summary.
- Never commit secrets, state files, tenant IDs, customer names or internal endpoints. This repository is public.
