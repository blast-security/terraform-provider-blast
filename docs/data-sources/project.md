---
page_title: "blast_project Data Source"
subcategory: ""
description: |-
  Look up a Blast project by ID or name.
---

# blast_project (Data Source)

## Example Usage

```terraform
data "blast_project" "default" {
  name = "All Cloud Units"
}

data "blast_exclusions" "default_project" {
  project_id = data.blast_project.default.id
}
```

## Argument Reference

Set exactly one of:

* `id` (Optional) Project ID.
* `name` (Optional) Exact project name, case-insensitive.

## Attribute Reference

`id`, `name`, `description`, `status`, `default`, `types`, `platform_types`, `number_of_units`, `unit_ids`.
