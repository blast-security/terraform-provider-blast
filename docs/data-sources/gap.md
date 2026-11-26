---
page_title: "blast_gap Data Source"
subcategory: ""
description: |-
  Look up a single security gap by ID.
---

# blast_gap (Data Source)

Look up a single security gap by ID. A gap represents a detected security posture issue, including its priority, status, and remediation guidance.

## Example Usage

```terraform
data "blast_gap" "example" {
  id         = "<GAP_ID>"
  project_id = "<PROJECT_ID>" # or set project_id on the provider
}
```

## Argument Reference

* `id` (Required) Gap ID.
* `project_id` (Optional) Overrides the provider project. Gaps belong to one project: the API rejects the request
  unless a specific project is set here or on the provider (the default "All Cloud Units" scope is not accepted).

## Attribute Reference

* `name` — Gap name.
* `description` — Detailed description of the gap.
* `external_key` — Stable key used for external upsert syncs (optional).
* `category` — Gap category (e.g. `missing_defense`, `defense_misalignment`, `exclusion_hygiene`, `defense_orchestration`).
* `type` — Gap type (e.g. `EXPIRED_EXCLUSION`, `UNUSED_EXCLUSION`, `WORKFLOW_BLOCKED`).
* `priority_score` — Numeric priority score.
* `priority_level` — Priority level: `critical`, `high`, `medium`, or `low`.
* `is_quick_win` — Whether this gap is classified as a quick win.
* `status` — Status: `open`, `in_progress`, `resolved`, `closed`, or `ignored`.
* `gain_points` — Points gained by resolving this gap.
* `priority_indicators` — Breakdown of what drives the priority. Contains `effective_exposure`, `suspicious_activities`, `violations_found` (each with `count`, `weight`, `max_weight`) and `threat_intel` (additionally `threat_id`, `link`).
* `mitigation` — Remediation guidance. Contains `description`, `link`, `guardrail_name`, `guardrail_id`, `impact_label`.
* `last_detected_at` — When the gap was last detected.
* `resolved_at` — When the gap was resolved (if applicable).
* `created_at`, `updated_at` — Timestamps.
