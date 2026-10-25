---
page_title: "blast_security_controls Data Source"
subcategory: ""
description: |-
  List security controls.
---

# blast_security_controls (Data Source)

List security controls.

## Example Usage

```terraform
data "blast_security_controls" "scps" {
  type          = "Service Control Policy"  # optional
  platform_type = "Amazon Web Services"     # optional
  name_contains = "baseline"                # optional, client-side
}
```

## Argument Reference

* `type`, `platform_type`, `name_contains`, `project_id` (all Optional).

## Attribute Reference

`ids` (list of strings) and `security_controls` (list of objects with the attributes of `blast_security_control`, without the policy fields).
