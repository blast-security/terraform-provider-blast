---
page_title: "blast_automation_rules Data Source"
subcategory: ""
description: |-
  List automation rules, optionally filtered by trigger or active state.
---

# blast_automation_rules (Data Source)

List automation rules, optionally filtered by trigger or active state.

## Example Usage

```terraform
# All rules
data "blast_automation_rules" "all" {}

# Only active rules for a specific trigger
data "blast_automation_rules" "blocked_active" {
  trigger = "BLOCKED_ACTIVITY"
  active  = true
}
```

## Argument Reference

* `project_id` (Optional) Overrides the provider project.
* `trigger` (Optional) Filter to rules of this trigger type (e.g. `BLOCKED_ACTIVITY`).
* `active` (Optional, bool) Only active (`true`) or disabled (`false`) rules. Unset returns both.

## Attribute Reference

* `rules` — List of matching rules. Each item contains:
  * `id` — Rule ID.
  * `name` — Rule name.
  * `trigger` — Trigger type.
  * `active` — Whether the rule is active.
  * `destinations` — Channels or actions the rule notifies (Slack channels, `jira`, `notification-routing`).
  * `modified_by`, `modified_at` — Last modification actor and timestamp.
