# ADR-001: Choosing Go + Flutter as Primary Technology Stack

## Status
Accepted

## Context
Snow Radar needs a technology stack that satisfies:
1. **Performance**: Control plane must handle thousands of concurrent peer operations with minimal latency
2. **Cross-platform clients**: Android, iOS, macOS, Windows, Linux from single codebase
3. **WireGuard ecosystem**: Go has first-class `wgctrl-go` library for kernel WireGuard management
4. **Team velocity**: Solo founder with Go background, learning Flutter
5. **Deployment simplicity**: Single binary (Go), single APK/IPA (Flutter)
6. **Long-term maintainability**: Strong standard libraries, backward compatibility

## Decision
- **Backend (Control Plane)**: Go 1.22+ with Gin framework
- **Client Applications**: Flutter 3.22+ (Dart 3) with Riverpod state management
- **Infrastructure**: Terraform for IaC, Ansible for config management
- **Observability**: Prometheus + Grafana

## Consequences

### Positive
- **Go**: Excellent `wgctrl-go` for dynamic peer management without daemon restart; single binary deployment; fast startup; built-in concurrency for connection storms; strong crypto libraries
- **Flutter**: True single codebase for all platforms; native performance via AOT compilation; `wireguard_flutter` plugin for direct WireGuard integration; growing ecosystem; Google-backed
- **Unified stack**: Both languages have excellent tooling, formatting, testing built-in; easy to hire for later

### Negative
- **Go**: Verbose error handling; smaller web framework ecosystem than Node/Python
- **Flutter**: Larger binary size (~10-15MB overhead); Dart less common than Kotlin/Swift/JS; iOS requires macOS for build; platform-specific VPN APIs need native channels

### Neutral / Risks
- **Flutter VPN plugins**: `wireguard_flutter` is community-maintained; may lag behind OS VPN API changes
- **Go wgctrl**: Requires `CAP_NET_ADMIN` and kernel headers; container deployment needs privileges

## Alternatives Considered

| Alternative | Pros | Cons | Why Not Chosen |
|-------------|------|------|----------------|
| **Rust + Tauri** | Memory safety, performance, smaller binaries | Steeper learning curve; Tauri mobile still maturing; WireGuard bindings less mature | Team velocity priority |
| **Node.js + React Native** | Huge ecosystem, JS everywhere | Single-threaded event loop problematic for high-concurrency peer ops; heavier runtime | Performance risk |
| **Python + Kivy/BeeWare** | Fast prototyping | GIL limits concurrency; mobile support immature; no production WireGuard control library | Not production-ready |
| **Kotlin Multiplatform + Ktor** | Native Android, shared logic | iOS support newer; smaller ecosystem; solo dev context-switching cost | Flutter wins on UI consistency |

## References
- [wgctrl-go documentation](https://pkg.go.dev/golang.zx2c4.com/wireguard/wgctrl)
- [Flutter VPN case studies](https://flutter.dev/showcase)
- [Go at Cloudflare (WireGuard)](https://blog.cloudflare.com/wireguard/)

## Metadata
- **Date**: 2025-01-15
- **Author**: Snow Radar Founder
- **Tags**: architecture, language, framework, client, backend