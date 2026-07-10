# Snow Radar — Step-by-Step Setup Guide

This guide walks you through the complete setup from zero to running infrastructure.

---

## Prerequisites

### Accounts & Credentials
- [ ] GitHub account
- [ ] Domain registrar account (Namecheap, Cloudflare, Porkbun, etc.)
- [ ] Oracle Cloud account (Always Free tier)
- [ ] Hetzner Cloud account (€4.51/mo for CX22)
- [ ] SSH key pair: `ssh-keygen -t ed25519 -C "snowradar-infra"`

### Local Tools
| Tool | Version | Install |
|------|---------|---------|
| Git | ≥ 2.40 | `winget install Git.Git` / `brew install git` |
| Terraform | ≥ 1.6 | `winget install HashiCorp.Terraform` / `brew install terraform` |
| Go | ≥ 1.22 | `winget install GoLang.Go` / `brew install go` |
| Flutter | ≥ 3.22 | https://flutter.dev/docs/get-started/install |
| Docker | ≥ 24 | Docker Desktop |
| VS Code / IDE | - | - |

### Verify
```bash
git --version
terraform version
go version
flutter --version
docker --version
```

---

## Phase 0: Foundation (Week 1)

### 1. Create GitHub Organization & Repositories

1. Go to https://github.com/organizations/new
2. Organization name: `snow-radar`
3. Create three **private** repositories:
   - `snowradar-infra`
   - `snowradar-api`
   - `snowradar-client`

### 2. Clone Repositories Locally

```bash
mkdir -p ~/code/snowradar
cd ~/code/snowradar

gh repo clone snow-radar/snowradar-infra
gh repo clone snow-radar/snowradar-api
gh repo clone snow-radar/snowradar-client
```

### 3. Register Domain

1. Buy `snowradar.app` or `snowradar.io`
2. Add DNS records (update IPs after Terraform):
   - A `@` → `<oracle-ip>` (Singapore)
   - A `eu` → `<hetzner-ip>` (Falkenstein)
   - CNAME `www` → `@`
3. Set TTL to 300 (5 min) for fast updates during testing

### 4. Generate SSH Keys

```bash
# If you don't have one already
ssh-keygen -t ed25519 -C "snowradar-infra" -f ~/.ssh/snowradar
# Public key: ~/.ssh/snowradar.pub
# Private key: ~/.ssh/snowradar (keep secure!)
```

### 5. Oracle Cloud Setup

1. Sign up at https://cloud.oracle.com (Always Free tier)
2. Create API signing key:
   - Console → Profile → User Settings → API Keys → Add API Key
   - Download private key → save as `~/.oci/oci_api_key.pem`
   - Note the **Fingerprint**, **Tenancy OCID**, **User OCID**
3. Get Compartment OCID:
   - Identity → Compartments → Copy OCID of root compartment
4. Find Availability Domain:
   - Identity → Availability Domains → Copy name (e.g., `AD-1`)

### 6. Hetzner Cloud Setup

1. Sign up at https://console.hetzner.cloud
2. Create Project: `snowradar`
3. Generate API Token:
   - Security → API Tokens → Generate Token
   - Name: `terraform`, Read & Write
   - Copy token (shown once!)

### 7. Configure Terraform Variables

```bash
cd ~/code/snowradar/snowradar-infra/infra/terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:
```hcl
oci_tenancy_ocid       = "ocid1.tenancy.oc1..xxxx"
oci_user_ocid          = "ocid1.user.oc1..xxxx"
oci_fingerprint        = "xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx"
oci_private_key_path   = "~/.oci/oci_api_key.pem"
oci_region             = "ap-singapore-1"
oci_compartment_id     = "ocid1.compartment.oc1..xxxx"
oci_availability_domain = "AD-1"  # or empty for auto

hcloud_token = "your-hetzner-api-token"

ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... snowradar-infra"
```

### 8. Deploy Infrastructure

```bash
terraform init
terraform fmt
terraform validate
terraform plan
terraform apply
```

**Expected outputs:**
```
oracle_instance_public_ip = "1xx.xxx.xxx.xxx"
hetzner_server_public_ip  = "5.xxx.xxx.xxx"
```

### 9. Update DNS

Point your domain A records to these IPs.

### 10. Verify SSH Access

```bash
ssh -i ~/.ssh/snowradar ubuntu@<oracle-ip>
ssh -i ~/.ssh/snowradar ubuntu@<hetzner-ip>
```

---

## Phase 1: Server Hardening & VPN (Week 2)

### 1. Initial Server Setup (Run on BOTH servers)

```bash
ssh -i ~/.ssh/snowradar ubuntu@<SERVER_IP>
```

```bash
# Update & install basics
sudo apt update && sudo apt upgrade -y
sudo apt install -y ufw fail2ban unattended-upgrades wireguard wireguard-tools qrencode curl

# Create admin user (replace 'admin' with your preferred username)
sudo adduser --gecos "" admin
sudo usermod -aG sudo admin
sudo mkdir -p /home/admin/.ssh
sudo cp ~/.ssh/authorized_keys /home/admin/.ssh/
sudo chown -R admin:admin /home/admin/.ssh
sudo chmod 700 /home/admin/.ssh
sudo chmod 600 /home/admin/.ssh/authorized_keys

# Test admin SSH in NEW terminal
ssh -i ~/.ssh/snowradar admin@<SERVER_IP>
```

### 2. Harden SSH

```bash
sudo tee /etc/ssh/sshd_config.d/99-snowradar.conf > /dev/null <<'EOF'
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys
MaxAuthTries 3
ClientAliveInterval 300
ClientAliveCountMax 2
EOF

sudo systemctl reload ssh
```

### 3. Configure Firewall (UFW)

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp      # SSH
sudo ufw allow 80/tcp      # HTTP
sudo ufw allow 443/tcp     # HTTPS
sudo ufw allow 51820/udp   # WireGuard
sudo ufw allow 51821/udp   # AmneziaWG (optional)
sudo ufw enable
sudo ufw status verbose
```

### 4. Enable IP Forwarding

```bash
sudo tee /etc/sysctl.d/99-snowradar.conf > /dev/null <<'EOF'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
EOF

sudo sysctl --system
```

### 5. Generate WireGuard Keys

```bash
# Server keys
wg genkey | sudo tee /etc/wireguard/server_private.key
sudo chmod 600 /etc/wireguard/server_private.key
sudo cat /etc/wireguard/server_private.key | wg pubkey | sudo tee /etc/wireguard/server_public.key

# Save public key for later
cat /etc/wireguard/server_public.key
```

### 6. Create WireGuard Config (wg0)

```bash
# Get primary interface name
ip route | grep default | awk '{print $5}'
# Usually: eth0 (Oracle) or eth0 (Hetzner)

sudo tee /etc/wireguard/wg0.conf > /dev/null <<'EOF'
[Interface]
Address = 10.10.0.1/24, fd00:10:10::1/64
ListenPort = 51820
PrivateKey = <SERVER_PRIVATE_KEY>
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; ip6tables -A FORWARD -i wg0 -j ACCEPT; ip6tables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE; ip6tables -D FORWARD -i wg0 -j ACCEPT; ip6tables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
EOF

sudo chmod 600 /etc/wireguard/wg0.conf
```

Replace `<SERVER_PRIVATE_KEY>` with actual key.

### 7. Start WireGuard

```bash
sudo systemctl enable wg-quick@wg0
sudo systemctl start wg-quick@wg0
sudo wg show wg0
```

### 8. Install AmneziaWG (Optional but Recommended)

```bash
# Download latest release
cd /tmp
wget https://github.com/amnezia-vpn/amneziawg-go/releases/download/v0.1.1/amneziawg-go-linux-amd64.tar.gz  # x86
# or arm64 for Oracle ARM
tar -xzf amneziawg-go-linux-*.tar.gz
sudo mv amneziawg-go /usr/local/bin/
sudo chmod +x /usr/local/bin/amneziawg-go

# Generate AmneziaWG keys
amneziawg-go genkey | sudo tee /etc/wireguard/awg_private.key
sudo chmod 600 /etc/wireguard/awg_private.key
cat /etc/wireguard/awg_private.key | amneziawg-go pubkey | sudo tee /etc/wireguard/awg_public.key

# Create awg0 config
sudo tee /etc/wireguard/awg0.conf > /dev/null <<'EOF'
[Interface]
Address = 10.11.0.1/24
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

sudo systemctl enable wg-quick@awg0
sudo systemctl start wg-quick@awg0
```

### 9. Install Prometheus Node Exporter

```bash
sudo useradd --no-create-home --shell /bin/false node_exporter
wget https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-1.7.0.linux-amd64.tar.gz
tar -xzf node_exporter-*.tar.gz
sudo cp node_exporter-*/node_exporter /usr/local/bin/
sudo chown node_exporter:node_exporter /usr/local/bin/node_exporter

sudo tee /etc/systemd/system/node_exporter.service > /dev/null <<'EOF'
[Unit]
Description=Node Exporter
After=network.target
[Service]
User=node_exporter
ExecStart=/usr/local/bin/node_exporter
[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now node_exporter
curl localhost:9100/metrics | head
```

### 10. Test Phone Connection (Manual Peer)

On server, add a test peer temporarily:
```bash
# Generate client keys
wg genkey | tee client_private.key | wg pubkey > client_public.key
CLIENT_PUB=$(cat client_public.key)
CLIENT_PRIV=$(cat client_private.key)

# Add peer to running interface
sudo wg set wg0 peer $CLIENT_PUB allowed-ips 10.10.0.2/32,fd00:10:10::2/128

# Create client config
cat > ~/test-client.conf <<EOF
[Interface]
PrivateKey = $CLIENT_PRIV
Address = 10.10.0.2/24, fd00:10:10::2/64
DNS = 1.1.1.1, 1.0.0.1

[Peer]
PublicKey = $(cat /etc/wireguard/server_public.key)
Endpoint = <SERVER_PUBLIC_IP>:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
EOF

cat ~/test-client.conf
```

Scan QR code on phone: `qrencode -t ansiutf8 < ~/test-client.conf`

---

## Phase 2: Go Backend (Weeks 3-4)

### 1. Initialize Project

```bash
cd ~/code/snowradar/snowradar-api
go mod init github.com/snow-radar/snowradar-api
```

### 2. Create Directory Structure

```bash
mkdir -p cmd/api internal/{auth,server,wgctrl,store,config,middleware} migrations
```

### 3. Docker Compose for Local Dev

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
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U snowradar"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    volumes: [redis_data:/data]
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

  api:
    build: .
    ports: ["8080:8080"]
    environment:
      DATABASE_URL: postgres://snowradar:devpass@postgres:5432/snowradar?sslmode=disable
      REDIS_URL: redis://redis:6379
      JWT_SECRET: dev-secret-change-in-prod
      WG_INTERFACE: wg0
      SERVER_SGP_PUBLIC_KEY: <oracle-server-pubkey>
      SERVER_SGP_ENDPOINT: <oracle-ip>:51820
      SERVER_FSN_PUBLIC_KEY: <hetzner-server-pubkey>
      SERVER_FSN_ENDPOINT: <hetzner-ip>:51820
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    cap_add: [NET_ADMIN, SYS_MODULE]

volumes:
  pg_data:
  redis_data:
```

### 4. Database Migrations

```sql
-- migrations/001_initial.sql
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
    server_id TEXT NOT NULL,  -- 'sgp' or 'fsn'
    protocol TEXT NOT NULL DEFAULT 'wireguard', -- or 'amneziawg'
    assigned_ip INET,
    assigned_ip6 INET,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen TIMESTAMPTZ
);

CREATE TABLE servers (
    id TEXT PRIMARY KEY,  -- 'sgp', 'fsn'
    name TEXT NOT NULL,
    region TEXT NOT NULL,
    public_key TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    subnet_cidr CIDR NOT NULL,
    subnet_cidr6 CIDR NOT NULL,
    is_active BOOLEAN DEFAULT true
);

INSERT INTO servers (id, name, region, public_key, endpoint, subnet_cidr, subnet_cidr6) VALUES
('sgp', 'Singapore', 'APAC', '<ORACLE_PUBKEY>', '<ORACLE_IP>:51820', '10.10.0.0/24', 'fd00:10:10::/64'),
('fsn', 'Falkenstein', 'EU', '<HETZNER_PUBKEY>', '<HETZNER_IP>:51820', '10.11.0.0/24', 'fd00:10:11::/64');
```

### 5. Run Migrations

```bash
# Install migrate tool
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run
migrate -path migrations -database "postgres://snowradar:devpass@localhost:5432/snowradar?sslmode=disable" up
```

### 6. Implement Core Packages

See IMPLEMENTATION_GUIDE.md for detailed code. Key files:
- `internal/config/config.go` - env parsing
- `internal/store/postgres.go` - DB access
- `internal/wgctrl/manager.go` - peer management
- `internal/auth/jwt.go` - token handling
- `internal/server/handlers.go` - HTTP routes
- `cmd/api/main.go` - entry point

### 7. Test Locally

```bash
docker compose up -d
docker compose logs -f api

# Test connect
curl -X POST http://localhost:8080/api/v1/connect \
  -H "Content-Type: application/json" \
  -d '{"public_key":"<CLIENT_PUB>", "server_id":"sgp"}'

# Verify on server
ssh admin@<oracle-ip> "sudo wg show wg0"
```

---

## Phase 3: Flutter Client (Weeks 5-6)

### 1. Create Project

```bash
cd ~/code/snowradar/snowradar-client
flutter create --org com.snowradar --platforms=android,ios,macos,windows,linux .
```

### 2. Add Dependencies

```yaml
# pubspec.yaml
dependencies:
  flutter:
    sdk: flutter
  flutter_riverpod: ^2.4.0
  freezed_annotation: ^2.4.0
  json_annotation: ^4.8.0
  flutter_secure_storage: ^9.0.0
  wireguard_flutter: ^0.1.0
  http: ^1.1.0
  path_provider: ^2.1.0

dev_dependencies:
  build_runner: ^2.4.0
  freezed: ^2.4.0
  json_serializable: ^6.7.0
  riverpod_generator: ^2.3.0
```

```bash
flutter pub get
flutter pub run build_runner build --delete-conflicting-outputs
```

### 3. Implement Core Features

Follow IMPLEMENTATION_GUIDE.md sections 3.1-3.5 for:
- Key generation & secure storage
- API client with auth
- VPN tunnel activation
- Riverpod state management
- UI: server dropdown + connect button

### 4. Build & Test

```bash
# Android
flutter build apk --release --split-per-abi
# Output: build/app/outputs/flutter-apk/app-*-release.apk

# iOS (requires macOS)
flutter build ios --release
```

---

## Phase 4: CI/CD & Stealth (Week 7)

### 1. GitHub Actions - API

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
      - name: Build Docker
        run: |
          docker build -t ghcr.io/snow-radar/snowradar-api:${{ github.sha }} .
      - name: Push
        run: |
          echo ${{ secrets.GITHUB_TOKEN }} | docker login ghcr.io -u ${{ github.actor }} --password-stdin
          docker push ghcr.io/snow-radar/snowradar-api:${{ github.sha }}
```

### 2. GitHub Actions - Flutter

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
      - name: Upload
        uses: actions/upload-artifact@v4
        with:
          name: apk
          path: snowradar-client/build/app/outputs/flutter-apk/*.apk
```

### 3. AmneziaWG Integration

**Backend:** Add `protocol` field, switch `wgctrl` interface (`wg0` vs `awg0`).

**Client:** Add "Stealth Mode" toggle, use AmneziaWG config format when enabled.

---

## Phase 5: Alpha Launch (Week 8)

### 1. Distribute APK

```bash
# Download from GitHub Actions artifacts
# Share via WhatsApp/Telegram/Discord
```

### 2. Tester Recruitment

Target 20 testers across:
- Dialog (5)
- SLT (5)
- Mobitel (5)
- Hutch (5)

### 3. Monitoring Setup

```bash
# On control plane VM (or dedicated)
cd ~/code/snowradar/snowradar-infra/infra/monitoring
docker compose up -d
# Grafana: http://<ip>:3000 (admin/admin)
# Import dashboard: 1860 (Node Exporter Full)
```

### 4. Block Abuse

```bash
# On both VPN servers
sudo iptables -A FORWARD -p tcp --dport 25 -j DROP   # SMTP
sudo iptables -A FORWARD -p tcp --dport 465 -j DROP  # SMTPS
sudo iptables -A FORWARD -p tcp --dport 587 -j DROP  # Submission
sudo iptables-save | sudo tee /etc/iptables/rules.v4
```

### 5. Devlog

Write and publish: "How I built a DPI-bypassing VPN for Sri Lanka using AmneziaWG and Go"

---

## Quick Reference Commands

| Task | Command |
|------|---------|
| SSH to Oracle | `ssh -i ~/.ssh/snowradar admin@<oracle-ip>` |
| SSH to Hetzner | `ssh -i ~/.ssh/snowradar admin@<hetzner-ip>` |
| View WireGuard peers | `sudo wg show wg0` |
| Add peer manually | `sudo wg set wg0 peer <PUBKEY> allowed-ips 10.10.0.x/32` |
| Remove peer | `sudo wg set wg0 peer <PUBKEY> remove` |
| View logs | `journalctl -u wg-quick@wg0 -f` |
| Terraform destroy | `cd infra/terraform && terraform destroy` |
| API logs | `docker compose -f snowradar-api/docker-compose.yml logs -f api` |
| Flutter clean | `cd snowradar-client && flutter clean && flutter pub get` |

---

## Troubleshooting

| Issue | Fix |
|-------|-----|
| Terraform: OCI auth fails | Check `~/.oci/oci_api_key.pem` permissions (600), verify fingerprint |
| Terraform: Hetzner token invalid | Regenerate token in Hetzner console |
| WireGuard: No handshake | Check UDP 51820 open on cloud firewall + UFW, verify server public key in client config |
| WireGuard: Connected but no internet | Verify `net.ipv4.ip_forward=1`, check `PostUp` MASQUERADE rule matches interface |
| API: wgctrl fails | Run with `cap_add: [NET_ADMIN, SYS_MODULE]` in Docker |
| Flutter: Tunnel not starting | Check `WireguardFlutter.addTunnel` error, verify config format, check Android VPN permission |
| DNS leaks | Ensure client config has `DNS = 1.1.1.1`, test on dnsleaktest.com |

---

## Next Steps After Phase 5

1. **Automated provisioning**: Ansible playbooks for server config
2. **Multi-user billing**: Stripe integration, subscription tiers
3. **More regions**: US West, Japan, Brazil, Australia
4. **Protocol upgrades**: QUIC, VLESS, Shadowsocks
5. **Client features**: Split tunneling, per-app routing, kill switch UI
6. **Audit**: Third-party security review

---

## Support

- GitHub Issues: https://github.com/snow-radar/snowradar-infra/issues
- Discord: (create invite)
- Email: security@snowradar.app