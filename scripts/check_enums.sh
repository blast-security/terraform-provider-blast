#!/usr/bin/env bash
# Prints the exclusion identifier/operator values the live Blast API accepts.
# Usage: BLAST_CLIENT_ID=... BLAST_CLIENT_SECRET=... scripts/check_enums.sh [filter]
set -euo pipefail
FILTER=${1:-identifier|operator|exclusion}
TOKEN=$(curl -sS -X POST "${BLAST_TOKEN_URL:-https://auth.blast.security/auth/v1/token}" \
  --data-urlencode grant_type=client_credentials \
  --data-urlencode "client_id=${BLAST_CLIENT_ID:?}" \
  --data-urlencode "client_secret=${BLAST_CLIENT_SECRET:?}" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')
curl -sS "${BLAST_API_URL:-https://api.blast.security}/api/v1/external/enums" -H "Authorization: Bearer $TOKEN" \
  | python3 -c '
import sys, json, re
data = json.load(sys.stdin)
pat = re.compile(sys.argv[1], re.I)
def walk(node, path=""):
    if isinstance(node, dict):
        for k, v in node.items():
            walk(v, f"{path}.{k}" if path else k)
    elif isinstance(node, list):
        if pat.search(path):
            print(f"\n{path}:")
            for item in node:
                print("  -", item if not isinstance(item, dict) else json.dumps(item))
        else:
            for i, v in enumerate(node):
                walk(v, path)
walk(data)
' "$FILTER"
