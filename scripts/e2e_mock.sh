#!/usr/bin/env bash
# End-to-end lifecycle test of the provider against the in-memory mock API.
# Usage: scripts/e2e_mock.sh [terraform|tofu binary]   (default: terraform)
set -euo pipefail
TF=${1:-terraform}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
WORK=$(mktemp -d)
BIN="$WORK/bin"; mkdir -p "$BIN"
trap 'kill $MOCK_PID 2>/dev/null || true; rm -rf "$WORK"' EXIT

(cd "$ROOT" && go build -o "$BIN/terraform-provider-blast" . && go build -o "$WORK/mockserver" ./tools/mockserver)
"$WORK/mockserver" -addr 127.0.0.1:18089 >"$WORK/mock.log" 2>&1 & MOCK_PID=$!
sleep 1

cat >"$WORK/tfrc" <<RC
provider_installation {
  dev_overrides {
    "registry.terraform.io/blast-security/blast" = "$BIN"
    "registry.opentofu.org/blast-security/blast" = "$BIN"
  }
  direct {}
}
RC
export TF_CLI_CONFIG_FILE="$WORK/tfrc" TF_IN_AUTOMATION=1
export BLAST_API_URL=http://127.0.0.1:18089 BLAST_TOKEN_URL=http://127.0.0.1:18089/auth/v1/token
export BLAST_CLIENT_ID=test BLAST_CLIENT_SECRET=test NO_PROXY=127.0.0.1,localhost
cp "$ROOT/examples/complete/main.tf" "$WORK/"
cd "$WORK"

step() { echo; echo "=== $*"; }
expect_clean() {
  if ! "$TF" plan -no-color -detailed-exitcode "$@" >plan.out 2>&1; then
    cat plan.out; echo "FAIL: expected an empty plan"; exit 1
  fi
  echo "ok: no changes"
}
expect_plan() { # $1 = grep pattern that must appear in the plan
  local pat=$1; shift
  "$TF" plan -no-color "$@" >plan.out 2>&1 || true
  grep -q -- "$pat" plan.out || { cat plan.out; echo "FAIL: expected '$pat'"; exit 1; }
  echo "ok: plan contains '$pat'"
}

step "create (delivery_mode=task)"; "$TF" apply -auto-approve -no-color >/dev/null; expect_clean
step "switch to open_pr in place"; expect_plan "2 to change" -var pr_mode=open_pr
"$TF" apply -auto-approve -no-color -var pr_mode=open_pr >/dev/null; expect_clean -var pr_mode=open_pr
"$TF" output -json | grep -q 'github.com/acme/infra/pull' && echo "ok: PR URL recorded"

step "exclusion in-place update"
sed -i 's/Security team emergency access/Updated note/' main.tf
expect_plan "1 to change" -var pr_mode=open_pr
"$TF" apply -auto-approve -no-color -var pr_mode=open_pr >/dev/null; expect_clean -var pr_mode=open_pr

step "expiration change forces replace"
sed -i 's/2027-12-31/2028-06-30/' main.tf
expect_plan "expiration_date.*forces replacement" -var pr_mode=open_pr
"$TF" apply -auto-approve -no-color -var pr_mode=open_pr >/dev/null; expect_clean -var pr_mode=open_pr

step "second open control change -> 409 with hint"
cat > dup.tf <<'TF'
resource "blast_security_control_change" "dup" {
  security_control_id = "sc-1"
  policy              = data.blast_security_control.baseline.policy
}
TF
if "$TF" apply -auto-approve -no-color -var pr_mode=open_pr >apply.out 2>&1; then echo "FAIL: expected 409"; exit 1; fi
grep -q "already has an open control-change task" apply.out && echo "ok: 409 surfaced with hint"
rm dup.tf; "$TF" apply -auto-approve -no-color -var pr_mode=open_pr >/dev/null

step "import round-trip"
EXC=$("$TF" output -raw exclusion_id)
DEP=$("$TF" output -json deployment_task | sed -E 's/.*"id":"([^"]+)".*/\1/')
CC=$("$TF" output -json control_change_task | sed -E 's/.*"id":"([^"]+)".*/\1/')
for r in blast_exclusion.break_glass blast_guardrail_deployment.baseline_scp blast_security_control_change.baseline_attach; do
  "$TF" state rm -no-color "$r" >/dev/null
done
"$TF" import -no-color -var pr_mode=open_pr blast_exclusion.break_glass "$EXC" >/dev/null
"$TF" import -no-color -var pr_mode=open_pr blast_guardrail_deployment.baseline_scp "$DEP" >/dev/null
"$TF" import -no-color -var pr_mode=open_pr blast_security_control_change.baseline_attach "$CC" >/dev/null
"$TF" plan -no-color -var pr_mode=open_pr >plan.out 2>&1 || true
if grep -q "must be replaced" plan.out; then cat plan.out; echo "FAIL: import caused replacement"; exit 1; fi
echo "ok: imports do not force replacement"
"$TF" apply -auto-approve -no-color -var pr_mode=open_pr >/dev/null; expect_clean -var pr_mode=open_pr

step "back to task mode replaces task-backed resources"
expect_plan "2 to add, 0 to change, 2 to destroy"

step "destroy"
"$TF" destroy -auto-approve -no-color -var pr_mode=open_pr >/dev/null
grep -c "DELETE /api/v1/external/" "$WORK/mock.log" | xargs echo "ok: DELETE calls:"

# ---------- automation rules (examples/automation-rules) ----------
mkdir "$WORK/automation" && cp "$ROOT/examples/automation-rules/main.tf" "$WORK/automation/" && cd "$WORK/automation"
edit() { perl -pi -e "$1" main.tf; } # portable in-place edit (macOS and Linux)

step "automation rules: create, then no drift from API defaults (# in channel, EQUAL, 180 days)"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "automation rules: in-place update"
edit 's/Blocked by Blast/Blocked by Blast, see the console/'
expect_plan "1 to change"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "automation rules: trigger change forces replace"
edit 's/"COVERAGE_DECREASED"/"NEW_GUARDRAILS_ADDED"/'
expect_plan "trigger.*forces replacement"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "automation rules: a field the trigger does not offer is rejected by the API, with every problem listed"
cat > bad.tf <<'TF'
resource "blast_automation_rule" "bad" {
  name       = "bad"
  trigger    = "COVERAGE_DECREASED"
  conditions = { security_control_ids = { values = ["sc-1"] } }
  slack      = { channel = "#x" }
}
TF
if "$TF" apply -auto-approve -no-color >apply.out 2>&1; then cat apply.out; echo "FAIL: expected a 400"; exit 1; fi
tr "\n" " " < apply.out | grep -q "conditions: not *offered by *COVERAGE_DECREASED" || { cat apply.out; echo "FAIL: API error not surfaced"; exit 1; }
echo "ok: API error surfaced"
rm bad.tf

step "automation rules: import round-trip"
RULE=$("$TF" output -raw blocked_to_channel_id)
"$TF" state rm -no-color blast_automation_rule.blocked_to_channel >/dev/null
"$TF" import -no-color blast_automation_rule.blocked_to_channel "$RULE" >/dev/null
"$TF" plan -no-color >plan.out 2>&1 || true
if grep -q "must be replaced" plan.out; then cat plan.out; echo "FAIL: import caused replacement"; exit 1; fi
echo "ok: import does not force replacement"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "automation rules: destroy"
"$TF" destroy -auto-approve -no-color >/dev/null
grep -c "DELETE /api/v1/external/automations/" "$WORK/mock.log" | xargs echo "ok: automation DELETE calls:"

# ---------- data sources (examples/data-sources) ----------
mkdir "$WORK/ds" && cp "$ROOT/examples/data-sources/main.tf" "$WORK/ds/" && cd "$WORK/ds"

step "data sources: create rule and read back via data sources"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "data sources: blast_automation_rule matches created rule"
RULE_ID=$("$TF" output -raw automation_rule_id)
[ -n "$RULE_ID" ] || { echo "FAIL: blast_automation_rule.id is empty"; exit 1; }
echo "ok: blast_automation_rule.id=$RULE_ID"

step "data sources: blast_automation_rules returns at least one rule"
COUNT=$("$TF" output -raw automation_rules_count)
[ "$COUNT" -ge 1 ] || { echo "FAIL: blast_automation_rules returned no rules"; exit 1; }
echo "ok: blast_automation_rules.rules count=$COUNT"

step "data sources: active = false filters out the (active) rule"
DISABLED=$("$TF" output -raw disabled_rules_count)
[ "$DISABLED" -eq 0 ] || { echo "FAIL: active = false returned $DISABLED rules"; exit 1; }
echo "ok: active = false returned 0 rules"

step "data sources: blast_gap reads pre-seeded gap"
GAP_NAME=$("$TF" output -raw gap_name)
[ -n "$GAP_NAME" ] || { echo "FAIL: blast_gap.name is empty"; exit 1; }
echo "ok: blast_gap.name=$GAP_NAME"

step "data sources: blast_gaps list data source returns at least the seed gap"
# The data-sources example has no blast_gaps usage; we probe it inline from this directory.
cat > gaps_ds.tf <<'TF'
data "blast_gaps" "all" {}
output "gaps_all_count" { value = length(data.blast_gaps.all.gaps) }
TF
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean
GAPS_COUNT=$("$TF" output -raw gaps_all_count)
[ "$GAPS_COUNT" -ge 1 ] || { echo "FAIL: blast_gaps returned no gaps (expected seed gap)"; exit 1; }
echo "ok: blast_gaps.gaps count=$GAPS_COUNT"
rm gaps_ds.tf

step "data sources: destroy"
"$TF" destroy -auto-approve -no-color >/dev/null

# ---------- gap resource (examples/gaps) ----------
mkdir "$WORK/gaps" && cd "$WORK/gaps"
cat > main.tf <<'TF'
terraform {
  required_providers { blast = { source = "blast-security/blast" } }
}
provider "blast" { project_id = "prj-test" }

resource "blast_gap" "test" {
  project_id     = "prj-test"
  name           = "IAM unguarded"
  category       = "missing_defense"
  type           = "CONFIGURATION_RESTRICTION"
  priority_score = 80
  priority_level = "high"
  is_quick_win   = false
  mitigation = {
    description = "Deploy a guardrail."
  }
  priority_indicators = {
    effective_exposure    = { count = 5, weight = 20, max_weight = 50 }
    suspicious_activities = { count = 0, weight = 0, max_weight = 20 }
    violations_found      = { count = 2, weight = 10, max_weight = 30 }
    threat_intel          = { count = 0, weight = 0, max_weight = 10 }
  }
}

data "blast_gaps" "open" {
  project_id = "prj-test"
  tab        = "open"
  depends_on = [blast_gap.test]
}

output "gap_id"          { value = blast_gap.test.id }
output "gap_status"      { value = blast_gap.test.status }
output "gap_gain_points" { value = blast_gap.test.gain_points }
output "gaps_count"      { value = length(data.blast_gaps.open.gaps) }
TF

step "gaps: create"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean
GAP_ID=$("$TF" output -raw gap_id)
[ -n "$GAP_ID" ] || { echo "FAIL: blast_gap.id is empty"; exit 1; }
echo "ok: blast_gap.id=$GAP_ID"

step "gaps: defaults filled by API (status=open, gain_points=0)"
STATUS=$("$TF" output -raw gap_status)
[ "$STATUS" = "open" ] || { echo "FAIL: expected status=open, got $STATUS"; exit 1; }
echo "ok: status=$STATUS"
GAIN=$("$TF" output -raw gap_gain_points)
[ "$GAIN" = "0" ] || { echo "FAIL: expected gain_points=0, got $GAIN"; exit 1; }
echo "ok: gain_points=$GAIN"

step "gaps: blast_gaps data source returns created gap"
COUNT=$("$TF" output -raw gaps_count)
[ "$COUNT" -ge 1 ] || { echo "FAIL: blast_gaps returned no gaps"; exit 1; }
echo "ok: blast_gaps.gaps count=$COUNT"

step "gaps: in-place update (name change)"
perl -pi -e 's/IAM unguarded/IAM unguarded - updated/' main.tf
expect_plan "1 to change"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "gaps: priority_level change in place"
perl -pi -e 's/priority_level = "high"/priority_level = "critical"/' main.tf
expect_plan "1 to change"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "gaps: import round-trip"
"$TF" state rm -no-color blast_gap.test >/dev/null
"$TF" import -no-color blast_gap.test "prj-test/$GAP_ID" >/dev/null
"$TF" plan -no-color >plan.out 2>&1 || true
if grep -q "must be replaced" plan.out; then cat plan.out; echo "FAIL: import caused replacement"; exit 1; fi
echo "ok: import does not force replacement"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "gaps: project_id change forces replacement"
perl -pi -e 's/project_id\s*= "prj-test"/project_id = "prj-other"/' main.tf
expect_plan "forces replacement"
# Revert — do not actually apply the replacement
perl -pi -e 's/project_id\s*= "prj-other"/project_id = "prj-test"/' main.tf

step "gaps: destroy"
"$TF" destroy -auto-approve -no-color >/dev/null
grep -c "DELETE /api/v1/external/gaps/" "$WORK/mock.log" | xargs echo "ok: gap DELETE calls:"

echo; echo "ALL E2E CHECKS PASSED"
