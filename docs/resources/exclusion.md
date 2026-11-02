---
page_title: "blast_exclusion Resource"
subcategory: ""
---

# blast_exclusion

An exclusion rule that exempts sources or targets from one or more guardrails.

## Example

```hcl
resource "blast_exclusion" "break_glass" {
  name                  = "Break-glass account"
  note                  = "Security team emergency access"
  guardrail_catalog_ids = ["g-aws-s3-73735b0c"]

  sources = [{
    identifier = "Account ID"
    operator   = "Equals"
    values     = ["123456789012"]
  }]

  expiration_date = "2027-12-31"
}
```

## Arguments

* `name` (Required) Rule name.
* `guardrail_catalog_ids` (Required, set) Guardrail catalog IDs. **Forces replacement.**
* `sources` / `targets` (exactly one required) List of `{ identifier, operator, values }`.
  `identifier`: for example `Account ID`, `ARN`, `Source IP`, `Tag`, `VPC ID` (`aws:SourceVpc`), `VPC Endpoint ID` (`aws:SourceVpce`).
  `operator`: for example `Equals`, `Not Equals`, `Like` (AWS rules use `Like`, with `*` wildcards).
  Allowed values depend on the platform and on source vs. target; the Blast API validates them (`GET /api/v1/external/enums`).
  Removing all rules of one kind forces replacement.
* `note` (Optional)
* `platform_type` (Optional) `Amazon Web Services`, `Azure Cloud`, `Google Cloud Platform`. Resolved from the guardrail when omitted.
* `units` (Optional, set) Org unit IDs. When omitted, the API applies the rule to all root org units of the guardrail.
* `expiration_date` (Optional) `YYYY-MM-DD`. **Forces replacement** because the API cannot update it in place.
* `project_id` (Optional) Overrides the provider's project. Forces replacement.

## Attributes

`id`, `is_expired`, `created_at`, `created_by`, `updated_at`.

## Import

```sh
terraform import blast_exclusion.x <exclusion_id>
terraform import blast_exclusion.x <project_id>/<exclusion_id>
```
