---
page_title: "blast_security_control_change Resource"
---

# blast_security_control_change

Changes an existing **Service Control Policy or Resource Control Policy**: its policy content and/or the org units it is attached to.
It does this by creating a Blast control-change task (`POST /tasks/control-changes`) and, if you choose, opening a Terraform PR.

Blast allows only one *open* control-change task per control. A second one fails with HTTP 409 and an explanation.

## Example

```hcl
data "blast_security_control" "baseline" {
  id = "sc-123"
}

resource "blast_security_control_change" "attach_dev" {
  security_control_id = data.blast_security_control.baseline.id
  policy              = data.blast_security_control.baseline.policy   # unchanged content
  attach_unit_ids     = ["ou-abcd-22222222"]
  delivery_mode       = "open_pr"
}
```

## Arguments

* `security_control_id` (Required) Forces replacement.
* `policy` (Required) Full desired policy JSON. Required even for attachment-only changes. Changes that only affect formatting are ignored. A change in meaning forces a new task.
* `attach_unit_ids`, `detach_unit_ids` (Optional, sets) Force replacement.
* `name` (Optional) Task name. Blast assigns one when omitted. Can be updated in place.
* `delivery_mode`, `pull_request`, `on_destroy`, `project_id`: same as `blast_guardrail_deployment`.

## Attributes

`id`, `status`, `task_type`, `terraform_pr_url`, `created_at`.

## Import

`terraform import blast_security_control_change.x <task_id>`
