---
page_title: "blast_guardrail / blast_guardrails Data Sources"
---

# blast_guardrail / blast_guardrails

```hcl
data "blast_guardrail" "s3" { catalog_id = "g-aws-s3-73735b0c" }

data "blast_guardrails" "aws_high" {
  platform_type      = ["Amazon Web Services"]
  severity           = ["High", "Critical"]
  status             = "Active"
  security_framework = ["CIS AWS Foundations 4.0"]
}
```

`blast_guardrails` filters (all optional): `name`, `severity`, `status`, `platform_type`, `platform_service`, `security_framework`,
`guardrail_type`, `risk`, `program_name`, `has_violations`. List filters match any of the values given. It returns `catalog_ids` and `guardrails`.

Each guardrail exposes: `id`, `catalog_id`, `name`, `description`*, `severity`, `status`, `coverage`, `defense_score`, `has_violations`,
`has_effective_exposure`, `simulation_results`, `risks`, `platform_type`, `platform_service`, `guardrail_type`*, `security_frameworks`*,
`policy_id`*, `security_controls`* (\* only filled in by the single-guardrail lookup).
