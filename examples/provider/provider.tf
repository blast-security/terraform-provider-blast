terraform {
  required_providers {
    blast = {
      source  = "blast-security/blast"
      version = "~> 0.2"
    }
  }
}

# Prefer environment variables in CI:
#   export BLAST_CLIENT_ID=...  BLAST_CLIENT_SECRET=...
provider "blast" {
  # client_id     = var.blast_client_id
  # client_secret = var.blast_client_secret
  # project_id    = "prj-123"   # optional, defaults to "All Cloud Units"
}
