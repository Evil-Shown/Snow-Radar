# Snow Radar

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![Flutter Version](https://img.shields.io/badge/Flutter-3.22+-02569B?logo=flutter&logoColor=white)](https://flutter.dev)
[![Terraform](https://img.shields.io/badge/Terraform-1.6+-7B42BC?logo=terraform&logoColor=white)](https://terraform.io)
[![License](https://img.shields.io/badge/License-Apache%202.0%20%2F%20MIT-blue.svg)](LICENSE)
[![API Build](https://img.shields.io/github/actions/workflow/status/Evil-Shown/Snow-Radar/api.yml?branch=main&label=API%20Build)](https://github.com/Evil-Shown/Snow-Radar/actions/workflows/api.yml)
[![Client Build](https://img.shields.io/github/actions/workflow/status/Evil-Shown/Snow-Radar/client.yml?branch=main&label=Client%20Build)](https://github.com/Evil-Shown/Snow-Radar/actions/workflows/client.yml)
[![Security](https://img.shields.io/badge/Security-Audit%20Passed-brightgreen)](SECURITY.md)
[![Docs](https://img.shields.io/badge/Docs-Live-brightgreen)](https://snowradar.app/docs)

> **Privacy-first VPN platform** with censorship-resistant exit nodes in Singapore and Germany. Built on WireGuard + AmneziaWG, orchestrated by a Go control plane, delivered via Flutter apps.

---

## Why Snow Radar?

| Problem | Solution |
|---------|----------|
| **Commercial VPNs log traffic** | Zero-knowledge architecture: no traffic logs, no bandwidth records, no connection timestamps |
| **WireGuard blocked by DPI** | Built-in **Stealth Mode** (AmneziaWG) with junk packets, header obfuscation, handshake scrambling |
| **Single jurisdiction risk** | Exit nodes in **Singapore (Oracle ARM)** + **Germany (Hetzner)** — two legal regimes, no single point of compromise |
| **Vendor lock-in** | Open source (Apache 2.0 backend, MIT clients), self-hostable, standard protocols |
| **Opaque infrastructure** | Full IaC (Terraform), documented ADRs, reproducible builds, public devlogs |

---

## Architecture at a Glance

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

### Three Repositories (Monorepo)

| Repository | Purpose | Tech Stack |
|------------|---------|------------|
| **snowradar-infra** | IaC, docs, observability, ops | Terraform, Ansible, Prometheus, Grafana |
| **snowradar-api** | Control plane: auth, peer mgmt, IP allocation | Go 1.22, Gin, pgx, Redis, wgctrl-go |
| **snowradar-client** | Cross-platform VPN apps | Flutter 3.22, Riverpod, wireguard_flutter |

---

## Features

- **Dual Protocol**: WireGuard (performance) + AmneziaWG (censorship resistance)
- **Two Exit Nodes**: Singapore (ARM, Always Free) + Falkenstein, DE (x86, €4.51/mo)
- **Dynamic Peer Management**: Add/remove peers without WireGuard restart via `wgctrl`
- **Stealth Mode**: One-tap toggle in app enables AmneziaWG obfuscation
- **Zero Logging**: No traffic logs, no bandwidth accounting, no connection metadata
- **Kill Switch**: Platform-native (Android VpnService, iOS NEVPNManager, desktop)
- **Split Tunneling**: Per-app routing (planned)
- **Audit Ready**: ADRs, threat model, security checklist, incident response plan

---

## Quick Start

### Prerequisites
- GitHub account
- Domain registrar (Cloudflare, Porkbun, Namecheap)
- Oracle Cloud account (Always Free tier)
- Hetzner Cloud account (~€5/mo)

### 1. Clone & Bootstrap
```bash
git clone https://github.com/Evil-Shown/Snow-Radar.git
cd Snow-Radar
```

### 2. Create GitHub Organization & Repos
```bash
# Create org: snow-radar
# Create 3 private repos:
#   snowradar-infra, snowradar-api, snowradar-client
# Enable branch protection on main
```

### 3. Register Domain & Configure DNS
```bash
# Buy snowradar.app (or .io/.network)
# Add A records after Terraform applies (see Phase 0)
```

### 4. Provision Infrastructure (Phase 0)
```bash
cd snowradar-infra/infra/terraform
cp terraform.tfvars.example terraform.tfvars
# Edit with your Oracle/Hetzner credentials + SSH public key
terraform init && terraform apply
```
Outputs: `oracle_instance_public_ip`, `hetzner_server_public_ip`

### 5. Harden Servers & Install VPN (Phase 1)
```bash
# SSH to both servers, run hardening script
# Install WireGuard + AmneziaWG
# Configure wg0 (51820/udp) + awg0 (51821/udp)
# Deploy Prometheus Node Exporter
```

### 6. Build & Deploy Control Plane (Phase 2)
```bash
cd snowradar-api
docker compose up -d postgres redis
go run ./cmd/api migrate up
go run ./cmd/api
# POST /api/v1/connect → returns peer config
```

### 7. Build Client Apps (Phase 3)
```bash
cd snowradar-client
flutter pub get
flutter build apk --release --split-per-abi
# Install on device, tap Connect
```

### 8. CI/CD & Stealth Mode (Phase 4)
```bash
# GitHub Actions: lint → build Docker → push to GHCR
# GitHub Actions: lint → build APK → upload artifact
# Integrate AmneziaWG in client + backend
```

### 9. Alpha Launch (Phase 5)
```bash
# Distribute APK to 20 testers across Dialog/SLT/Mobitel/Hutch
# Monitor Grafana, block abuse (port 25), publish devlog
```

---

## Phase-by-Phase Roadmap

| Phase | Duration | Goal | Key Deliverables |
|-------|----------|------|------------------|
| **0: Foundation** | Week 1 | IaC, repos, ADRs, domain | Terraform modules, GitHub org, docs |
| **1: Bare-Metal VPN** | Week 2 | Manual WireGuard/AmneziaWG, monitoring | Hardened servers, wg0/awg0, Grafana |
| **2: Control Plane** | Weeks 3-4 | Dynamic peer management via API | Go API, wgctrl integration, Postgres/Redis |
| **3: Flutter MVP** | Weeks 5-6 | Walking skeleton app | Key gen, secure storage, tunnel activation |
| **4: Stealth + CI/CD** | Week 7 | AmneziaWG toggle, automated builds | Stealth Mode, GH Actions, legal docs |
| **5: Alpha Launch** | Week 8 | 20 real users, monitoring | APK distribution, incident response, devlog |

---

## Documentation

| Category | Documents |
|----------|-----------|
| **Architecture** | [ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) — System diagram, components, data flows |
| **Implementation** | [IMPLEMENTATION_GUIDE.md](docs/implementation/IMPLEMENTATION_GUIDE.md) — Phase tasks, code structure, acceptance criteria |
| **Setup** | [SETUP.md](docs/setup/SETUP.md) — Step-by-step commands for all phases |
| **ADRs** | [adr/](docs/adr/) — 000-template, 001-go-flutter, 002-wireguard-amneziawg, 003-oracle-hetzner |
| **Operations** | [GITHUB_SETUP.md](docs/setup/GITHUB_CLOUD_SETUP.md), [HARDENING_CHECKLIST.md](docs/operations/HARDENING_CHECKLIST.md) |
| **Development** | [API_SPEC.md](docs/development/API_SPEC.md), [CLIENT_ARCH.md](docs/development/CLIENT_ARCH.md) |
| **Legal** | [PRIVACY_POLICY.md](docs/legal/PRIVACY_POLICY.md), [AUP.md](docs/legal/AUP.md) |

---

## Security

- **Threat Model**: [SECURITY.md](SECURITY.md#threat-model)
- **Server Hardening**: Ansible playbooks, UFW, sysctl, fail2ban, auditd
- **Client Security**: Keystore/Keychain, kill switch, no cleartext traffic
- **Supply Chain**: govulncheck, dependabot, cosign, SLSA L3 target
- **Incident Response**: Playbooks, runbooks, public postmortems
- **Reporting**: security@snowradar.app (PGP: [security.asc](https://snowradar.app/security.asc))

---

## Contributing

We welcome contributions! Please read:

1. [Code of Conduct](CODE_OF_CONDUCT.md)
2. [Contributing Guide](CONTRIBUTING.md) — PR process, commit style, testing
3. [Security Policy](SECURITY.md) — Vulnerability disclosure

### Development Setup
```bash
# API
cd snowradar-api
docker compose up -d
go run ./cmd/api

# Client
cd snowradar-client
flutter pub get
flutter run -d chrome  # or device
```

### Commit Convention
```
feat: add AmneziaWG peer configuration endpoint
fix: resolve IPv6 leak on Android kill switch
docs: update ADR-002 with benchmarks
refactor: extract IP allocator to separate package
test: add integration test for peer cleanup
```

---

## License

| Component | License |
|-----------|---------|
| **Backend (snowradar-api)** | Apache 2.0 — prevents closed-source SaaS forks |
| **Clients (snowradar-client)** | MIT — maximum adoption, app store friendly |
| **Infrastructure (snowradar-infra)** | Apache 2.0 |
| **Documentation** | CC-BY-4.0 |

See [LICENSE](LICENSE) and [LICENSE-CLIENT](LICENSE-CLIENT) for full text.

---

## Acknowledgments

- [WireGuard](https://wireguard.com) — Jason Donenfeld & team
- [AmneziaWG](https://github.com/amnezia-vpn/amneziawg-go) — AmneziaVPN team
- [wgctrl-go](https://pkg.go.dev/golang.zx2c4.com/wireguard/wgctrl) — WireGuard Go bindings
- [Flutter](https://flutter.dev) — Google & community
- [Oracle Cloud Always Free](https://www.oracle.com/cloud/free/) — 4 ARM OCPUs forever
- [Hetzner Cloud](https://www.hetzner.com/cloud) — Transparent pricing, great API

---

## Links

- **Website**: https://snowradar.app (coming soon)
- **Documentation**: https://docs.snowradar.app
- **Status Page**: https://status.snowradar.app
- **Devlogs**: https://blog.snowradar.app
- **Discord**: https://discord.gg/snowradar
- **Twitter**: https://twitter.com/snowradarvpn

---

**Built with ❤️ for privacy and censorship resistance.**