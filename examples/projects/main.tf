terraform {
  required_providers {
    blast = { source = "blast-security/blast" }
  }
}

provider "blast" {}

variable "prod_ou_name" {
  type    = string
  default = "Prod"
}
variable "status" {
  type    = string
  default = "Active"
}
variable "extra_units" {
  type    = list(string)
  default = []
}

# Find the cloud units to put in the project.
data "blast_cloud_units" "prod_ou" {
  type          = "ORGANIZATIONAL_UNIT"
  name_contains = var.prod_ou_name
}

data "blast_cloud_units" "all_accounts" {
  type = "ACCOUNT"
}

resource "blast_project" "production" {
  name        = "Production"
  description = "Production accounts, managed by Terraform"
  types       = ["Production"]
  unit_ids    = concat(data.blast_cloud_units.prod_ou.ids, var.extra_units)
  status      = var.status
}

# Scope other Blast resources and data sources to the project.
data "blast_guardrails" "in_production" {
  project_id = blast_project.production.id
  status     = "Active"
}

data "blast_project" "default" {
  name = "All Cloud Units"
}

data "blast_projects" "all" {
  depends_on = [blast_project.production]
}

output "project" {
  value = {
    id       = blast_project.production.id
    status   = blast_project.production.status
    units    = [for u in blast_project.production.units : "${u.id} (${u.name})"]
    count    = blast_project.production.number_of_units
    platform = blast_project.production.platform_types
  }
}
output "default_project_id" { value = data.blast_project.default.id }
output "all_projects" { value = [for p in data.blast_projects.all.projects : "${p.name}${p.default ? " (default)" : ""}"] }
output "account_ids" { value = data.blast_cloud_units.all_accounts.ids }
