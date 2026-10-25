---
page_title: "blast_exclusion / blast_exclusions Data Sources"
---

# blast_exclusion / blast_exclusions

```hcl
data "blast_exclusion" "one" { id = "exc-123" }

data "blast_exclusions" "for_guardrail" {
  guardrail_id  = "g-aws-s3-73735b0c"  # optional
  name_contains = "break-glass"        # optional, client-side
}
```

Each exclusion exposes `id`, `name`, `note`, `platform_type`, `guardrail_catalog_ids`, `sources`, `targets`, `units`,
`expiration_date`, `is_expired`, `created_at`, `created_by`, `updated_at`.
