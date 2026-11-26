---
page_title: "blast_gaps Data Source"
subcategory: ""
description: |-
  List defense gaps with optional filters.
---

# blast_gaps (Data Source)

Lists Blast defense gaps for a project. Gaps are project-scoped — a specific `project_id` is required (the default all-units scope is rejected by the API).

Returns summary fields for each gap. For full detail including mitigation guidance and priority indicators, use the `blast_gap` data source.

## Example Usage

```terraform
data "blast_gaps" "open_critical" {
  project_id     = var.project_id
  tab            = "open"
  priority_level = ["critical", "high"]
}

output "open_critical_count" {
  value = length(data.blast_gaps.open_critical.gaps)
}
```

## Argument Reference

* `project_id` (Optional) — Blast project ID. Gaps are project-scoped; this field is required (the default all-units scope is rejected by the API).
* `tab` (Optional) — Status group: `open` (open + in_progress), `history` (resolved, closed, ignored), or `all` (no status filter). The API defaults to `open` when unset.
* `category` (Optional) — Filter by one or more gap categories.
* `type` (Optional) — Filter by one or more gap types.
* `priority_level` (Optional) — Filter by one or more priority levels: `critical`, `high`, `medium`, `low`.
* `status` (Optional) — Filter by one or more status values: `open`, `in_progress`, `resolved`, `closed`, `ignored`.
* `is_quick_win` (Optional) — Only quick-win (`true`) or non-quick-win (`false`) gaps. Unset returns both.
* `external_key` (Optional) — Exact match on `external_key`. Useful for a sync to look up the gap it owns.
* `name` (Optional) — Partial match on gap name.

## Attribute Reference

* `gaps` — List of matching gaps. Each element contains:
  * `id` — Gap ID.
  * `project_id` — Project the gap belongs to.
  * `external_key` — Stable external key (empty string if not set).
  * `name` — Gap name.
  * `category` — Gap category.
  * `type` — Gap type.
  * `priority_score` — Numeric priority score.
  * `priority_level` — Priority level.
  * `is_quick_win` — Whether this gap is a quick win.
  * `status` — Gap status.
  * `gain_points` — Points awarded on resolution.
  * `created_at`, `updated_at` — Timestamps.
