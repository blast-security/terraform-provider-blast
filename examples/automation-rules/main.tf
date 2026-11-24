terraform {
  required_providers {
    blast = {
      source = "blast-security/blast"
    }
  }
}

# Credentials come from BLAST_CLIENT_ID / BLAST_CLIENT_SECRET.
provider "blast" {}

variable "alerts_channel" {
  type    = string
  default = "#sec-alerts"
}

# Blocked activity of chosen security controls, to a channel.
resource "blast_automation_rule" "blocked_to_channel" {
  name    = "Blocked activity to sec-alerts"
  trigger = "BLOCKED_ACTIVITY"

  conditions = {
    security_control_ids = { values = ["sc-1"] } # operator defaults to EQUAL
  }

  slack = {
    channel     = var.alerts_channel
    custom_text = "Blocked by Blast"
  }
}

# Blocked activity to the people your notification routing rules pick, with an exclusion request button.
resource "blast_automation_rule" "blocked_with_exclusion_requests" {
  name    = "Blocked activity with exclusion requests"
  trigger = "BLOCKED_ACTIVITY"

  slack = {
    use_notification_routing_rules = true
    allow_exclusion_request        = true
    allow_request_for              = ["g-aws-s3-73735b0c"]
    max_exclusion_duration_days    = 30
  }
}

resource "blast_automation_rule" "expiring_exclusions" {
  name    = "Expiring exclusions"
  trigger = "EXCLUSION_ABOUT_TO_EXPIRE"

  slack = {
    channel                    = var.alerts_channel
    notify_requester_on_expiry = true
  }
}

# Exclusion requests to Slack and Jira.
resource "blast_automation_rule" "exclusion_requests" {
  name    = "Exclusion requests"
  trigger = "EXCLUSION_REQUEST_CREATED"

  slack = {
    channel          = var.alerts_channel
    notify_on_review = true
  }

  jira = {
    project          = "SEC"
    issue_type       = "Task"
    summary          = "Exclusion request"
    update_on_review = true
  }
}

resource "blast_automation_rule" "coverage_dropped" {
  name    = "Coverage dropped"
  trigger = "COVERAGE_DECREASED"
  active  = false # created disabled

  slack = {
    channel = var.alerts_channel
  }
}

output "blocked_to_channel_id" {
  value = blast_automation_rule.blocked_to_channel.id
}
