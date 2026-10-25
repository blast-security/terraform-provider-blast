---
page_title: "blast_exclusion Data Source"
subcategory: ""
description: |-
  Look up one exclusion rule by ID.
---

# blast_exclusion (Data Source)

Look up one exclusion rule by ID.

## Example Usage

```terraform
data "blast_exclusion" "one" {
  id = "<EXCLUSION_ID>"
}
```

## Argument Reference

* `id` (Required) Exclusion ID.
* `project_id` (Optional) Overrides the provider project.

## Attribute Reference

`name`, `note`, `platform_type`, `guardrail_catalog_ids`, `sources`, `targets` (each `identifier`, `operator`, `values`), `units`, `expiration_date`, `is_expired`, `created_at`, `created_by`, `updated_at`.
