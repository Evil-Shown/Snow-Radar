# ADR-002: Choosing WireGuard + AmneziaWG as VPN Protocols

## Status
Accepted

## Context
Snow Radar must provide censorship-resistant VPN connectivity in environments with active Deep Packet Inspection (DPI) such as Sri Lanka, China, Iran, Russia. Standard WireGuard is trivially identifiable by its static handshake pattern and fixed header structure.

Requirements:
1. **Baseline**: Modern, audited, kernel-accelerated VPN protocol
2. **Stealth mode**: Obfuscation to bypass DPI without sacrificing performance
3. **Auditability**: Open source, formal verification preferred
4. **Client support**: Available on all target platforms (Android, iOS, Desktop)
4. **Operational simplicity**: Minimal configuration drift, easy rotation

## Decision
- **Primary protocol**: WireGuard (kernel module, `wgctrl-go` management)
- **Stealth protocol**: AmneziaWG (fork of WireGuard with obfuscation) as secondary interface
- **Port allocation**: 
  - WireGuard: 51820/udp (standard)
  - AmneziaWG: 51821/udp (alternative)
- **Client-side**: "Stealth Mode" toggle in UI selects protocol

## Consequences

### Positive
- **WireGuard**: Industry standard; in Linux kernel since 5.6; formal verification (Noise_IK); high performance (kernel fast path); tiny attack surface; ubiquitous client support
- **AmneziaWG**: 
  - Junk packet padding (Jc, Jmin, Jmax) masks packet timing/size
  - Header obfuscation (S1, S2, H1-H4) breaks static signature detection
  - Compatible with WireGuard config format (mostly)
  - Actively maintained by AmneziaVPN team
  - Used in production by AmneziaVPN, NekoBox, others
- **Dual-protocol**: Users in free regions get optimal WireGuard; censored regions get AmneziaWG; seamless fallback possible

### Negative
- **AmneziaWG**: 
  - Less audited than WireGuard (no formal verification of obfuscation)
  - Slightly higher CPU (junk packet generation)
  - Slightly more bandwidth overhead (padding)
  - Fewer client libraries (must use `amneziawg-go` or `amneziawg-rs`)
  - Non-standard port may be blocked by default-deny firewalls
- **Operational**: Two interfaces to monitor, two config formats, key rotation for both

### Neutral
- **Key management**: Separate keypairs per protocol per server (cleaner isolation)
- **IP allocation**: Separate subnets (10.10.0.0/24 for WG, 10.11.0.0/24 for AWG)

## Alternatives Considered

| Alternative | Pros | Cons | Why Not Chosen |
|-------------|------|------|----------------|
| **WireGuard only** | Simplest, best audited | Trivially blocked by DPI | Fails core censorship-resistance requirement |
| **WireGuard + Shadowsocks/V2Ray** | Mature obfuscation, multiple protocols | Complex stack; Shadowsocks deprecated; V2Ray heavy | Over-engineered; multiple moving parts |
| **WireGuard + XRay/VLESS** | Good obfuscation, active dev | Complex config; single Go binary but heavy | Protocol complexity |
| **WireGuard + Obfs4** | Tor's obfuscation, well-studied | Separate proxy layer; adds latency | Architecture mismatch |
| **AmneziaWG only** | Single protocol | No fallback; newer, less battle-tested | Risk mitigation via dual-stack |
| **Custom obfuscation (WG + custom)** | Full control | Reinventing wheel; audit burden | Not core competency |

## References
- [WireGuard Protocol Whitepaper](https://www.wireguard.com/papers/wireguard.pdf)
- [AmneziaWG Specification](https://github.com/amnezia-vpn/amneziawg-go/blob/main/docs/protocol.md)
- [DPI Detection of WireGuard](https://www.ndss-symposium.org/ndss-paper/automated-detection-and-analysis-of-vpn-protocol-dpi/)
- [AmneziaVPN Production Use](https://amnezia.org/)

## Metadata
- **Date**: 2025-01-15
- **Author**: Snow Radar Founder
- **Tags**: protocol, wireguard, amneziawg, censorship, dpi, security