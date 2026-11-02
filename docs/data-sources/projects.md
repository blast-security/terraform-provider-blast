---
page_title: "blast_projects Data Source"
subcategory: ""
description: |-
  List Blast projects.
---

# blast_projects (Data Source)

## Example Usage

```terraform
data "blast_projects" "production" {
  type   = ["Production"]
  status = "Active"
}
```

## Argument Reference

All optional: `search` (name text search), `type` (list: Development, Test, Production, Default), `status` (Active or Inactive), `platform_type` (list).

## Attribute Reference

* `ids`: list of project IDs.
* `projects`: list of objects with the attributes of `blast_project` (`unit_ids` is empty in list results).
