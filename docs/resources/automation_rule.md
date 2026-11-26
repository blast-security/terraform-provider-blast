---
page_title: "blast_automation_rule Resource"
subcategory: ""
---

# blast_automation_rule

An automation rule: when a trigger fires (a blocked activity, an exclusion about to expire, …), Blast posts to Slack and/or opens a Jira ticket.

Which fields apply depends on the `trigger`. The provider does not check them: the Blast API validates the rule and lists every problem in one error at apply, so new triggers and fields work without a provider release.

## Example

```hcl
resource "blast_automation_rule" "blocked_to_channel" {
  name    = "Blocked activity to sec-alerts"
  trigger = "BLOCKED_ACTIVITY"

  conditions = {
    security_control_ids = { values = ["gc-aws-scp-1234"] }
  }

  slack = {
    channel     = "#sec-alerts"
    custom_text = "Blocked by Blast"
  }
}

resource "blast_automation_rule" "exclusion_requests" {
  name    = "Exclusion requests"
  trigger = "EXCLUSION_REQUEST_CREATED"

  slack = { channel = "#sec-alerts", notify_on_review = true }
  jira  = { project = "SEC", issue_type = "Task", summary = "Exclusion request" }
}
```

More in `examples/automation-rules`.

## What each trigger takes

| `trigger` | `conditions` | `slack` | `jira` |
|---|---|---|---|
| `BLOCKED_ACTIVITY` | optional | required: `channel`, **or** `use_notification_routing_rules = true` (needed for the exclusion request fields) | — |
| `EXCLUSION_ABOUT_TO_EXPIRE` | — | required: `channel`, `custom_text`, `notify_requester_on_expiry` | — |
| `COVERAGE_DECREASED`, `NEW_GUARDRAILS_ADDED`, `CONTROL_DRIFT_DETECTED` | — | required: `channel`, `custom_text` | — |
| `EXCLUSION_REQUEST_CREATED` | — | `channel`, `custom_text`, `notify_on_review` | optional; give `slack`, `jira` or both |

## Arguments

* `name` (Required) Rule name.
* `trigger` (Required) One of the triggers above (the API rejects others). **Forces replacement.**
* `active` (Optional) Whether the rule runs. Defaults to `true`.
* `conditions` (Optional, `BLOCKED_ACTIVITY` only)
  * `security_control_ids` / `source_entity_ids` (Optional) `{ operator, values }`. `operator` is `EQUAL` (the default) or `NOT_EQUAL`.
* `slack` (Optional)
  * `channel` Slack channel name, with or without `#`.
  * `use_notification_routing_rules` Send to the recipients your notification routing rules pick, instead of a channel.
  * `custom_text` Text added to the message.
  * `allow_exclusion_request`, `allow_request_for`, `max_exclusion_duration_days` (API default 180): the "request an exclusion" button. These need `use_notification_routing_rules`.
  * `notify_on_review`, `notify_requester_on_expiry`: see the table above.
* `jira` (Optional, `EXCLUSION_REQUEST_CREATED` only) `project` (the project **key**), `issue_type`, `summary`, `update_on_review`.
* `overwrite_console_changes` (Optional, bool) Allow an update to delete actions and Slack channels set in the console (see below). Default false.
* `project_id` (Optional) Overrides the provider's project. Changing the effective project (this value, or the provider's `project_id` when unset) forces replacement. Writing out the provider's project explicitly, or adding it after an import by bare ID, does not.

## Attributes

`id`, `unmanaged_actions`, `other_slack_channels`, `created_at`, `created_by`, `modified_at`, `modified_by`.

## Rules edited outside Terraform

An update sends the whole rule (`PUT`), so it would remove anything this resource cannot express: actions such as webhooks (listed in `unmanaged_actions`) and extra Slack channels added in the console (listed in `other_slack_channels`). When a rule has either, a plan that updates or replaces it **fails** and names what would be deleted. To go ahead, remove them in the console, or set:

```hcl
  overwrite_console_changes = true # delete what the console added
```

## Empty values

Leave a block out instead of writing it empty: `slack = {}`, `conditions = {}` and `allow_request_for = []` are rejected at plan time, because the API drops empty values.

## Import

```sh
terraform import blast_automation_rule.x <rule_id>
terraform import blast_automation_rule.x <project_id>/<rule_id>
```
