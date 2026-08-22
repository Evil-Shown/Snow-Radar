# ADR-005: Remote Encrypted Terraform State & Secret Handling

- Status: Accepted
- Date: 2026-08-22
- Deciders: DevOps/SRE role
- Related: audit items #10 (plaintext secrets), #22 (no destroy protection)

## Context

State was `backend "local"`: tfvars held `hcloud_token` and OCI API key paths in plaintext; state files (which contain ALL variable values in plaintext) lived on disk unencrypted, unversioned, unlockable. Any laptop loss = cloud account compromise; any concurrent run = state corruption.

## Options considered

### Option A — Terraform Cloud (HCL-managed)
Managed state, locking, and RBAC out of the box; OIDC-friendly. Rejected for now: adds a third-party dependency to critical infra before launch, and TFC pricing/retention policy is another data point to defend in a privacy-focused threat model.

### Option B — S3-compatible backend + KMS encryption + DynamoDB-style locking (SELECTED)
Boring, proven, auditable. Hetzner doesn't provide object storage, so we use one provider-agnostic bucket (e.g., Cloudflare R2 or AWS S3) encrypted server-side, versioned, with native locking (`use_lockfile = true` on modern providers / DynamoDB table on AWS). Credentials supplied ONLY via environment variables — never written to disk in this repo.

### Option C — GitOps-encrypted state (sops-in-git)
Rejected: merge conflicts on state blobs are miserable and git history becomes a liability surface.

## Decision

Option B. Implementation contract:

1. `infra/terraform/main.tf` declares a **partial** backend: `backend "s3" {}`. Concrete values live in `backend.hcl` (gitignored), created from `backend.hcl.example`.
2. Provider credentials come from env vars only:
   - `TF_VAR_hcloud_token`, `TF_VAR_oci_tenancy_ocid`, `TF_VAR_oci_user_ocid`, `TF_VAR_oci_fingerprint`, `TF_VAR_oci_compartment_id`
   - AWS/R2 creds for the state bucket via standard AWS_* variables.
3. `terraform.tfvars` must never contain real credentials. The example file documents env-var usage.
4. State bucket: versioning ON, encryption ON, public access blocked, lifecycle: keep last 30 versions.
5. History hygiene: repo scanned for committed secrets at Phase 1 close (best-effort grep pass documented); `gitleaks` runs in CI from Phase 3 onward to prevent recurrence.

## Consequences

- `terraform init -reconfigure -backend-config=backend.hcl` required once per workstation (migration from local state documented).
- CI applies require short-lived cloud creds (GitHub Actions OIDC) — implemented in Phase 3.
