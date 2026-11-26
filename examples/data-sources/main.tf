terraform {
  required_providers {
    blast = { source = "blast-security/blast" }
  }
}

# Credentials come from BLAST_CLIENT_ID / BLAST_CLIENT_SECRET.
provider "blast" {}

# Create an automation rule and read it back via the data source.
resource "blast_automation_rule" "coverage" {
  name    = "Coverage dropped"
  trigger = "COVERAGE_DECREASED"
  slack   = { channel = "#sec-data-sources-test" }
}

data "blast_automation_rule" "coverage" {
  id = blast_automation_rule.coverage.id
}

data "blast_automation_rules" "all" {
  depends_on = [blast_automation_rule.coverage]
}

# active = false is a real filter (disabled rules only); leaving it unset returns both.
data "blast_automation_rules" "disabled" {
  active     = false
  depends_on = [blast_automation_rule.coverage]
}

# Gaps are project-scoped: the API rejects a request without a specific project.
data "blast_gap" "example" {
  id         = "gap-seed-1"
  project_id = "prj-default"
}

data "blast_gaps" "open" {
  project_id = "prj-default"
  tab        = "open"
}

output "automation_rule_id" {
  value = data.blast_automation_rule.coverage.id
}

output "automation_rules_count" {
  value = length(data.blast_automation_rules.all.rules)
}

output "disabled_rules_count" {
  value = length(data.blast_automation_rules.disabled.rules)
}

output "gaps_count" {
  value = length(data.blast_gaps.open.gaps)
}

output "gap_name" {
  value = data.blast_gap.example.name
}
