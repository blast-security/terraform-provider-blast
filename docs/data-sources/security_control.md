---
page_title: "blast_security_control Data Source"
subcategory: ""
description: |-
  Look up a security control (SCP, RCP, ...) and its policy document.
---

# blast_security_control (Data Source)

Look up a security control (SCP, RCP, ...) and its policy document.

## Example Usage

```terraform
data "blast_security_control" "baseline" {
  id                       = "<SECURITY_CONTROL_ID>"
  include_policy           = true   # default; fills `policy`
  include_optimized_policy = false  # fills `optimized_policy`
}
```

## Argument Reference

* `id` (Required) Blast security control ID.
* `include_policy` (Optional) Fetch the deployed policy JSON into `policy`. Default `true`.
* `include_optimized_policy` (Optional) Fetch Blast's optimized policy into `optimized_policy`. Default `false`.
* `project_id` (Optional)

## Attribute Reference

`name`, `type`, `cloud_provider`, `status`, `attached_guardrails`, `control_id` (e.g. the SCP ARN), `resource_type`, `open_action_count`, `actions_disabled`, `platform_resources` (`resource_id`, `resource_name`, `resource_type`, `platform_type`, `attached_effectiveness`, `monitored`), `policy`, `optimized_policy`.
