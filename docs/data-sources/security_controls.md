---
page_title: "blast_security_control / blast_security_controls Data Sources"
---

# blast_security_control / blast_security_controls

```hcl
data "blast_security_controls" "scps" {
  type          = "Service Control Policy"   # optional
  platform_type = "Amazon Web Services"      # optional
  name_contains = "baseline"                 # optional, client-side
}

data "blast_security_control" "one" {
  id                       = data.blast_security_controls.scps.ids[0]
  include_policy           = true    # default; fills `policy`
  include_optimized_policy = false   # fills `optimized_policy`
}
```

Each control exposes `id`, `name`, `type`, `cloud_provider`, `status`, `attached_guardrails`, `control_id` (for example the SCP ARN),
`resource_type`, `open_action_count`, `actions_disabled`, and `platform_resources` (`resource_id`, `resource_name`, `resource_type`,
`platform_type`, `attached_effectiveness`, `monitored`).
If a control type has no policy document, `policy` is null and you get a warning.
