# Step-by-step live test of the write resources against a real tenant.
# Every step is off by default; turn them on one at a time with -var flags.
# Required: -var guardrail_catalog_id=... -var security_control_name_contains=...
# Everything this creates is a test object that `destroy` removes again.
#
#   step 0  project          -var step_project=true [-var project_unit_name=<test account name>]
#   step 1  exclusion        -var step_exclusion=true
#   step 2  guardrail task   -var step_deployment=true        (task only; nothing reaches AWS)
#   step 3  control change   -var step_control_change=true    (task only; nothing reaches AWS)
#   step 4  open a PR        add -var pr_mode=open_pr to step 2/3 (needs a git integration in Blast)

terraform {
  required_providers {
    blast = { source = "blast-security/blast" }
  }
}

provider "blast" {}

variable "step_exclusion" {
  type    = bool
  default = false
}
variable "step_deployment" {
  type    = bool
  default = false
}
variable "step_project" {
  type    = bool
  default = false
}
variable "project_unit_name" {
  description = "Name (or part of it) of ONE test account/OU to put in the test project (step 0)."
  type        = string
  default     = ""
}
variable "step_control_change" {
  type    = bool
  default = false
}
variable "pr_mode" {
  type    = string
  default = "task"
}

variable "guardrail_catalog_id" {
  description = "Guardrail catalog ID used by the test exclusion and test deployment (see data.blast_guardrails)."
  type        = string
}
variable "security_control_name_contains" {
  description = "Name (or part of it) of a TEST SCP used for the control-change and deployment steps. Never point this at a production SCP."
  type        = string
}
variable "test_identifier" {
  description = "Exclusion identifier to test, e.g. \"Account ID\", \"VPC ID\" (aws:SourceVpc) or \"VPC Endpoint ID\" (aws:SourceVpce)."
  type        = string
  default     = "Account ID"
}
variable "test_operator" {
  type    = string
  default = "Like"
}
variable "test_value" {
  description = "Value for the test exclusion (a fake account ID, vpc-..., or vpce-...)."
  type        = string
  default     = "000000000000"
}

locals {
  tag = "tf-provider-test"
}

data "blast_guardrail" "target" {
  catalog_id = var.guardrail_catalog_id
}

data "blast_security_controls" "test_scp" {
  type          = "Service Control Policy"
  name_contains = var.security_control_name_contains
}

data "blast_security_control" "test_scp" {
  count = length(data.blast_security_controls.test_scp.ids) > 0 ? 1 : 0
  id    = data.blast_security_controls.test_scp.ids[0]
}

locals {
  test_scp   = try(data.blast_security_control.test_scp[0], null)
  test_units = try([for r in local.test_scp.platform_resources : r.resource_id], [])
}

# ---------- step 0: project ----------
data "blast_cloud_units" "project_test" {
  count         = var.step_project ? 1 : 0
  name_contains = var.project_unit_name
}

resource "blast_project" "test" {
  count       = var.step_project ? 1 : 0
  name        = "${local.tag} project"
  description = "Created by the Terraform provider live test; safe to delete."
  types       = ["Test"]
  unit_ids    = var.project_unit_name == "" ? [] : slice(data.blast_cloud_units.project_test[0].ids, 0, 1)
}

output "project" {
  value = try({
    id     = blast_project.test[0].id
    status = blast_project.test[0].status
    units  = [for u in blast_project.test[0].units : "${u.id} (${u.name})"]
  }, null)
}

# ---------- step 1 ----------
resource "blast_exclusion" "test" {
  count                 = var.step_exclusion ? 1 : 0
  name                  = "${local.tag} exclusion"
  note                  = "Created by the Terraform provider live test; safe to delete."
  guardrail_catalog_ids = [data.blast_guardrail.target.catalog_id]
  sources = [{
    identifier = var.test_identifier
    operator   = var.test_operator
    values     = [var.test_value]
  }]
  expiration_date = "2027-01-31"
}

# ---------- step 2 ----------
resource "blast_guardrail_deployment" "test" {
  count                 = var.step_deployment ? 1 : 0
  name                  = "${local.tag} guardrail deployment"
  guardrail_catalog_ids = [data.blast_guardrail.target.catalog_id]
  security_control_type = "Service Control Policy"
  security_control_name = "${local.tag}-scp"
  platform_unit_ids     = local.test_units
  delivery_mode         = var.pr_mode
  pull_request = {
    title       = "[TEST] ${local.tag} guardrail deployment"
    description = "Opened by the Blast Terraform provider live test. Do not merge."
  }

  lifecycle {
    precondition {
      condition     = length(local.test_units) > 0
      error_message = "No org units found on the test SCP; set security_control_name_contains to an SCP that is attached somewhere."
    }
  }
}

# ---------- step 3 ----------
resource "blast_security_control_change" "test" {
  count               = var.step_control_change ? 1 : 0
  name                = "${local.tag} control change (no-op)"
  security_control_id = local.test_scp.id
  policy              = local.test_scp.policy # unchanged content: a harmless no-op change
  delivery_mode       = var.pr_mode
  pull_request = {
    title       = "[TEST] ${local.tag} control change"
    description = "Opened by the Blast Terraform provider live test. Do not merge."
  }

  lifecycle {
    precondition {
      condition     = local.test_scp != null
      error_message = "No SCP matched security_control_name_contains."
    }
  }
}

output "test_scp" { value = try("${local.test_scp.id}  ${local.test_scp.name}  units=${jsonencode(local.test_units)}", "none") }
output "exclusion" { value = try({ id = blast_exclusion.test[0].id, platform = blast_exclusion.test[0].platform_type, expired = blast_exclusion.test[0].is_expired }, null) }
output "deployment" { value = try({ id = blast_guardrail_deployment.test[0].id, status = blast_guardrail_deployment.test[0].status, pr = blast_guardrail_deployment.test[0].terraform_pr_url }, null) }
output "control_change" { value = try({ id = blast_security_control_change.test[0].id, status = blast_security_control_change.test[0].status, pr = blast_security_control_change.test[0].terraform_pr_url }, null) }
