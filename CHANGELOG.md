## 0.2.0 (Unreleased)

FEATURES:

* **New Resource:** `blast_project` (create, update in place, delete, import). Requires Blast External API 1.5.
* **New Data Sources:** `blast_project`, `blast_projects`, `blast_cloud_units`

ENHANCEMENTS:

* `blast_exclusion`: `identifier` and `operator` are no longer limited to a fixed list in the provider; the Blast API validates them. Enables identifiers such as `VPC ID` (`aws:SourceVpc`) and `VPC Endpoint ID` (`aws:SourceVpce`) where your tenant supports them.

## 0.1.1 (October 25, 2026)

DOCUMENTATION:

* One Registry page per data source (`blast_exclusion`, `blast_guardrail` and `blast_security_control` were missing).

## 0.1.0 (October 25, 2026)

FEATURES:

* **New Resource:** `blast_exclusion`
* **New Resource:** `blast_guardrail_deployment` (delivery modes `task` and `open_pr`)
* **New Resource:** `blast_security_control_change` (SCP / RCP; delivery modes `task` and `open_pr`)
* **New Data Sources:** `blast_exclusion`, `blast_exclusions`, `blast_guardrail`, `blast_guardrails`, `blast_security_control`, `blast_security_controls`
