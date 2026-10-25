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
echo; echo "ALL E2E CHECKS PASSED"
