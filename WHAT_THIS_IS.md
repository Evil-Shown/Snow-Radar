# Snow Radar — What This Is

**Snow Radar** is a **privacy-first, censorship-resistant VPN platform** built for users in heavily censored regions (Sri Lanka, China, Iran, etc.). It is an open-source project by [Evil-Shown](https://github.com/Evil-Shown/Snow-Radar).

---

## Core Problem & Solution

| Problem | Solution |
|---------|----------|
| Commercial VPNs log traffic | Zero-knowledge architecture: no traffic logs, no bandwidth records, no connection timestamps |
| WireGuard blocked by DPI | Built-in **Stealth Mode** (AmneziaWG) with junk packets, header obfuscation, handshake scrambling |
| Single jurisdiction risk | Exit nodes in **Singapore (Oracle ARM)** + **Germany (Hetzner)** — two legal regimes |
| Vendor lock-in | Open source (Apache 2.0 backend, MIT clients), self-hostable, standard protocols |
| Opaque infrastructure | Full IaC (Terraform), documented ADRs, reproducible builds |

---

## Architecture

```
┌─────────────┐     HTTPS      ┌──────────────────┐     wgctrl      ┌──────────────────┐
│   Client    │ ─────────────▶ │  Control Plane   │ ──────────────▶ │   Exit Node 1    │
│  (Flutter)  │  POST /connect │    (Go + Gin)    │  ConfigurePeer  │  Oracle ARM SGP  │
└─────────────┘                │  PostgreSQL +    │                 │  wg0 / awg0      │
                               │  Redis           │                 │  10.10.0.0/24    │
                               └──────────────────┘                 └──────────────────┘
                                        │
                                        │ wgctrl
                                        ▼
                               ┌──────────────────┐
                               │   Exit Node 2    │
                               │  Hetzner CX22    │
                               │  wg0 / awg0      │
                               │  10.11.0.0/24    │
                               └──────────────────┘
```

### Components

| Component | Stack | Role |
|-----------|-------|------|
| **Client App** | Flutter 3.22, Riverpod, wireguard_flutter | Cross-platform VPN UI (Android, iOS, Desktop) |
| **Control Plane** | Go 1.22, Gin, pgx, Redis, wgctrl-go | Auth, peer management, IP allocation |
| **Exit Nodes** | Ubuntu 22.04, WireGuard + AmneziaWG | VPN tunnel endpoints |
| **Observability** | Prometheus + Grafana | Monitoring, alerts, dashboards |
| **Infrastructure** | Terraform + Ansible | IaC, server hardening |

### Dual Protocol Strategy

- **Standard (WireGuard)**: Port 51820/udp — kernel-accelerated, low overhead, for uncensored regions
- **Stealth (AmneziaWG)**: Port 51821/udp — obfuscation (junk padding, header scrambling) to bypass DPI

---

## Infrastructure & Cost

| Exit Node | Provider | Instance | Specs | Monthly Cost |
|-----------|----------|----------|-------|-------------|
| Singapore (APAC) | Oracle Cloud | VM.Standard.A1.Flex (ARM) | 2 OCPU, 12 GB RAM, 10 TB transfer | **$0** (Always Free) |
| Falkenstein, DE (EU) | Hetzner Cloud | CX22 (x86) | 2 vCPU, 4 GB RAM, 20 TB transfer | **€4.51** (~$4.90) |

Total monthly cost: **~$5** for two geographically distributed exit nodes.

---

## Technology Choices

| Layer | Choice | Rationale |
|-------|--------|-----------|
| VPN Protocol | WireGuard + AmneziaWG | Modern crypto, kernel perf, censorship resistance |
| API Language | Go | wgctrl-go, concurrency, single binary |
| API Framework | Gin | Mature, fast, middleware ecosystem |
| Database | PostgreSQL | ACID, JSONB, mature Go drivers |
| Cache/Queue | Redis | Session store, rate limiting |
| Client Framework | Flutter | Single codebase, native VPN APIs |
| IaC | Terraform | Multi-provider, declarative |
| Config Mgmt | Ansible | Agentless, idempotent |
| CI/CD | GitHub Actions | Matrix builds, OIDC |
| Monitoring | Prometheus + Grafana | Industry standard |

---

## 6-Phase Roadmap

| Phase | Duration | Goal |
|-------|----------|------|
| **0: Foundation** | Week 1 | IaC, repos, ADRs, domain |
| **1: Bare-Metal VPN** | Week 2 | Hardened servers, wg0/awg0, Grafana |
| **2: Control Plane** | Weeks 3-4 | Go API, wgctrl, Postgres/Redis |
| **3: Flutter MVP** | Weeks 5-6 | Key gen, secure storage, tunnel activation |
| **4: Stealth + CI/CD** | Week 7 | AmneziaWG toggle, automated builds |
| **5: Alpha Launch** | Week 8 | 20 testers across Sri Lankan ISPs |

---

## Security Model

- **No logging**: Zero traffic logs, bandwidth records, or connection metadata
- **Local key generation**: Private key never leaves the device
- **Kill switch**: Platform-native (Android VpnService, iOS NEVPNManager)
- **Server hardening**: UFW, fail2ban, SSH key-only, auto-updates, sysctl hardening
- **Supply chain**: govulncheck, dependabot, cosign, SLSA L3 target

---

## Licensing

| Component | License |
|-----------|---------|
| Backend (snowradar-api) | Apache 2.0 |
| Clients (snowradar-client) | MIT |
| Infrastructure (snowradar-infra) | Apache 2.0 |
| Documentation | CC-BY-4.0 |

---

## Current State

The project is in **planning/design phase**. The `infra/` directory contains:
- Terraform modules for Oracle + Hetzner
- Docker Compose configs for Prometheus/Grafana
- Phase 1 shell scripts for server hardening, WireGuard, AmneziaWG, Node Exporter

The Go API (`snowradar-api/`) and Flutter client (`snowradar-client/`) directories are yet to be implemented.

---

## Links

- **Repository**: https://github.com/Evil-Shown/Snow-Radar
- **Docs**: https://github.com/Evil-Shown/Snow-Radar/tree/main/docs
- **Target audience**: Sri Lankan users facing ISP-level DPI (Dialog, SLT, Mobitel, Hutch)
