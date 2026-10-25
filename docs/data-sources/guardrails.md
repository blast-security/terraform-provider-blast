---
page_title: "blast_guardrails Data Source"
subcategory: ""
description: |-
  Search the guardrail catalog.
---

# blast_guardrails (Data Source)

Search the guardrail catalog.

## Example Usage

```terraform
data "blast_guardrails" "aws_high" {
  platform_type      = ["Amazon Web Services"]
  severity           = ["High", "Critical"]
  status             = "Active"
  security_framework = ["CIS AWS Foundations 4.0"]
}
```

## Argument Reference

All optional, combined with AND; list filters match any value: `name`, `severity`, `status`, `platform_type`, `platform_service`,
`security_framework`, `guardrail_type`, `risk`, `program_name`, `has_violations`, `project_id`.

## Attribute Reference

`catalog_ids` (list of strings) and `guardrails` (list of objects with the summary attributes of `blast_guardrail`; `description`, `guardrail_type`, `security_frameworks`, `policy_id` and `security_controls` are only filled by the single lookup).
