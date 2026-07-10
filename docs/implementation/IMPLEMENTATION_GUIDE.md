# Snow Radar Implementation Guide

This guide breaks down each phase into actionable tasks with acceptance criteria, test commands, and rollback points.

---

## Phase 0: Foundation, Repositories & IaC (Week 1)

### 0.1 GitHub Organization & Repositories

**Tasks:**
1. Create GitHub organization `snow-radar`
2. Create three private repositories:
   - `snowradar-infra`
   - `snowradar-api`
   - `snowradar-client`
3. Enable branch protection on `main`: require PR reviews, status checks
4. Add `snowradar-bot` machine user for CI/CD with write access to packages

**Acceptance:**
- All repos visible in org
- Branch protection active

### 0.2 Architecture Decision Records

**Tasks:**
1. Create `docs/adr/000-template.md`
2. Write ADR-001: Go + Flutter
3. Write ADR-002: WireGuard + AmneziaWG
4. Write ADR-003: Oracle + Hetzner (note Frankfurt → Falkenstein)

**Acceptance:**
- ADRs committed and linked from README

### 0.3 Domain Registration

**Tasks:**
1. Register `snowradar.app` (or `.io` / `.network`)
2. Configure DNS:
   - `@` → placeholder IP (1.1.1.1)
   - `api` → placeholder IP
   - `sgp` → placeholder IP
   - `fsn` → placeholder IP
   - `www` → placeholder IP
3. Enable DNSSEC
4. Add CAA records for Let's Encrypt

**Acceptance:**
- `dig snowradar.app` returns placeholder IPs

### 0.4 Terraform Infrastructure

**Files to create in `snowradar-infra/`:**
```
infra/
├── main.tf
├── variables.tf
├── outputs.tf
├── terraform.tfvars.example
├── .gitignore
├── modules/
│   ├── oracle/
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   └── outputs.tf
│   └── hetzner/
│       ├── main.tf
│       ├── variables.tf
│       └── outputs.tf
```

**Key Resources:**
- Oracle: VCN, subnet, IGW, route table, security list, compute instance (VM.Standard.A1.Flex, 2 OCPU, 12GB)
- Hetzner: Firewall, SSH key, server (CX22, fsn1)
- Both: Ubuntu 22.04, cloud-init for user setup

**Security Rules (both):**
| Port | Protocol | CIDR | Purpose |
|------|----------|------|---------|
| 22 | TCP | YOUR_IP/32 | SSH (restrict!) |
| 80 | TCP | 0.0.0.0/0 | HTTP |
| 443 | TCP | 0.0.0.0/0 | HTTPS |
| 51820 | UDP | 0.0.0.0/0 | WireGuard |

**Acceptance:**
```bash
cd infra && terraform init && terraform plan
terraform apply  # completes without errors
ssh -i ~/.ssh/snowradar_ed25519 ubuntu@<oracle-ip> "echo OK"
ssh -i ~/.ssh/snowradar_ed25519 root@<hetzner-ip> "echo OK"
```

### 0.5 Cloud-Init User Data

**Oracle (`modules/oracle/cloud-init.yaml`):**
```yaml
#cloud-config
users:
  - name: snowradar
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - <PUBLIC_KEY>
packages:
  - wireguard
  - wireguard-tools
  - ufw
  - curl
  - htop
  - net-tools
write_files:
  - path: /etc/sysctl.d/99-snowradar.conf
    content: |
      net.ipv4.ip_forward=1
      net.ipv6.conf.all.forwarding=1
runcmd:
  - ufw allow 22/tcp
  - ufw allow 80/tcp
  - ufw allow 443/tcp
  - ufw allow 51820/udp
  - ufw --force enable
  - systemctl enable --now systemd-resolved
```

**Hetzner (`modules/hetzner/cloud-init.yaml`):**
Similar, but user is `root` initially.

---

## Phase 1: Bare-Metal VPN & Observability (Week 2)

### 1.1 SSH Hardening

**On both servers:**
```bash
# Create admin user (Oracle already has ubuntu)
adduser --disabled-password --gecos "" snowradar
usermod -aG sudo snowradar
mkdir -p /home/snowradar/.ssh
cp /root/.ssh/authorized_keys /home/snowradar/.ssh/
chown -R snowradar:snowradar /home/snowradar/.ssh
chmod 700 /home/snowradar/.ssh
chmod 600 /home/snowradar/.ssh/authorized_keys

# Harden sshd
cat > /etc/ssh/sshd_config.d/99-snowradar.conf <<'EOF'
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys
ChallengeResponseAuthentication no
UsePAM no
AllowUsers snowradar
ClientAliveInterval 300
ClientAliveCountMax 2
EOF

systemctl reload sshd
```

**Test:** New SSH session works; root login denied.

### 1.2 Kernel Forwarding & Sysctl

```bash
cat > /etc/sysctl.d/99-snowradar-forward.conf <<'EOF'
net.ipv4.ip_forward=1
net.ipv6.conf.all.forwarding=1
net.ipv4.conf.all.accept_redirects=0
net.ipv4.conf.all.send_redirects=0
net.ipv4.conf.default.rp_filter=1
net.ipv4.conf.all.rp_filter=1
EOF
sysctl --system
```

**Verify:** `sysctl net.ipv4.ip_forward net.ipv6.conf.all.forwarding`

### 1.3 WireGuard Server Config

```bash
# Generate server keys
wg genkey | tee /etc/wireguard/server_private.key | wg pubkey > /etc/wireguard/server_public.key
chmod 600 /etc/wireguard/server_private.key

# wg0.conf
cat > /etc/wireguard/wg0.conf <<'EOF'
[Interface]
Address = 10.0.0.1/24, fd00::1/64
ListenPort = 51820
PrivateKey = <SERVER_PRIVATE_KEY>
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; ip6tables -A FORWARD -i wg0 -j ACCEPT; ip6tables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE; ip6tables -D FORWARD -i wg0 -j ACCEPT; ip6tables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
EOF

systemctl enable --now wg-quick@wg0
wg show
```

**Leak Prevention:** Add `net.ipv4.conf.all.rp_filter=1` and ensure `PostUp`/`PostDown` only masquerade outbound on physical interface.

### 1.4 AmneziaWG Installation

```bash
# Install from GitHub releases
curl -fsSL https://github.com/amnezia-vpn/amneziawg-go/releases/latest/download/amneziawg-linux-amd64 -o /usr/local/bin/amneziawg
chmod +x /usr/local/bin/amneziawg

# Generate keys
amneziawg genkey | tee /etc/amneziawg/server_private.key | amneziawg pubkey > /etc/amneziawg/server_public.key

# awg0.conf
cat > /etc/amneziawg/awg0.conf <<'EOF'
[Interface]
Address = 10.0.1.1/24
ListenPort = 51821
PrivateKey = <AWG_PRIVATE_KEY>
Jc = 3
Jmin = 40
Jmax = 70
S1 = 0
S2 = 0
H1 = 1
H2 = 2
H3 = 3
H4 = 4
PostUp = iptables -A FORWARD -i awg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i awg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
EOF

systemctl enable --now amneziawg@awg0
```

### 1.5 Prometheus + Grafana (Docker Compose)

**File: `docker/observability/docker-compose.yml`**
```yaml
version: '3.8'
services:
  prometheus:
    image: prom/prometheus:v2.47
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prom_data:/prometheus
    ports:
      - "9090:9090"
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'

  grafana:
    image: grafana/grafana:10.1
    volumes:
      - grafana_data:/var/lib/grafana
      - ./dashboards:/etc/grafana/provisioning/dashboards
      - ./datasources:/etc/grafana/provisioning/datasources
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=changeme
      - GF_USERS_ALLOW_SIGN_UP=false

  node-exporter:
    image: prom/node-exporter:v1.6
    network_mode: host
    pid: host
    volumes:
      - /:/host:ro,rslave
    command:
      - '--path.rootfs=/host'

volumes:
  prom_data:
  grafana_data:
```

**Prometheus config (`prometheus.yml`):**
```yaml
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: 'node'
    static_configs:
      - targets: ['sgp:9100', 'fsn:9100']
```

**Deploy:**
```bash
cd docker/observability
docker compose up -d
# Grafana at http://<your-ip>:3000
```

### 1.6 Client Test

On phone: WireGuard app → Add tunnel:
```
[Interface]
PrivateKey = <CLIENT_PRIVATE_KEY>
Address = 10.0.0.2/24, fd00::2/64
DNS = 1.1.1.1

[Peer]
PublicKey = <SERVER_PUBLIC_KEY>
Endpoint = <SERVER_PUBLIC_IP>:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
```

**Verify:** `curl https://ip.snowradar.app` shows server IP.

---

## Phase 2: Control Plane / Go Backend (Weeks 3-4)

### 2.1 Project Structure

```
snowradar-api/
├── cmd/api/main.go
├── internal/
│   ├── config/
│   ├── database/
│   ├── handler/
│   ├── middleware/
│   ├── model/
│   ├── repository/
│   ├── service/
│   └── wgctrl/
├── migrations/
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── go.sum
```

### 2.2 Dependencies

```go
// go.mod
module github.com/snow-radar/snowradar-api

go 1.22

require (
    github.com/gin-gonic/gin v1.9
    github.com/golang-migrate/migrate/v4 v4.17
    github.com/jackc/pgx/v5 v5.5
    github.com/redis/go-redis/v9 v9.3
    github.com/rs/zerolog v1.32
    golang.zx2c4.com/wireguard/wgctrl v0.0.0-20231212
    golang.org/x/crypto v0.17
)
```

### 2.3 Database Schema

**Migration `001_init.sql`:**
```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    public_key TEXT UNIQUE NOT NULL,
    assigned_ip INET UNIQUE,
    server_id TEXT NOT NULL, -- 'sgp' or 'fsn'
    protocol TEXT DEFAULT 'wireguard', -- 'wireguard' or 'amneziawg'
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT true
);

CREATE TABLE servers (
    id TEXT PRIMARY KEY, -- 'sgp', 'fsn'
    name TEXT NOT NULL,
    endpoint TEXT NOT NULL, -- '1.2.3.4:51820'
    public_key TEXT NOT NULL,
    amneziawg_public_key TEXT,
    amneziawg_port INT DEFAULT 51821,
    amneziawg_params JSONB,
    subnet_cidr CIDR NOT NULL DEFAULT '10.0.0.0/24',
    amneziawg_subnet_cidr CIDR DEFAULT '10.0.1.0/24',
    is_active BOOLEAN DEFAULT true
);

INSERT INTO servers (id, name, endpoint, public_key, subnet_cidr) VALUES
('sgp', 'Singapore', 'sgp.snowradar.app:51820', '<SGP_PUBKEY>', '10.0.0.0/24'),
('fsn', 'Frankfurt (Falkenstein)', 'fsn.snowradar.app:51820', '<FSN_PUBKEY>', '10.0.0.0/24');
```

### 2.4 IP Allocation Service

```go
// internal/service/ipalloc.go
func (s *IPAllocator) Allocate(serverID string) (net.IP, error) {
    // Find next free IP in server's subnet
    // Use SELECT ... FOR UPDATE SKIP LOCKED on a sequence table
    // or simple: find max(assigned_ip) + 1 where not in use
}
```

### 2.5 wgctrl Integration

```go
// internal/wgctrl/manager.go
func (m *Manager) AddPeer(serverID, pubKey string, allowedIPs []net.IPNet) error {
    client, err := wgctrl.New()
    if err != nil { return err }
    defer client.Close()

    device, err := client.Device("wg0")
    if err != nil { return err }

    peer := wgtypes.PeerConfig{
        PublicKey:  pubKey,
        AllowedIPs: allowedIPs,
    }
    return client.ConfigureDevice("wg0", wgtypes.Config{
        ReplacePeers: false,
        Peers:        []wgtypes.PeerConfig{peer},
    })
}
```

### 2.6 API Handlers

```go
// POST /api/v1/connect
type ConnectRequest struct {
    DevicePublicKey string `json:"public_key" binding:"required"`
    ServerID        string `json:"server_id" binding:"required,oneof=sgp fsn"`
    Protocol        string `json:"protocol" binding:"omitempty,oneof=wireguard amneziawg"`
}

func (h *Handler) Connect(c *gin.Context) {
    var req ConnectRequest
    if err := c.ShouldBindJSON(&req); err != nil { ... }

    // 1. Find or create device record
    // 2. Allocate IP
    // 3. Add peer via wgctrl
    // 4. Return config
    c.JSON(200, ConnectResponse{
        AssignedIP:     ip.String(),
        ServerPublicKey: server.PublicKey,
        ServerEndpoint:  server.Endpoint,
        DNSServers:      []string{"1.1.1.1", "1.0.0.1"},
    })
}
```

### 2.7 Rate Limiting (Redis)

```go
// internal/middleware/ratelimit.go
func RateLimit(redisClient *redis.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := "ratelimit:" + c.ClientIP()
        count, _ := redisClient.Incr(c, key).Result()
        if count == 1 {
            redisClient.Expire(c, key, time.Minute)
        }
        if count > 30 {
            c.AbortWithStatusJSON(429, gin.H{"error": "rate limited"})
            return
        }
        c.Next()
    }
}
```

### 2.8 Docker Compose for Local Dev

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: snowradar
      POSTGRES_USER: snowradar
      POSTGRES_PASSWORD: devpass
    ports: ["5432:5432"]
    volumes: [pg_data:/var/lib/postgresql/data]

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    volumes: [redis_data:/data]

  api:
    build: .
    ports: ["8080:8080"]
    environment:
      DATABASE_URL: postgres://snowradar:devpass@postgres:5432/snowradar?sslmode=disable
      REDIS_URL: redis://redis:6379
      WG_INTERFACE: wg0
    depends_on: [postgres, redis]
    cap_add: [NET_ADMIN, SYS_MODULE] # for wgctrl in container
```

### 2.9 Acceptance Tests

```bash
# Start stack
docker compose up -d

# Run migrations
migrate -path migrations -database "postgres://..." up

# Test connect
curl -X POST localhost:8080/api/v1/connect \
  -H "Content-Type: application/json" \
  -d '{"public_key":"<CLIENT_PUB>", "server_id":"sgp"}'

# Verify peer added
wg show wg0
```

---

## Phase 3: Flutter Client MVP (Weeks 5-6)

### 3.1 Project Setup

```bash
flutter create --org com.snowradar --platforms=android,ios,macos,windows,linux snowradar_client
cd snowradar_client
flutter pub add flutter_riverpod freezed_annotation json_annotation flutter_secure_storage wireguard_flutter http
flutter pub add dev:build_runner freezed json_serializable riverpod_generator
```

### 3.2 State Management (Riverpod)

```dart
// lib/core/providers/vpn_provider.dart
@riverpod
class VpnController extends _$VpnController {
  @override
  VpnState build() => VpnState.disconnected();

  Future<void> connect(Server server) async {
    state = VpnState.connecting();
    try {
      final keyPair = await _generateKeyPair();
      await _storePrivateKey(keyPair.privateKey);
      final response = await _api.connect(keyPair.publicKey, server.id);
      await _activateTunnel(response, keyPair.privateKey);
      state = VpnState.connected(server, response.assignedIp);
    } catch (e) {
      state = VpnState.error(e.toString());
    }
  }
}
```

### 3.3 Key Generation & Storage

```dart
// lib/core/crypto/keys.dart
Future<KeyPair> generateKeyPair() async {
  final privateKey = await WireguardFlutter.generatePrivateKey();
  final publicKey = await WireguardFlutter.getPublicKey(privateKey);
  return KeyPair(privateKey: privateKey, publicKey: publicKey);
}

Future<void> storePrivateKey(String key) async {
  const storage = FlutterSecureStorage();
  await storage.write(key: 'wg_private_key', value: key);
}
```

### 3.4 Tunnel Activation

```dart
// lib/core/vpn/tunnel.dart
Future<void> activateTunnel(ConnectResponse response, String privateKey) async {
  final config = WireguardConfig(
    interface: InterfaceConfig(
      privateKey: privateKey,
      addresses: [response.assignedIp],
      dns: response.dnsServers,
    ),
    peers: [
      PeerConfig(
        publicKey: response.serverPublicKey,
        endpoint: response.serverEndpoint,
        allowedIPs: ['0.0.0.0/0', '::/0'],
        persistentKeepalive: 25,
      ),
    ],
  );
  await WireguardFlutter.addTunnel('snowradar', config);
  await WireguardFlutter.setTunnelEnabled('snowradar', true);
}
```

### 3.5 UI

```dart
// lib/features/home/home_screen.dart
class HomeScreen extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(vpnControllerProvider);
    final servers = ref.watch(serversProvider);

    return Scaffold(
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            DropdownButton<Server>(
              value: state.selectedServer,
              items: servers.map((s) => DropdownMenuItem(value: s, child: Text(s.name))).toList(),
              onChanged: state.isConnecting ? null : (s) => ref.read(vpnControllerProvider.notifier).selectServer(s!),
            ),
            const SizedBox(height: 32),
            FilledButton.tonalIcon(
              icon: Icon(state.isConnected ? Icons.power_off : Icons.power),
              label: Text(state.isConnected ? 'Disconnect' : 'Connect'),
              onPressed: state.isConnecting ? null : () => state.isConnected
                  ? ref.read(vpnControllerProvider.notifier).disconnect()
                  : ref.read(vpnControllerProvider.notifier).connect(state.selectedServer!),
            ),
            if (state.isError) Text(state.error!, style: TextStyle(color: Colors.red)),
          ],
        ),
      ),
    );
  }
}
```

### 3.6 Acceptance

```bash
flutter build apk --release
# Install on Android device
# Select Singapore → Connect
# Verify: browser shows Singapore IP
# Disconnect → verify original IP restored
```

---

## Phase 4: Stealth, Security & CI/CD (Week 7)

### 4.1 AmneziaWG Toggle

**Backend:** Add `protocol` field to connect request, switch `wgctrl` interface name (`wg0` vs `awg0`), use AmneziaWG params.

**Client:** Add `StealthMode` switch in settings. When enabled:
- Use AmneziaWG config format (Junk packets, S1/S2, H1-H4)
- Call backend with `protocol: 'amneziawg'`

### 4.2 GitHub Actions - Go API

```yaml
# .github/workflows/api.yml
name: Build & Push API
on:
  push:
    branches: [main]
    paths: ['snowradar-api/**']
jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: Build
        working-directory: snowradar-api
        run: CGO_ENABLED=0 go build -o api ./cmd/api
      - name: Docker build
        run: |
          docker build -t ghcr.io/snow-radar/snowradar-api:${{ github.sha }} .
      - name: Push
        run: |
          echo ${{ secrets.GITHUB_TOKEN }} | docker login ghcr.io -u ${{ github.actor }} --password-stdin
          docker push ghcr.io/snow-radar/snowradar-api:${{ github.sha }}
```

### 4.3 GitHub Actions - Flutter APK

```yaml
# .github/workflows/client.yml
name: Build Flutter APK
on:
  push:
    branches: [main]
    paths: ['snowradar-client/**']
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: subosito/flutter-action@v2
        with: { flutter-version: '3.22', channel: 'stable' }
      - name: Get deps
        working-directory: snowradar-client
        run: flutter pub get
      - name: Build APK
        working-directory: snowradar-client
        run: flutter build apk --release --split-per-abi
      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: apk
          path: snowradar-client/build/app/outputs/flutter-apk/*.apk
```

### 4.4 Legal Documents

- `PRIVACY_POLICY.md`: No logs, no bandwidth tracking, email only for billing
- `AUP.md`: No illegal activity, no spam, no Tor exit abuse
- Host at `/legal` on landing page

---

## Phase 5: Alpha Launch & Observability (Week 8)

### 5.1 Distribution

```bash
# Download APK from GitHub Actions artifact
# Share via WhatsApp/Discord/Telegram
# Install: adb install app-arm64-v8a-release.apk
```

### 5.2 Tester Onboarding

- 5 testers per ISP: Dialog, SLT, Mobitel, Hutch
- Each gets unique device name in app
- Google Form: `https://forms.gle/...` with fields: ISP, Device, Issue, Screenshot

### 5.3 Monitoring Checklist

- [ ] Grafana alerts: bandwidth > 80% of NIC, CPU > 80%, memory > 85%
- [ ] Block port 25 (SMTP) on both servers: `iptables -A FORWARD -p tcp --dport 25 -j DROP`
- [ ] Log rotation configured for WireGuard (`/var/log/wireguard.log`)
- [ ] Backup: daily `pg_dump` to object storage

### 5.4 Devlog

Write: "How I built a DPI-bypassing VPN for Sri Lanka using AmneziaWG and Go"
Publish on: personal blog, Dev.to, Hacker News, Reddit r/srilanka, r/privacytoolsIO

---

## Rollback Points

| Phase | Rollback Command |
|-------|-----------------|
| 0 | `terraform destroy` |
| 1 | `systemctl stop wg-quick@wg0 amneziawg@awg0; docker compose down` |
| 2 | `docker compose down -v` (removes DB) |
| 3 | Uninstall app |
| 4 | Revert GitHub Actions, remove stealth code |
| 5 | Revoke testers' device keys via API |

---

## Definition of Done (Per Phase)

- [ ] All tasks checked
- [ ] Acceptance tests pass
- [ ] Documentation updated
- [ ] ADR written (if architectural decision made)
- [ ] Code reviewed and merged to `main`
- [ ] CI pipeline green