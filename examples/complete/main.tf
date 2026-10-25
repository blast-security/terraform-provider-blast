terraform {
  required_providers {
    blast = {
      source = "blast-security/blast"
    }
  }
}

# Credentials come from BLAST_CLIENT_ID / BLAST_CLIENT_SECRET.
provider "blast" {
  # project_id = "prj-123"   # optional; default is "All Cloud Units"
}

variable "pr_mode" {
  type    = string
  default = "task" # or "open_pr"
}

# ---------- Read: guardrails ----------
data "blast_guardrails" "aws_high" {
  platform_type = ["Amazon Web Services"]
  severity      = ["High", "Critical"]
  status        = "Active"
}

data "blast_guardrail" "s3_public" {
  catalog_id = "g-aws-s3-73735b0c"
}

# ---------- Exclusion rule ----------
resource "blast_exclusion" "break_glass" {
  name                  = "Break-glass account"
  note                  = "Security team emergency access"
  guardrail_catalog_ids = [data.blast_guardrail.s3_public.catalog_id]

  sources = [
    {
      identifier = "Account ID"
      operator   = "Equals"
      values     = ["123456789012"]
    }
  ]

  expiration_date = "2027-12-31"
}

data "blast_exclusions" "for_s3" {
  guardrail_id = data.blast_guardrail.s3_public.catalog_id
  depends_on   = [blast_exclusion.break_glass]
}

data "blast_exclusion" "break_glass" {
  id = blast_exclusion.break_glass.id
}

# ---------- Read: security controls ----------
data "blast_security_controls" "scps" {
  type = "Service Control Policy"
}

data "blast_security_control" "baseline" {
  id = "sc-1"
}

# ---------- "Custom guardrail": deploy catalog guardrails as a new SCP ----------
resource "blast_guardrail_deployment" "baseline_scp" {
  name                  = "Deploy high-severity AWS guardrails"
  guardrail_catalog_ids = data.blast_guardrails.aws_high.catalog_ids
  security_control_type = "Service Control Policy"
  security_control_name = "blast-high-sev-baseline"
  platform_unit_ids     = ["ou-abcd-11111111"]

  delivery_mode = var.pr_mode
  pull_request = {
    title       = "Blast: high-severity AWS guardrails"
    description = "Opened by Terraform"
  }
}

# ---------- Change an existing SCP ----------
resource "blast_security_control_change" "baseline_attach" {
  security_control_id = data.blast_security_control.baseline.id
  name                = "Attach baseline SCP to Dev OU"

  # Policy content: start from the deployed document and add a statement.
  policy = jsonencode(merge(jsondecode(data.blast_security_control.baseline.policy), {
    Statement = concat(jsondecode(data.blast_security_control.baseline.policy).Statement, [{
      Sid      = "DenyLeaveOrg"
      Effect   = "Deny"
      Action   = "organizations:LeaveOrganization"
      Resource = "*"
    }])
  }))

  attach_unit_ids = ["ou-abcd-22222222"]
  delivery_mode   = var.pr_mode
}

output "aws_high_guardrails" { value = data.blast_guardrails.aws_high.catalog_ids }
output "exclusion_id" { value = blast_exclusion.break_glass.id }
output "exclusions_for_s3" { value = [for e in data.blast_exclusions.for_s3.exclusions : e.name] }
output "scp_names" { value = [for c in data.blast_security_controls.scps.security_controls : c.name] }
output "deployment_task" {
  value = {
    id     = blast_guardrail_deployment.baseline_scp.id
    status = blast_guardrail_deployment.baseline_scp.status
    pr     = blast_guardrail_deployment.baseline_scp.terraform_pr_url
  }
}
output "control_change_task" {
  value = {
    id     = blast_security_control_change.baseline_attach.id
    status = blast_security_control_change.baseline_attach.status
    pr     = blast_security_control_change.baseline_attach.terraform_pr_url
  }
}
