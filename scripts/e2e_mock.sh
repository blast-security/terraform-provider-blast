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

step "automation rules: bare-ID import + explicit project_id equal to the provider default plans no replacement"
"$TF" state rm -no-color blast_automation_rule.blocked_to_channel >/dev/null
BLAST_PROJECT_ID=prj-e2e "$TF" import -no-color blast_automation_rule.blocked_to_channel "$RULE" >/dev/null
edit 's/(trigger = "BLOCKED_ACTIVITY")/$1\n  project_id = "prj-e2e"/'
BLAST_PROJECT_ID=prj-e2e "$TF" plan -no-color >plan.out 2>&1 || true
if grep -q "blocked_to_channel must be replaced" plan.out; then cat plan.out; echo "FAIL: explicit project_id after bare import forced replacement"; exit 1; fi
echo "ok: no replacement"
grep -q "blocked_with_exclusion_requests must be replaced" plan.out || { cat plan.out; echo "FAIL: a rule created in All Cloud Units must be replaced when its effective project changes"; exit 1; }
echo "ok: a real project change still replaces"
edit 's/\n  project_id = "prj-e2e"//; s/^  project_id = "prj-e2e"\n//'

step "automation rules: destroy"
"$TF" destroy -auto-approve -no-color >/dev/null
grep -c "DELETE /api/v1/external/automations/" "$WORK/mock.log" | xargs echo "ok: automation DELETE calls:"
# ---------- projects (examples/projects; tenant-scoped, no x-project-id) ----------
cd "$WORK" && mkdir "$WORK/projects" && cp "$ROOT/examples/projects/main.tf" "$WORK/projects/" && cd "$WORK/projects"

step "projects: create, then no drift"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "projects: in-place update"
perl -pi -e 's/managed by Terraform/managed by Terraform (e2e)/' main.tf
expect_plan "1 to change"
"$TF" apply -auto-approve -no-color >/dev/null; expect_clean

step "projects: import round-trip"
PRJ=$("$TF" output -json project | sed -E 's/.*"id":"([^"]+)".*/\1/')
"$TF" state rm -no-color blast_project.production >/dev/null
"$TF" import -no-color blast_project.production "$PRJ" >/dev/null
expect_clean

step "projects: destroy"
"$TF" destroy -auto-approve -no-color >/dev/null
grep -c "DELETE /api/v1/external/projects/" "$WORK/mock.log" | xargs echo "ok: project DELETE calls:"
echo; echo "ALL E2E CHECKS PASSED"
