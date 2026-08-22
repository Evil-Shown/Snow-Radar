# ADR-004: Unified Subnet & Port Scheme

- Status: Accepted
- Date: 2026-08-22
- Deciders: Principal Architect / Network Engineer roles
- Supersedes: implicit ad-hoc allocations in phase-1 scripts

## Context

Audit item #1: four sources disagreed on tunnel addressing:

| Source | Allocation |
|---|---|
| `setup-wireguard.sh` (old) | wg0 = 10.0.0.0/24 |
| `setup-amneziawg.sh` (old) | wg1 = 10.10.0.0/24 |
| `bootstrap-phase1.sh` (old) | wg0 = 10.10.0.0/24, wg1 = 10.11.0.0/24 |
| Oracle Terraform VCN | 10.10.0.0/16 |

The AmneziaWG subnet collided with the OCI VCN supernet (routing breakage on SGP), and no allocation was machine-readable from a single source.

## Options considered

### Option A — Keep VCN at 10.10.0.0/16, tunnels inside it
Tunnels carve out of the VCN range. Rejected: conflates cloud-fabric addressing with customer tunnel space; any future VCN peering/subnet growth collides again.

### Option B — Role-blocked ranges outside provider fabric (SELECTED)
Provider networks own their existing ranges (OCI VCN stays 10.10.0.0/16). Customer tunnels live in a dedicated, non-overlapping block, one /16 per region, split by protocol role:

```
Node            wg0 (standard)      awg0 (stealth)        ULA v6 (wg0)
Singapore (sgp) 10.20.0.0/24 (.1)   10.20.1.0/24 (.1)     fd00:20::/64 (::1)
Falkenstein(fsn)10.21.0.0/24 (.1)   10.21.1.0/24 (.1)     fd00:21::/64 (::1)
```

Allocation rule for future nodes: next free /16 in `10.20.0.0 – 10.99.255.255`, first /24 = standard WG, second /24 = stealth AWG. Ports fixed: UDP 51820 (wg0), UDP 51821 (awg0).

### Option C — Single flat supernet (100.64-style CGNAT pool)
Rejected: CGNAT space leaks into odd ISP NATs and complicates client-side AllowedIPs debugging; RFC1918 with strict per-node blocks is boring and auditable, which is our stated default.

## Decision

Option B, centralized in **`infra/configs/subnets.env`** — the only place tunnel addressing is declared. Shell scripts source it; Terraform variables must mirror it (validated by plan-time checks in a later phase). Interface naming standardized to `wg0` / `awg0` everywhere (the old `wg1` name is retired).

## Consequences

- No overlap between tunnel space (10.20–10.99) and either provider's fabric (OCI 10.10/16; Hetzner assigns public-only).
- Client IP allocation (future control plane) derives from these blocks: `.2`–`.254` per peer pool.
- Any new consumer of addresses MUST read `subnets.env` or the Terraform variables — never hardcode.
