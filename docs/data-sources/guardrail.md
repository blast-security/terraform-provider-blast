---
page_title: "blast_guardrail Data Source"
subcategory: ""
description: |-
  Look up a guardrail by catalog ID.
---

# blast_guardrail (Data Source)

Look up a guardrail by catalog ID.

## Example Usage

```terraform
data "blast_guardrail" "vpc" {
  catalog_id = "g-aws-vpc-f7264580"
}
```

## Argument Reference

* `catalog_id` (Required) Guardrail catalog ID.
* `project_id` (Optional)

## Attribute Reference

`id`, `name`, `description`, `severity`, `status`, `coverage`, `defense_score`, `has_violations`, `has_effective_exposure`, `simulation_results`, `risks`, `platform_type`, `platform_service`, `guardrail_type`, `security_frameworks`, `policy_id`, `security_controls` (`id`, `name`, `type`, `status`).
