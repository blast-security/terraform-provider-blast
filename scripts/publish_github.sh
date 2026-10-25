#!/usr/bin/env bash
# One-time: create the public GitHub repo blast-security/terraform-provider-blast and push this code.
# Run from anywhere:  ~/projects/terraform-provider-blast/scripts/publish_github.sh
set -euo pipefail
OWNER=blast-security
REPO=terraform-provider-blast
cd "$(dirname "$0")/.."

command -v git >/dev/null || { echo "git is not installed (xcode-select --install)"; exit 1; }

if [ -d .git ]; then echo "This folder is already a git repo; aborting so nothing is overwritten."; exit 1; fi

# Generate go.sum if Go is available (CI also does this).
if command -v go >/dev/null; then echo "== go mod tidy"; go mod tidy; else echo "(Go not installed: skipping go.sum; CI will generate it)"; fi

echo "== creating local repo"
git init -q -b main
git add -A
echo "Files to be published:"; git ls-files | sed 's/^/   /'

# Safety: never publish credentials or local build artefacts.
if [ -n "${BLAST_CLIENT_SECRET:-}" ] && git grep -q -F -- "$BLAST_CLIENT_SECRET"; then
  echo "ABORT: your BLAST_CLIENT_SECRET appears in a file."; rm -rf .git; exit 1
fi
if [ -n "${BLAST_CLIENT_ID:-}" ] && git grep -q -F -- "$BLAST_CLIENT_ID"; then
  echo "ABORT: your BLAST_CLIENT_ID appears in a file."; rm -rf .git; exit 1
fi
if git ls-files | grep -E '^(dist|bin)/|dev\.tfrc|\.tfstate|\.tgz$'; then
  echo "ABORT: build artefacts or local config would be published."; rm -rf .git; exit 1
fi

git commit -q -m "Initial release of the Blast Terraform provider (preview)"
echo "== committed"

if command -v gh >/dev/null && gh auth status >/dev/null 2>&1; then
  gh repo create "$OWNER/$REPO" --public --source . --push \
    --description "Terraform provider for Blast Security" \
    --homepage "https://docs.blast.security/guides/blast-platform/api/terraform-provider"
  echo; echo "Done: https://github.com/$OWNER/$REPO"
else
  cat <<MSG

GitHub CLI not found or not logged in. Finish in two steps:
  1. Create an EMPTY public repo at https://github.com/organizations/$OWNER/repositories/new
     name: $REPO   (no README, no license, no .gitignore)
  2. Then run:
     cd "$(pwd)"
     git remote add origin https://github.com/$OWNER/$REPO.git
     git push -u origin main
(Or install the CLI with 'brew install gh', run 'gh auth login', delete the .git folder and re-run this script.)
MSG
fi
