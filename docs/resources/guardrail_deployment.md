---
page_title: "blast_guardrail_deployment Resource"
subcategory: ""
---

# blast_guardrail_deployment

Deploys one or more catalog guardrails as a security control (for example a new SCP). It does this by creating a Blast task (`POST /tasks`) and, if you choose, opening a Terraform PR.
This is the MVP's "create custom guardrail".

## Example

```hcl
data "blast_guardrails" "aws_high" {
  platform_type = ["Amazon Web Services"]
  severity      = ["High", "Critical"]
}

resource "blast_guardrail_deployment" "baseline" {
  name                  = "High-severity AWS baseline"
  guardrail_catalog_ids = data.blast_guardrails.aws_high.catalog_ids
  security_control_type = "Service Control Policy"
  security_control_name = "blast-high-sev-baseline"
  platform_unit_ids     = ["ou-abcd-11111111"]

  delivery_mode = "open_pr"
  pull_request = {
    title = "Blast: high-severity AWS baseline"
  }
}
```

## Arguments

* `name` (Required) Task name. Can be updated in place.
* `guardrail_catalog_ids` (Required, set) Forces replacement.
* `security_control_type` (Required) For example `Service Control Policy` or `Resource Control Policy`. Forces replacement.
* `security_control_name` (Required) Name of the new control, or of the existing control. Forces replacement.
* `platform_unit_ids` (Required, set) Org unit or account IDs to enforce on. Forces replacement.
* `existing_security_control_id` (Optional) Merge the guardrails into an existing control instead of creating a new one.
* `platform_type` (Optional) Derived from the guardrails when omitted.
* `delivery_mode` (Optional) `task` (default) or `open_pr`. Changing `task` to `open_pr` opens the PR in place. Changing back forces replacement.
* `pull_request` (Optional) `{ title, description, source_branch_prefix, targets }`. Used only with `open_pr`.
* `on_destroy` (Optional) `delete` (default) deletes the Blast task. `abandon` only removes it from Terraform state.
* `project_id` (Optional)

## Attributes

`id` (task ID), `status`, `task_type`, `security_control_id`, `terraform_pr_url`, `created_at`.

## Import

`terraform import blast_guardrail_deployment.x <task_id>`. After import, `security_control_name` is taken from your config without replacing the resource.
