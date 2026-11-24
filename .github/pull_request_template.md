## What and why

<!-- One or two sentences. For a sync PR: the bff-hub commit and which API objects changed. -->

## Types added or changed

| Type | Kind (resource / data source) | Policy status | New or changed |
|---|---|---|---|
| | | | |

## Needs a product decision

<!-- API objects not in terraform-sync-policy.json, or anything you think the policy gets wrong. "None" if none. -->

## Checklist (AGENTS.md sections 10 and 11)

- [ ] Every resource in this PR is `approved` in `terraform-sync-policy.json` (no resource for Blast-computed objects)
- [ ] No `*_gen.go`, generator header, `rawJSON` or `TODO` left
- [ ] API rewrites (case, prefixes, name vs key, enum labels) keep the configured value; no "inconsistent result"
- [ ] Optional collections and all-optional objects have validators
- [ ] `project_id` uses `projectIDAttribute()` + `planProjectReplace`; tenant-scoped calls use `NoProject`
- [ ] Full-replace `PUT`: fields Terraform doesn't manage can't be silently deleted
- [ ] Page size within the API maximum
- [ ] Mock behaves like the API (rewrites, timestamps, stable order, errors)
- [ ] e2e: create, clean plan, update, import (`<id>` and `<project_id>/<id>`) -> clean plan, empty values, destroy
- [ ] Registry doc, example, CHANGELOG, README table
- [ ] No secrets, state, tenant IDs or internal details (public repo)

## Test output

```
go vet ./...
go test ./...
scripts/e2e_mock.sh terraform
```

## Unsure about

<!-- Anything a reviewer should look at closely. -->
