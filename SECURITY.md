# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Main branch (pre-v1.0) | ✅ Security fixes only |
| Released tags | ✅ Full support |

> **Note**: Pre-1.0 versions receive security patches but may have breaking changes. Pin to specific commit hashes in production.

---

## Reporting a Vulnerability

**Do not open public issues for security vulnerabilities.**

### Private Disclosure
**Email: security@snowradar.app**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline
| Severity | Acknowledgment | Fix Target |
|----------|----------------|------------|
| Critical (RCE, auth bypass) | 24 hours | 72 hours |
| High (privilege escalation, data leak) | 48 hours | 7 days |
| Medium (DoS, info disclosure) | 5 days | 30 days |
| Low (minor issues) | 14 days | Next release |

We will:
1. Acknowledge receipt within the timeline above
2. Provide regular progress updates
3. Credit you in the advisory (unless you request anonymity)
4. Coordinate disclosure after fix is deployed

---

## Security Architecture

### Threat Model
| Asset | Threats | Mitigations |
|-------|---------|-------------|
| User traffic | Eavesdropping, tampering | WireGuard/AmneziaWG encryption (ChaCha20-Poly1305) |
| User identity | Correlation, logging | Zero-knowledge: no traffic logs, no bandwidth records |
| Control plane | Compromise, DoS | Minimal attack surface, rate limiting, JWT auth |
| Exit nodes | Seizure, backdoor | Ephemeral infrastructure, reproducible builds |
| Client device | Malware, key extraction | Platform secure storage (Keystore, Keychain, TPM) |

### Cryptography
| Component | Algorithm | Library |
|-----------|-----------|---------|
| WireGuard handshake | Noise_IK (Curve25519, ChaCha20-Poly1305, BLAKE2s) | Kernel / wireguard-go |
| AmneziaWG obfuscation | Custom header + junk packets | amneziawg-go |
| API auth | RS256 JWT (2048-bit RSA) | golang-jwt |
| Password hashing | Argon2id (configurable params) | golang.org/x/crypto |
| TLS | TLS 1.3 (ECDHE, AES-GCM) | Go stdlib |

### Key Management
- **Server keys**: Generated at deploy, stored in `/etc/wireguard/`, rotated quarterly via Ansible
- **Client keys**: Generated on device, private key never leaves Secure Enclave/Keystore/TPM
- **API keys**: RS256 JWT, 15min access + 30d refresh, stored hashed in PostgreSQL
- **Database**: Encrypted at rest (PostgreSQL TDE), TLS in transit

---

## Infrastructure Security

### Cloud Provider Hardening
| Provider | Measures |
|----------|----------|
| **Oracle Cloud** | VCN with strict security lists, no public IPs except VPN port, cloud-init only |
| **Hetzner** | Firewall rules (22, 80, 443, 51820/udp), no floating IPs, cloud-init only |

### Server Hardening (Phase 1)
- Non-root sudo user, SSH key-only, root login disabled
- UFW default-deny, allow only 22/tcp, 80/tcp, 443/tcp, 51820/udp, 51821/udp
- Kernel sysctl: IP forwarding, no redirects, rp_filter=1
- unattended-upgrades (security only), fail2ban on SSH
- Node Exporter for monitoring, no other services

### Network Security
- WireGuard: `AllowedIPs = 10.x.x.x/32, fdxx::x/128` (single IP per peer)
- AmneziaWG: Separate subnet `10.y.y.y/24`
- NAT masquerade only on physical interface
- IPv6 ULA (fd00::/8) to prevent global IPv6 leaks
- Port 25/465/587 blocked on FORWARD chain (anti-spam)

---

## Application Security

### Backend (Go)
- **Dependencies**: `govulncheck` in CI, `go mod tidy`, pinned versions
- **Input validation**: Strict struct tags, no reflection-based parsing
- **Auth**: JWT RS256, short-lived access tokens, refresh token rotation
- **Rate limiting**: Redis-backed, per-IP and per-user tiers
- **Database**: Parameterized queries (pgx), no raw SQL
- **Logging**: Structured (zerolog), no PII, no secrets
- **Headers**: Secure defaults via Gin middleware (CSP, HSTS, etc.)

### Client (Flutter)
- **Key storage**: `flutter_secure_storage` (Keystore/Keychain/EncryptedSharedPreferences)
- **VPN API**: Platform-native (Android VpnService, iOS NetworkExtension, desktop wireguard-go)
- **Network**: Certificate pinning for API, no cleartext traffic
- **Permissions**: Minimal (network, foreground service)
- **Obfuscation**: `--obfuscate --split-debug-info` for release builds

---

## Supply Chain Security

### CI/CD (GitHub Actions)
- **Runners**: GitHub-hosted (ephemeral), no self-hosted
- **Permissions**: Least privilege (`contents: read`, `packages: write`, `id-token: write`)
- **OIDC**: Used for cloud deployments (no long-lived secrets)
- **Artifacts**: Signed (cosign), SBOM generated (syft)
- **Dependencies**: `govulncheck`, `npm audit` (if any), `flutter pub outdated`

### Container Images
- **Base**: `gcr.io/distroless/static:nonroot` or `alpine:3.20`
- **User**: Non-root (UID 65532), read-only rootfs
- **Capabilities**: Only `CAP_NET_ADMIN` + `CAP_SYS_MODULE` (for wgctrl)
- **Scanning**: Trivy in CI, no HIGH/CRITICAL allowed

### Reproducible Builds
- Go: `CGO_ENABLED=0`, fixed `GOOS/GOARCH`, `-trimpath`, `-ldflags="-buildid="`
- Flutter: Fixed Flutter version, `--build-number` from git tag
- Docker: `--build-arg SOURCE_DATE_EPOCH`, deterministic layer ordering

---

## Incident Response

### Runbooks (docs/operations/)
| Scenario | Runbook |
|----------|---------|
| Exit node compromise | `INCIDENT_EXIT_NODE_COMPROMISE.md` |
| Control plane breach | `INCIDENT_API_BREACH.md` |
| Key material leak | `INCIDENT_KEY_LEAK.md` |
| DDoS on VPN ports | `INCIDENT_DDOS.md` |
| Legal request | `INCIDENT_LEGAL_REQUEST.md` |

### Communication
- **Internal**: `#incidents` Discord channel, PagerDuty for critical
- **External**: Status page (status.snowradar.app), email to affected users
- **Postmortem**: Blameless, published within 5 business days

---

## Audit History

| Date | Auditor | Scope | Findings |
|------|---------|-------|----------|
| TBD | TBD | Full stack | TBD |

---

## Bug Bounty

| Severity | Reward |
|----------|--------|
| Critical | $2,000 |
| High | $1,000 |
| Medium | $500 |
| Low | $100 |

**Scope**: `*.snowradar.app`, `snowradar-api`, `snowradar-client`, `snowradar-infra`
**Exclusions**: Social engineering, physical attacks, DoS, issues in third-party services
**Payment**: Via GitHub Sponsors or wire transfer (W-9/W-8BEN required)

---

## Contact

- **Security**: security@snowradar.app (PGP: `0xDEADBEEFCAFEBABE`)
- **General**: hello@snowradar.app
- **Legal**: legal@snowradar.app