# Snow Radar — High-Level Architecture

## Overview

Snow Radar is a privacy-first VPN platform with two geographically distributed exit nodes:
- **APAC**: Oracle Cloud ARM instance in Singapore (Always Free tier)
- **EU**: Hetzner CX22 instance in Falkenstein, Germany (closest to Frankfurt)

Clients connect via WireGuard/AmneziaWG. A Go-based control plane dynamically manages peers without restarting the WireGuard interface. Flutter apps (Android, iOS, Desktop) provide the user interface.

---

## System Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT DEVICES                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ Android App │  │ iOS App     │  │ Desktop App │  │ CLI / Router│          │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘          │
└─────────┼────────────────┼────────────────┼────────────────┼─────────────────┘
          │                │                │                │
          │          HTTPS + gRPC          │                │
          │◀───────────────▶│◀───────────────▶│◀───────────────▶│
          │                │                │                │
          ▼                ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CONTROL PLANE (Go API)                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │  Gin/Fiber HTTP Server                                                    │ │
│  │  ├── POST /api/v1/auth/register     ├── GET  /api/v1/servers            │ │
│  │  ├── POST /api/v1/auth/login        ├── POST /api/v1/peers              │ │
│  │  ├── POST /api/v1/auth/refresh      ├── DELETE /api/v1/peers/:id        │ │
│  │  └── GET  /api/v1/me                └── GET  /api/v1/peers/:id/config   │ │
│  └────────────────────────────┬────────────────────────────────────────────┘ │
│                               │                                              │
│  ┌────────────────────────────▼────────────────────────────────────────────┐ │
│  │  WireGuard Control (wgctrl-go)                                            │ │
│  │  • Create peer (public key, allowed IPs, preshared key)                  │ │
│  │  • Remove peer                                                            │ │
│  │  • List peers, get stats                                                 │ │
│  │  • No daemon restart required                                            │ │
│  └────────────────────────────┬────────────────────────────────────────────┘ │
└───────────────────────────────┼──────────────────────────────────────────────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          │                     │                     │
          ▼                     ▼                     ▼
┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐
│  ORACLE SINGAPORE   │ │  HETZNER FALKENSTEIN│ │  OBSERVABILITY      │
│  (APAC Exit Node)   │ │  (EU Exit Node)     │ │  (Prometheus+Grafana)│
│                     │ │                     │ │                     │
│  Ubuntu 22.04 ARM   │ │  Ubuntu 22.04 x86   │ │  Node Exporter      │
│  VM.Standard.A1.Flex│ │  CX22 (2 vCPU, 4GB) │ │  Prometheus         │
│  2 OCPU, 12GB RAM   │ │                     │ │  Grafana            │
│                     │ │  WireGuard wg0      │ │                     │
│  WireGuard wg0      │ │  AmneziaWG awg0     │ │  AlertManager       │
│  AmneziaWG awg0     │ │  iptables/nftables  │ │                     │
│  iptables/nftables  │ │  Prometheus Node    │ │                     │
│  Prometheus Node    │ │  Exporter           │ │                     │
│  Exporter           │ │                     │ │                     │
└─────────────────────┘ └─────────────────────┘ └─────────────────────┘
```

---

## Component Breakdown

### 1. Client Applications (Flutter)
| Platform | Transport | Config Delivery |
|----------|-----------|-----------------|
| Android  | WireGuard Go backend / AmneziaWG | QR code, file import, in-app download |
| iOS      | NetworkExtension + WireGuardKit | QR code, file import |
| Desktop  | wireguard-go / wg-quick | File import, CLI |
| Router   | wg-quick / OpenWrt | Config file |

**Key privacy features:**
- No telemetry, no analytics, no crash reporting by default
- Local-only key generation (private key never leaves device)
- Kill switch via platform VPN APIs
- DNS leak protection (enforce DoH/DoT to trusted resolvers)

### 2. Control Plane API (Go)
```
snowradar-api/
├── cmd/api              # Entry point
├── internal/
│   ├── auth/            # JWT issuance, validation, refresh
│   ├── server/          # HTTP handlers, middleware
│   ├── wgctrl/          # WireGuard peer management (wgctrl-go)
│   ├── store/           # PostgreSQL repositories
│   ├── config/          # Environment-driven config
│   └── middleware/      # Logging, rate limit, CORS
├── migrations/          # sql-migrate / golang-migrate
├── docker-compose.yml   # Postgres, Redis, API
└── Dockerfile
```

**Responsibilities:**
- User authentication (email/password + optional 2FA)
- Server catalog (list exit nodes with capacity/health)
- Peer lifecycle: create → return config → revoke
- Subscription/status checks (future: paid tiers)
- Audit logging (no traffic logs, only auth/peer events)

**WireGuard Management:**
```go
// Using wgctrl-go (go.wgctrl.io/wgctrl)
client, _ := wgctrl.New()
cfg := wgtypes.Config{
    Peers: []wgtypes.PeerConfig{
        {
            PublicKey:  peerPubKey,
            AllowedIPs: []net.IPNet{{IP: peerIP, Mask: /32}},
            PresharedKey: &psk,
        },
    },
}
client.ConfigureDevice("wg0", cfg)  // No restart needed
```

### 3. Exit Nodes (VPN Servers)
Each exit node runs identical software stack:

**OS Hardening:**
- Ubuntu 22.04 LTS
- Non-root sudo user, SSH key-only, disable root login
- UFW/nftables default-deny, allow only 22, 80, 443, 51820/udp
- Automatic security updates (unattended-upgrades)
- Fail2ban on SSH

**Network Stack:**
```bash
# /etc/sysctl.d/99-snowradar.conf
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
```

**WireGuard Interface (wg0):**
```ini
# /etc/wireguard/wg0.conf
[Interface]
Address = 10.10.0.1/24, fd00:10:10::1/64
ListenPort = 51820
PrivateKey = <server-private-key>
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; ip6tables -A FORWARD -i wg0 -j ACCEPT; ip6tables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE; ip6tables -D FORWARD -i wg0 -j ACCEPT; ip6tables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
```

**AmneziaWG Interface (awg0):**
- Separate port (default 51821/udp)
- Additional obfuscation headers
- Useful for censorship-heavy regions

**Leak Prevention:**
- `nf_conntrack` helper for UDP
- IPv6 ULA addressing (fd00::/8) to avoid global IPv6 leaks
- `IPV6_PRIVACY=0` on wg0
- Strict `AllowedIPs` on peers (single /32 + /128)

### 4. Observability Stack
| Component | Purpose | Deployment |
|-----------|---------|------------|
| Prometheus | Metrics collection (scrape Node Exporter, wg_exporter) | Docker Compose on control plane VM or dedicated monitoring VM |
| Grafana | Dashboards (bandwidth, peers, latency, errors) | Docker Compose |
| AlertManager | Alert on peer count spikes, bandwidth saturation, server down | Docker Compose |
| Loki (optional) | Log aggregation for audit logs | Docker Compose |

**Key Metrics:**
- `wg_peer_received_bytes_total`, `wg_peer_sent_bytes_total`
- `node_network_receive_bytes_total`, `node_network_transmit_bytes_total`
- `snowradar_active_peers`, `snowradar_peer_created_total`
- System: CPU, memory, disk, load

---

## Data Flow

### Peer Registration
```
Client                         API Server                      Exit Node
  │                              │                               │
  │── POST /auth/register ──────▶│                               │
  │◀─── 201 + JWT ───────────────│                               │
  │                              │                               │
  │── GET /servers ─────────────▶│                               │
  │◀─── [{id, region, pubkey}]───│                               │
  │                              │                               │
  │── POST /peers {server_id} ──▶│                               │
  │                              │── wgctrl.ConfigureDevice() ──▶│
  │◀─── {peer_ip, config} ───────│                               │
  │                              │                               │
  │── Apply config to WireGuard ─▶│                               │
  │                              │                               │
```

### Traffic Path
```
Client → (UDP 51820) → Exit Node wg0 → NAT → Internet
Internet → Exit Node eth0 → NAT → wg0 → (UDP 51820) → Client
```

---

## Security Model

| Layer | Controls |
|-------|----------|
| **Transport** | WireGuard (Noise_IK), AmneziaWG (obfuscated) |
| **Authentication** | Curve25519 keypairs + optional PSK per peer |
| **Authorization** | JWT (RS256) for API, peer config tied to user account |
| **Network** | `AllowedIPs` = single client IP, no subnet routes |
| **Host** | Minimal attack surface, no SSH password, auto-updates |
| **Application** | No logging of traffic, no PII in metrics, audit logs only |
| **Legal** | No data retention, jurisdiction: Singapore + Germany |

---

## Scaling Strategy

| Phase | Exit Nodes | Control Plane | Users |
|-------|------------|---------------|-------|
| MVP (Phase 0-2) | 2 (SGP, FSN) | Single VM | ~100 |
| Beta (Phase 3) | +1 US, +1 JP | Single VM + Redis | ~1,000 |
| Launch | 6-8 regions | K8s (managed) | ~10,000 |
| Growth | 15+ regions | Multi-region active-active | 100,000+ |

**Stateless API** → horizontal scaling behind load balancer.
**Peer state** → stored in PostgreSQL, synced to exit nodes via API calls.
**Exit nodes** → independent, can be added without control plane changes.

---

## Repository Structure

```
snowradar-infra/          # Terraform, Ansible, Docker Compose, docs
├── infra/terraform/      # Phase 0: Cloud resources
├── infra/ansible/        # Phase 1: Phase 1+: Server provisioning
├── docs/
│   ├── architecture/     # This file + diagrams
│   ├── adr/              # Architecture Decision Records
│   └── setup/            # Step-by-step guides
└── docker/               # Observability stack

snowradar-api/            # Go control plane
├── cmd/api/
├── internal/
├── migrations/
├── docker-compose.yml
└── Dockerfile

snowradar-client/         # Flutter apps
├── mobile/               # Android + iOS
├── desktop/              # macOS, Windows, Linux
├── shared/               # Common Dart packages
└── scripts/              # Build, release automation
```

---

## Technology Choices Summary

| Layer | Choice | Rationale |
|-------|--------|-----------|
| **IaC** | Terraform | Multi-provider, declarative, stateful |
| **Config Mgmt** | Ansible | Agentless, idempotent, good for server hardening |
| **VPN Protocol** | WireGuard + AmneziaWG | Modern crypto, kernel performance, censorship resistance |
| **API Language** | Go | wgctrl-go, concurrency, single binary, fast startup |
| **API Framework** | Gin | Mature, fast, middleware ecosystem |
| **Database** | PostgreSQL | ACID, JSONB, mature Go drivers |
| **Cache/Queue** | Redis | Session store, rate limiting, pub/sub |
| **Client Framework** | Flutter | Single codebase, native performance, VPN APIs |
| **Monitoring** | Prometheus + Grafana | Industry standard, Kubernetes-native |
| **CI/CD** | GitHub Actions | Native to GitHub, matrix builds, OIDC |

---

## Future Extensibility

- **Multi-hop**: Chain exit nodes (entry in EU → exit in SGP)
- **Split tunneling**: Per-app routing via `AllowedIPs` + platform APIs
- **Custom DNS**: DoH/DoT resolvers per user
- **Team/Org accounts**: Shared peer pools, admin panel
- **Audit log export**: SIEM integration
- **Protocol upgrades**: Post-quantum (PQ-WireGuard) when standardized