#!/usr/bin/env bash
# Tries the Blast token endpoint a few ways and prints only HTTP status codes (never the token or secret).
# Usage: BLAST_CLIENT_ID=... BLAST_CLIENT_SECRET=... scripts/auth_check.sh
URL=${BLAST_TOKEN_URL:-https://auth.blast.security/auth/v1/token}
ID=${BLAST_CLIENT_ID:?set BLAST_CLIENT_ID}; SECRET=${BLAST_CLIENT_SECRET:?set BLAST_CLIENT_SECRET}
echo "id length=${#ID} secret length=${#SECRET} "
DECODED_ID=$(printf '%s' "$ID" | base64 -d 2>/dev/null | LC_ALL=C cut -d: -f1)

try() { # label, curl args...
  local label=$1; shift
  local out code
  out=$(curl -sS -m 20 -X POST "$URL" "$@" -w '\n%{http_code}')
  code=$(printf '%s' "$out" | tail -n1)
  body=$(printf '%s' "$out" | sed '$d' | sed -E 's/"(access_token|id_token|refresh_token)" *: *"[^"]*"/"\1":"<redacted>"/g' | cut -c1-200)
  printf '%-45s HTTP %s  %s\n' "$label" "$code" "$body"
}

try "A: form body, id+secret as given" --data-urlencode grant_type=client_credentials --data-urlencode "client_id=$ID" --data-urlencode "client_secret=$SECRET"
try "B: Basic auth header, id+secret" -u "$ID:$SECRET" --data-urlencode grant_type=client_credentials
try "C: Basic header = id as-is (pre-encoded)" -H "Authorization: Basic $ID" --data-urlencode grant_type=client_credentials
[ -n "$DECODED_ID" ] && try "D: form body, base64-decoded id + secret" --data-urlencode grant_type=client_credentials --data-urlencode "client_id=$DECODED_ID" --data-urlencode "client_secret=$SECRET"
