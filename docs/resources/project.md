---
page_title: "blast_project Resource"
subcategory: ""
description: |-
  A Blast project: a named view over a set of cloud units.
---

# blast_project (Resource)

A Blast project is a named view over a set of cloud units (AWS accounts and OUs, Azure subscriptions, ...).
Use its `id` as `project_id` on other Blast resources and data sources to scope them to the project.

## Example Usage

```terraform
data "blast_cloud_units" "prod_ou" {
  type          = "ORGANIZATIONAL_UNIT"
  name_contains = "Prod"
}

resource "blast_project" "production" {
  name        = "Production"
  description = "Production accounts"
  types       = ["Production"]
  unit_ids    = data.blast_cloud_units.prod_ou.ids
}

data "blast_guardrails" "in_production" {
  project_id = blast_project.production.id
}
```

## Argument Reference

* `name` (Required) Unique within the tenant. Checked at plan time, so a duplicate name fails `terraform plan`, not `apply`.
* `description` (Optional)
* `types` (Optional, set) `Development`, `Test` and/or `Production`.
* `unit_ids` (Optional, set) Platform unit IDs in the project. This is the **exact** membership: units not listed are removed. Use `[]` for an empty project. Omit the argument to leave membership unmanaged by Terraform (for example, when it is edited in the Blast UI). Find IDs with `data.blast_cloud_units`.
* `status` (Optional) `Active` (default) or `Inactive`.

## Attribute Reference

* `id`: project ID.
* `default`: `true` for the tenant default project ("All Cloud Units").
* `platform_types`: cloud platforms covered.
* `number_of_units`
* `units`: list of `id`, `name`, `platform_type`, `platform_org_id`, `grouping_unit`, `unit_types`, `regions`.

## Behavior

* Every argument updates in place; nothing forces a replacement.
* The default project ("All Cloud Units") can be imported for reference, but any change to it fails at plan time, and `destroy` refuses to delete it. Use `terraform state rm` to stop tracking it.
* Projects are tenant-wide: the provider's `project_id` setting does not apply to them.

## Import

```shell
terraform import blast_project.production <PROJECT_ID>
```
