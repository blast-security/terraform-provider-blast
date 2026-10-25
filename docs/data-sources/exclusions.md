---
page_title: "blast_exclusions Data Source"
subcategory: ""
description: |-
  List exclusion rules, optionally for one guardrail.
---

# blast_exclusions (Data Source)

List exclusion rules, optionally for one guardrail.

## Example Usage

```terraform
data "blast_exclusions" "for_guardrail" {
  guardrail_id  = "g-aws-vpc-f7264580"  # optional
  name_contains = "break-glass"         # optional, client-side
}
```

## Argument Reference

* `guardrail_id` (Optional) Only exclusions attached to this guardrail.
* `name_contains` (Optional) Case-insensitive substring filter on the name.
* `project_id` (Optional)

## Attribute Reference

`exclusions`: list of objects with `id` and the attributes of `blast_exclusion`.
