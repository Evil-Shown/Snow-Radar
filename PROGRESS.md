# PROGRESS.md — institutional memory

> Updated every session. Treat as the resume point for future agents.

## Status: ALL MACHINE-SIDE WORK COMPLETE
Phases 0–5 code-complete. Zero-trust audit executed (22 findings closed).
Everything verifiable on this machine is verified with command output.

## Verification evidence (this machine)
| Check | Evidence |
|---|---|
| Go build/vet | BUILD_OK / VET_OK (go1.22.6 → go.mod 1.25) |
| Go tests | `go test ./... -race -count=1` ALL PASS, 5 packages |
| Terraform | fmt clean; `validate` = "configuration is valid" (v1.9.8); init OK |
| Shell | bash -n / sh -n clean on all phase scripts |
| Secret history | pattern scan clean; gitleaks in CI |

## Human gates — exact commands, in order
1. **Push & watch CI** (first real Actions run):
   `git push origin main` → check api.yml/client.yml/infra.yml green
2. **Disposable node validation** (Hetzner CX11, ~€0.01/hr):
   ```
   cd infra/ansible && cp inventory.yml.example inventory.yml  # fill IP
   ansible-playbook site.yml --limit fsn
   # capture: wg show wg0 && awg show awg0 && ss -ulnp | grep 5182
   nmap -sU -p 51820,51821 <ip>   # from OUTSIDE: both open
   nmap -p 9100 <ip>              # from OUTSIDE: closed
   ```
3. **Terraform apply**: create backend.hcl + TF_VAR_* env per docs/adr/005-remote-state-secrets.md → `terraform init -reconfigure -backend-config=backend.hcl && terraform apply`
4. **End-to-end tunnels**: client config from setup scripts → connect SGP+FSN, standard+stealth; verify public IP change; Android kill-switch test
5. **Postgres integration**: `docker compose up postgres` in a compose file (TODO: add one for dev DB) → run migrations/001_init.sql → API smoke test with DATABASE_URL
6. **Billing sandbox**: PayHere + Paddle sandboxes; mint checkout via API → pay → webhook → subscription=active → /connect succeeds

## Known residuals (tracked)
- B1: allocator persistence now backed by Postgres store (written, needs integration test)
- B3: rate limiter still in-memory single-instance (Redis version when >1 replica)
- B4: alertmanager behind compose profile 'monitoring' — prometheus targets it by default
- Dev DB compose file not yet added (step 5 above)
- iOS/Android native configs for client not yet created (flutter create scaffolding)

## Commit history this effort
e0743a4 Phase 0 · d78d00f Phase 1 · e58c082 Phase 2 · df9c35c Phase 3 ·
cbc52c0 Phase 4 · f0b6012 zero-trust fixes ×16 · 73dc88a progress ·
0a730b5 checkout sessions + pgx + tested bug fixes · c660615 truthing + CI completion
