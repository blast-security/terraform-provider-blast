# Read-only smoke test against a real tenant: makes no changes.
#   export BLAST_CLIENT_ID=... BLAST_CLIENT_SECRET=...
#   terraform plan
terraform {
  required_providers {
    blast = { source = "blast-security/blast" }
  }
}

provider "blast" {}

data "blast_guardrails" "aws_active" {
  platform_type = ["Amazon Web Services"]
  status        = "Active"
}

data "blast_security_controls" "scps" {
  type = "Service Control Policy"
}

data "blast_exclusions" "all" {}

data "blast_guardrail" "first" {
  count      = length(data.blast_guardrails.aws_active.catalog_ids) > 0 ? 1 : 0
  catalog_id = data.blast_guardrails.aws_active.catalog_ids[0]
}

data "blast_security_control" "first_scp" {
  count = length(data.blast_security_controls.scps.ids) > 0 ? 1 : 0
  id    = data.blast_security_controls.scps.ids[0]
}

output "active_aws_guardrails" { value = length(data.blast_guardrails.aws_active.guardrails) }
output "scps" { value = [for c in data.blast_security_controls.scps.security_controls : "${c.id}  ${c.name}"] }
output "exclusions" { value = [for e in data.blast_exclusions.all.exclusions : "${e.id}  ${e.name}  ${e.platform_type}"] }
output "first_guardrail" { value = try(data.blast_guardrail.first[0].name, null) }
output "first_scp_policy_bytes" { value = try(length(data.blast_security_control.first_scp[0].policy), null) }
