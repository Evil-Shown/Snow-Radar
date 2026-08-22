# PROGRESS.md — institutional memory

> Updated every session. Status legend: ✅ done (with evidence) | 🔶 code-complete, needs manual verification on real infra | ⬜ not started

## Current phase
Phase 0 complete (code-level). Phases 1–7 in progress this session.

## Phase 0 — Stop the bleeding (audit #1–5)

| Item | Task | Status | Evidence |
|------|------|--------|----------|
| 1 | Unified subnet scheme | ✅ | ADR-004 + `infra/configs/subnets.env` (single source); wg0/awg0 = 10.20.x/10.21.x, no VCN overlap; all scripts source it |
| 2 | Open UDP 51821 both clouds | ✅ code / 🔶 apply | `modules/oracle/main.tf` + `modules/hetzner/main.tf` add awg_port rule (parameterized). `terraform validate/apply` NOT runnable in this env — manual verify required |
| 3 | Fix AmneziaWG install | ✅ code / 🔶 run | Rewritten to official `ppa:amnezia/ppa` (fingerprint-pinned key), packages `amneziawg` + `amneziawg-linux-kmod`, keys via `awg genkey|pubkey`, service `awg-quick@awg0`. Old broken `amneziawg-go genkey` removed. First-run verification needs a real Ubuntu 22.04 node |
| 4 | Node Exporter private bind | ✅ code | Both copies rewritten: binds `${WG_IP}:9100`, fails loudly if no WG IP, NO ufw allow 9100; prometheus.yml targets now WG IPs |
| 5 | Bootstrap paths | ✅ syntax-verified | `bootstrap-phase1.sh` resolves canonical scripts via SCRIPT_DIR, sources subnets.env, passes node arg. `bash -n` OK on all 5 scripts |

Notes:
- Interface naming standardized `wg0`/`awg0`; old `wg1` retired.
- AWG obfuscation params are still the 1.x set — flagged for review when client support lands (AWG 2.0 changed params upstream).
- Duplicate script/config trees still exist (shimmed) — Phase 1 consolidates.

## Phase 1 — Foundation
- [ ] Consolidate duplicated trees (`infra/phase1/scripts/*` vs `infra/scripts/phase1/*`, dup observability ymls)
- [ ] Remote encrypted Terraform state backend + no plaintext secrets in repo/history (tfvars.example cleanup, git-secrets/trufflehog scan)
- [ ] `prevent_destroy` on stateful resources, backup plan
- [ ] Resolve IPv6 doc-vs-Terraform mismatch

## Phase 2 — Hardening + Observability (audit #6–9, 14–18)
- [ ] Grafana/Prometheus/Alertmanager auth + private binding only
- [ ] Alertmanager env templating (envsubst entrypoint)
- [ ] sysctl file shebang fix
- [ ] SSH key provisioning fail-loud check
- [ ] SSH banner wording (privacy posture) — product/legal decision, propose options
- [ ] rp_filter per-interface after testing
- [ ] Remove/scope NOPASSWD sudo
- [ ] Image updates + Trivy/Renovate automation

## Phase 3 — Ansible + CI/CD
- [ ] Ansible playbooks: node provisioning, unattended-upgrades, key rotation
- [ ] GitHub Actions: lint/shellcheck/tf plan, block merge

## Phase 4–5 — Go API / Flutter client
- [ ] Scaffold with auth + billing webhook paths tested first

## Phase 6–7 — Docs gap closure + QA evidence pass
