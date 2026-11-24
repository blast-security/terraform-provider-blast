terraform {
  required_providers {
    blast = { source = "blast-security/blast" }
  }
}

# Credentials from BLAST_CLIENT_ID / BLAST_CLIENT_SECRET.
# project_id is required for gaps (the all-units scope is rejected by the API).
provider "blast" {
  project_id = var.project_id
}

variable "project_id" {
  description = "Blast project ID. Gaps are project-scoped — a specific project is required."
  type        = string
}

resource "blast_gap" "iam_unguarded" {
  project_id = var.project_id
  name       = "IAM role permission changes going unguarded"
  description = "IAM permission changes are not covered by an active guardrail."
  category   = "missing_defense"
  type       = "CONFIGURATION_RESTRICTION"

  priority_score = 85
  priority_level = "high"
  is_quick_win   = false

  status      = "open"
  gain_points = 5

  priority_indicators = {
    effective_exposure = {
      count      = 10
      weight     = 30
      max_weight = 50
    }
    suspicious_activities = {
      count      = 0
      weight     = 0
      max_weight = 20
    }
    violations_found = {
      count      = 3
      weight     = 15
      max_weight = 30
    }
    threat_intel = {
      count      = 0
      weight     = 0
      max_weight = 10
    }
  }

  mitigation = {
    description   = "Deploy a guardrail that blocks IAM role permission changes."
    guardrail_name = "Block IAM Role Permission Changes"
  }
}

data "blast_gaps" "open_high" {
  project_id     = var.project_id
  tab            = "open"
  priority_level = ["high", "critical"]
  depends_on     = [blast_gap.iam_unguarded]
}

output "gap_id" {
  value = blast_gap.iam_unguarded.id
}

output "open_high_gaps_count" {
  value = length(data.blast_gaps.open_high.gaps)
}
