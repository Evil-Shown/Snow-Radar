# Snow Radar — Documentation Index

## Architecture & Decisions
- [High-Level Architecture](architecture/ARCHITECTURE.md) — System diagram, component breakdown, data flows, security model
- [Implementation Guide](implementation/IMPLEMENTATION_GUIDE.md) — Phase-by-phase tasks, code structure, acceptance criteria
- [Step-by-Step Setup](setup/SETUP.md) — Prerequisites, cloud accounts, Terraform deploy, server hardening, VPN config, backend, client, CI/CD

## Architecture Decision Records (ADRs)
- [ADR Template](adr/000-template.md)
- [ADR-001: Go + Flutter](adr/001-go-flutter.md)
- [ADR-002: WireGuard + AmneziaWG](adr/002-wireguard-amneziawg.md)
- [ADR-003: Oracle + Hetzner](adr/003-oracle-hetzner.md)

## Operations
- [GitHub & Cloud Setup](operations/GITHUB_SETUP.md) — Org, repos, secrets, machine user, domain
- [Server Hardening Checklist](operations/HARDENING_CHECKLIST.md) — SSH, UFW, sysctl, unattended-upgrades, fail2ban
- [Monitoring Runbook](operations/MONITORING_RUNBOOK.md) — Grafana dashboards, alerts, log locations, incident response

## Development
- [API Specification](development/API_SPEC.md) — OpenAPI/Swagger, auth, endpoints, error codes
- [Client Architecture](development/CLIENT_ARCH.md) — Riverpod providers, platform channels, key storage, tunnel lifecycle
- [Release Process](development/RELEASE_PROCESS.md) — Versioning, changelog, APK/IPA build, TestFlight/Play Console

## Legal & Compliance
- [Privacy Policy](legal/PRIVACY_POLICY.md)
- [Acceptable Use Policy](legal/AUP.md)
- [Data Processing Addendum](legal/DPA.md)

---

## Quick Links

| Task | Document |
|------|----------|
| Start Phase 0 (IaC) | [Setup Guide → Phase 0](setup/SETUP.md#phase-0-foundation-week-1) |
| Deploy Terraform | [Setup → Terraform Deploy](setup/SETUP.md#8-deploy-infrastructure) |
| Harden Servers | [Setup → Phase 1](setup/SETUP.md#phase-1-bare-metal-vpn--observability-week-2) |
| Write Go Backend | [Implementation Guide → Phase 2](implementation/IMPLEMENTATION_GUIDE.md#phase-2-control-plane--go-backend-weeks-3-4) |
| Build Flutter App | [Implementation Guide → Phase 3](implementation/IMPLEMENTATION_GUIDE.md#phase-3-flutter-client-mvp-weeks-5-6) |
| Configure CI/CD | [Implementation Guide → Phase 4](implementation/IMPLEMENTATION_GUIDE.md#phase-4-stealth-security--cicd-week-7) |

---

## Repository Structure

```
snowradar-infra/
├── docs/
│   ├── architecture/
│   ├── implementation/
│   ├── setup/
│   ├── adr/
│   ├── operations/
│   ├── development/
│   └── legal/
├── infra/
│   ├── terraform/           # Phase 0: Cloud resources
│   └── ansible/             # Phase 1+: Server config
├── docker/
│   └── observability/       # Prometheus + Grafana
└── scripts/                 # Bootstrap, backup, rotate keys

snowradar-api/
├── cmd/api/
├── internal/
├── migrations/
├── docker-compose.yml
└── Dockerfile

snowradar-client/
├── mobile/                  # Android + iOS
├── desktop/                 # macOS, Windows, Linux
├── shared/                  # Common Dart packages
└── scripts/                 # Build, sign, release
```

---

## Contributing to Docs

1. All ADRs use the template in `adr/000-template.md`
2. Architecture diagrams: Mermaid.js in markdown (renders on GitHub)
3. Update this index when adding new documents
4. Keep docs in sync with code — review in PRs