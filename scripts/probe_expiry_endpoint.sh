#!/usr/bin/env bash
# Safe probe of the internal bulk expiry endpoint: sends an EMPTY body, which changes nothing.
# Validation errors (400) reveal the expected fields; 401/403 tells us if API client tokens are accepted.
# Usage: BLAST_CLIENT_ID=... BLAST_CLIENT_SECRET=... scripts/probe_expiry_endpoint.sh
set -uo pipefail
API=${BLAST_API_URL:-https://api.blast.security}
TOKEN=$(curl -sS -X POST "${BLAST_TOKEN_URL:-https://auth.blast.security/auth/v1/token}" \
  --data-urlencode grant_type=client_credentials \
  --data-urlencode "client_id=${BLAST_CLIENT_ID:?}" \
  --data-urlencode "client_secret=${BLAST_CLIENT_SECRET:?}" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])') || { echo "token request failed"; exit 1; }

for path in /api/v1/exclusions/bulk/expiry-date /api/v1/external/exclusions/bulk/expiry-date; do
  for m in PATCH PUT POST; do
    out=$(curl -sS -m 20 -X "$m" "$API$path" -H "Authorization: Bearer $TOKEN" \
          -H "Content-Type: application/json" -d '{}' -w '\n%{http_code}')
    code=$(printf '%s' "$out" | tail -n1); body=$(printf '%s' "$out" | sed '$d' | cut -c1-400)
    printf '%-6s %-45s HTTP %s  %s\n' "$m" "$path" "$code" "$body"
  done
done
