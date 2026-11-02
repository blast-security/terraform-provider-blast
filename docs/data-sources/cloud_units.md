---
page_title: "blast_cloud_units Data Source"
subcategory: ""
description: |-
  List onboarded cloud units (organization roots, OUs, accounts, subscriptions).
---

# blast_cloud_units (Data Source)

Lists every unit of every onboarded cloud organization, flattened. Use the IDs as `unit_ids` on `blast_project`.

## Example Usage

```terraform
data "blast_cloud_units" "prod_accounts" {
  type          = "ACCOUNT"
  name_contains = "prod"
}
```

## Argument Reference

All optional:

* `project_id`: only units of this project.
* `type`: unit type, e.g. `ACCOUNT` or `ORGANIZATIONAL_UNIT`.
* `platform_type`: e.g. `Amazon Web Services`.
* `name_contains`: case-insensitive substring of the unit name.
* `parent_id`: only direct children of this unit.

`type`, `platform_type`, `name_contains` and `parent_id` are applied by the provider after fetching the list.

## Attribute Reference

* `ids`: list of unit IDs.
* `units`: list of `id`, `name`, `type`, `platform_type`, `platform_org_id`, `monitoring_status`, `parent_id` (empty for an organization root).
