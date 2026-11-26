---
page_title: "blast_automation_rule Data Source"
subcategory: ""
description: |-
  Look up a single automation rule by ID.
---

# blast_automation_rule (Data Source)

Look up a single automation rule by ID.

## Example Usage

```terraform
data "blast_automation_rule" "example" {
  id = "<AUTOMATION_RULE_ID>"
}
```

## Argument Reference

* `id` (Required) Automation rule ID.
* `project_id` (Optional) Overrides the provider project.

## Attribute Reference

* `name` — Rule name.
* `trigger` — What fires the rule (e.g. `BLOCKED_ACTIVITY`, `EXCLUSION_ABOUT_TO_EXPIRE`, `COVERAGE_DECREASED`, `NEW_GUARDRAILS_ADDED`, `EXCLUSION_REQUEST_CREATED`, `CONTROL_DRIFT_DETECTED`).
* `active` — Whether the rule is currently active.
* `conditions` — (BLOCKED\_ACTIVITY only) Filter conditions. Contains `security_control_ids` and `source_entity_ids`, each with `operator` and `values`.
* `slack` — Slack notification settings. Contains `channel`, `use_notification_routing_rules`, `custom_text`, `allow_exclusion_request`, `allow_request_for`, `max_exclusion_duration_days`, `notify_on_review`, `notify_requester_on_expiry`.
* `jira` — Jira notification settings. Contains `project`, `issue_type`, `summary`, `update_on_review`.
* `unmanaged_actions` — Actions on the rule that this provider cannot express (e.g. webhooks added in the console).
* `created_at`, `created_by`, `modified_at`, `modified_by` — Audit timestamps and actors.
