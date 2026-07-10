# Contributing to Snow Radar

Thank you for contributing! This guide covers workflow, standards, and requirements.

---

## Getting Started

### Prerequisites
- **Go** 1.22+ (backend)
- **Flutter** 3.22+ / Dart 3.4+ (client)
- **Terraform** 1.6+ (infrastructure)
- **Docker** 24+ / **Docker Compose** (local dev)
- **Git** with GPG signing configured

### First-Time Setup
```bash
# 1. Fork & clone
gh repo fork Evil-Shown/Snow-Radar --clone
cd Snow-Radar

# 2. Install Go tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 3. Install Flutter tools
flutter pub global activate build_runner
flutter pub global activate dart_code_metrics

# 4. Verify
golangci-lint --version
flutter --version
terraform version
```

---

## Development Workflow

### Branch Naming
| Type | Format | Example |
|------|--------|---------|
| Feature | `feat/<short-desc>` | `feat/stealth-mode-toggle` |
| Bug Fix | `fix/<short-desc>` | `fix/ip-allocation-race` |
| Docs | `docs/<short-desc>` | `docs/adr-004-post-quantum` |
| Refactor | `refactor/<short-desc>` | `refactor/wgctrl-pool` |
| Chore | `chore/<short-desc>` | `chore/update-dependencies` |

### Commit Messages
Follow [Conventional Commits](https://www.conventionalcommits.org/):
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:** `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `chore`, `ci`, `security`

**Examples:**
```
feat(api): add AmneziaWG peer management endpoint

Implements POST /api/v1/peers/amneziawg for stealth mode.
Adds AwgManager with separate interface handling.

Closes #42
```

```
fix(client): prevent key leakage in debug builds

Private keys were logged in debug mode on Android.
Removed all key material from log statements.

Security: CVE-2025-XXXX
```

### Signing Commits
```bash
git config --global commit.gpgsign true
git config --global user.signingkey <YOUR_KEY_ID>
# Or per-repo:
git config commit.gpgsign true
```

---

## Pull Request Process

### Before Opening PR
- [ ] Branch up-to-date with `main`
- [ ] All tests pass locally
- [ ] Linting passes (`golangci-lint`, `flutter analyze`)
- [ ] Formatters applied (`gofmt`, `dart format`)
- [ ] Commit history clean (squash fixups)
- [ ] ADR updated if architectural change
- [ ] CHANGELOG.md updated (for user-facing changes)

### PR Template
```markdown
## Summary
Brief description of changes.

## Type
- [ ] Feature
- [ ] Bug Fix
- [ ] Documentation
- [ ] Refactor
- [ ] Security
- [ ] CI/CD

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing performed (describe)

## Screenshots (UI changes)
| Before | After |
|--------|-------|
|        |       |

## Checklist
- [ ] Linting passes
- [ ] Tests pass
- [ ] No sensitive data in diff
- [ ] ADR updated (if applicable)
- [ ] CHANGELOG updated
```

### Review Requirements
| Repo | Required Approvals | Required Checks |
|------|-------------------|-----------------|
| `snowradar-infra` | 1 (owner) | `terraform plan`, `tflint`, `checkov` |
| `snowradar-api` | 1 (owner) | `golangci-lint`, `go test`, `gosec`, `docker build` |
| `snowradar-client` | 1 (owner) | `flutter analyze`, `flutter test`, `dart_code_metrics` |

---

## Code Standards

### Go (Backend)
```go
// Package naming: lowercase, single word, no underscores
package handler

// Interface first, concrete types private
type PeerManager interface {
    AddPeer(ctx context.Context, req AddPeerRequest) (*Peer, error)
}

// Errors: wrap with context, use sentinel errors for expected cases
var ErrPeerNotFound = errors.New("peer not found")

// Logging: structured, leveled, no PII
log.Info().
    Str("server_id", serverID).
    Str("peer_pubkey", pubkey[:8]+"...").
    Msg("peer added")
```

**Tools:**
```bash
golangci-lint run
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Dart/Flutter (Client)
```dart
// File naming: snake_case.dart
// Class naming: PascalCase
// Private: _prefix

class VpnController extends StateNotifier<VpnState> {
  VpnController(this._api, this._storage) : super(const VpnState.disconnected());

  final ApiClient _api;
  final SecureStorage _storage;

  Future<void> connect(Server server) async {
    state = const VpnState.connecting();
    try {
      final keyPair = await _generateKeyPair();
      await _storage.write(key: 'private_key', value: keyPair.privateKey);
      final config = await _api.connect(keyPair.publicKey, server.id);
      await _activateTunnel(config, keyPair.privateKey);
      state = VpnState.connected(server, config.assignedIp);
    } catch (e) {
      state = VpnState.error(e.toString());
    }
  }
}
```

**Tools:**
```bash
flutter analyze
dart format --set-exit-if-changed .
flutter test --coverage
dart run dart_code_metrics:metrics analyze .
```

### Terraform (Infrastructure)
```hcl
# Modules: each resource type in own file
# Naming: snake_case for resources, variables, outputs
# Required: description, type, validation for all variables

variable "instance_shape" {
  description = "OCI instance shape (ARM Flex)"
  type        = string
  default     = "VM.Standard.A1.Flex"
  validation {
    condition     = contains(["VM.Standard.A1.Flex", "VM.Standard.A2.Flex"], var.instance_shape)
    error_message = "Must be ARM Flex shape."
  }
}

# Outputs: all meaningful values, sensitive = true for secrets
output "instance_public_ip" {
  description = "Public IP of the VPN server"
  value       = oci_core_instance.vpn.public_ip
  sensitive   = false
}
```

**Tools:**
```bash
terraform fmt -recursive
terraform validate
tflint --recursive
checkov -d .
```

---

## Testing Requirements

### Backend (Go)
| Test Type | Location | Command | Coverage Target |
|-----------|----------|---------|-----------------|
| Unit | `*_test.go` next to code | `go test ./...` | ≥ 80% |
| Integration | `integration_test.go` | `go test -tags=integration ./...` | Key paths |
| Contract | `api/contract_test.go` | `go test ./api/...` | All endpoints |

### Client (Flutter)
| Test Type | Location | Command | Coverage Target |
|-----------|----------|---------|-----------------|
| Unit | `test/unit/` | `flutter test test/unit` | ≥ 70% |
| Widget | `test/widget/` | `flutter test test/widget` | Key flows |
| Integration | `integration_test/` | `flutter test integration_test` | Critical paths |

### Infrastructure
- `terraform plan` must show zero unexpected changes
- `checkov` must pass (no HIGH/CRITICAL findings)
- Manual verification checklist for each apply

---

## Architecture Decision Records (ADRs)

**Required for:**
- New language/framework/database
- Protocol changes (WireGuard → AmneziaWG params)
- Cloud provider changes
- Data model migrations
- Security model changes

**Process:**
1. Copy `docs/adr/000-template.md` → `docs/adr/NNN-title.md`
2. Fill all sections
3. Submit PR with implementation
4. Link PR in ADR "References"
5. Status → "Accepted" on merge

---

## Release Process

### Versioning
[Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`
- **MAJOR**: Breaking API changes, protocol changes
- **MINOR**: New features, backwards compatible
- **PATCH**: Bug fixes, security patches

### Release Checklist
- [ ] All PRs merged to `main`
- [ ] CHANGELOG.md updated with version + date
- [ ] Git tag created: `git tag -s vX.Y.Z -m "Release vX.Y.Z"`
- [ ] GitHub Release drafted with artifacts
- [ ] Docker images pushed to GHCR
- [ ] Flutter builds uploaded to Play Store / TestFlight
- [ ] Announcement drafted (blog, Discord, Twitter)

---

## Code of Conduct

We follow the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). Be respectful, inclusive, and constructive.

Report violations to: conduct@snowradar.app

---

## Getting Help

| Channel | Purpose |
|---------|---------|
| GitHub Discussions | Design questions, RFCs |
| GitHub Issues | Bugs, feature requests |
| Discord (`#dev`) | Real-time help, pairing |
| Email | Security, legal, private matters |

---

## Recognition

Contributors are added to:
- `AUTHORS.md` (all contributors)
- GitHub Release notes (per release)
- Hall of Fame (security reporters)

---

*Thank you for building privacy infrastructure!* 🔒