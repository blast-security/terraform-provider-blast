---
page_title: "blast_gap Resource"
subcategory: ""
description: |-
  Manages a Blast defense gap: a detected security-posture issue that tracks priority, status, and remediation guidance.
---

# blast_gap (Resource)

A defense gap is a detected security-posture issue tracked by Blast. Gaps are project-scoped — a specific `project_id` is required (the default all-units scope is rejected by the API).

## Example Usage

```terraform
resource "blast_gap" "iam_unguarded" {
  project_id = var.project_id   # required; gaps are project-scoped
  name       = "IAM role permission changes going unguarded"
  category   = "missing_defense"
  type       = "CONFIGURATION_RESTRICTION"

  priority_score = 85
  priority_level = "high"
  is_quick_win   = false

  priority_indicators = {
    effective_exposure    = { count = 10, weight = 30, max_weight = 50 }
    suspicious_activities = { count = 0,  weight = 0,  max_weight = 20 }
    violations_found      = { count = 3,  weight = 15, max_weight = 30 }
    threat_intel          = { count = 0,  weight = 0,  max_weight = 10 }
  }

  mitigation = {
    description = "Deploy a guardrail that blocks IAM role permission changes."
  }
}
```

## Argument Reference

### Required

* `project_id` — Blast project ID. Gaps are project-scoped; this field is required (the default all-units scope is rejected by the API). Changing this forces a new gap.
* `name` — Human-readable name of the gap.
* `category` — Gap category. Known values: `missing_defense`, `defense_misalignment`, `exclusion_hygiene`, `defense_orchestration`. Validated by the API.
* `type` — Gap type. Known values: `EXPIRED_EXCLUSION`, `UNUSED_EXCLUSION`, `UNMANAGED_EXCLUSION`, `MULTI_CLOUD_MISALIGNMENT`, `CROSS_ENVIRONMENT_MISALIGNMENT`, `PARTIAL_ENFORCED`, `CONFIGURATION_RESTRICTION`, `ACCESS_RESTRICTION`, `DEFENSE_PROTECTION`, `UNOPTIMIZED_CONTROL`, `UNCONSOLIDATED_ENFORCEMENT`, `WORKFLOW_BLOCKED`. Validated by the API.
* `priority_score` — Integer priority score from 0 to 100.
* `priority_level` — Priority level: `critical`, `high`, `medium`, or `low`.
* `is_quick_win` — Whether resolving this gap is considered a quick win.
* `priority_indicators` — Breakdown of signals driving the priority score. Required sub-blocks:
  * `effective_exposure`, `suspicious_activities`, `violations_found` — Each with:
    * `count` (Int) — Number of occurrences.
    * `weight` (Float) — Weight in the priority score.
    * `max_weight` (Float) — Maximum possible weight.
  * `threat_intel` — Same three fields plus optional `threat_id` and `link`.
* `mitigation` — Remediation guidance.
  * `description` (Required) — Human-readable remediation description.
  * `link` (Optional) — Deep link to the mitigating guardrail or exclusion.
  * `guardrail_name` (Optional) — Name of the mitigating guardrail.
  * `guardrail_id` (Optional) — ID of the mitigating guardrail.
  * `impact_label` (Optional) — Human-readable impact label.

### Optional

* `external_key` — Stable key for external upsert syncs. Unique per project when set.
* `description` — Detailed description of the gap.
* `status` — Gap status: `open` (default), `in_progress`, `resolved`, `closed`, or `ignored`.
* `gain_points` — Integer points awarded when the gap is resolved. Defaults to 0.
* `last_detected_at` — ISO 8601 timestamp of the most recent detection.
* `resolved_at` — ISO 8601 timestamp when the gap was resolved. When `status` is `resolved` and this is omitted, the API defaults it to the current time.

## Attribute Reference

In addition to the arguments listed above, the following computed attributes are exported:

* `id` — Gap ID.
* `created_at` — When the gap was created.
* `updated_at` — When the gap was last updated.

## Update Behaviour

`PUT /gaps/{id}` is a full replacement: all current plan values are sent on every update. Any field that is omitted from the config is removed from the stored gap. All fields except `project_id` can be changed in place.

## Import

Gaps can be imported using `<project_id>/<id>`. The project ID is always required because gaps are project-scoped.

```
terraform import blast_gap.example prj-abc123/gap-def456
```
