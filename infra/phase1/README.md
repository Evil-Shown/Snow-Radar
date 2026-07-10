# Phase 1: Bare-Metal VPN & Observability

This directory contains all scripts and configurations for Phase 1 of Snow Radar deployment.

## Contents

```
phase1/
├── scripts/
│   ├── phase1-bootstrap.sh       # Master script (run on each server)
│   ├── 01-ssh-hardening.sh       # SSH hardening + fail2ban
│   ├── setup-wireguard.sh        # WireGuard wg0 (dual-stack)
│   ├── setup-amneziawg.sh        # AmneziaWG awg0 (DPI bypass)
│   └── 05-node-exporter.sh       # Prometheus Node Exporter
├── configs/
│   └── 99-snowradar-sysctl.conf  # Kernel hardening
```

## Quick Start (on each VPN server)

```bash
# 1. Copy scripts to server
scp -r infra/phase1/scripts snowradar@<SERVER_IP>:~/

# 2. SSH to server
ssh snowradar@<SERVER_IP>

# 3. Run bootstrap (auto-detects Oracle/Hetzner)
sudo ~/phase1/scripts/phase1-bootstrap.sh

# OR specify provider explicitly:
sudo ~/phase1/scripts/phase1-bootstrap.sh oracle
sudo ~/phase1/scripts/phase1-bootstrap.sh hetzner
```

## What Gets Configured

### SSH Hardening
- Creates `snowradar` sudo user (Hetzner starts as root)
- Disables root login, password auth
- Key-only authentication
- fail2ban: 3 failures → 1hr ban

### Kernel Hardening (sysctl)
- IPv4/IPv6 forwarding enabled
- RP filter strict, no redirects
- TCP SYN cookies, increased conntrack
- ASLR, dmesg restrict, kptr restrict

### WireGuard (wg0) - Standard Protocol
- Port: 51820/udp
- Subnet: 10.10.0.0/24 (IPv4) + fd00:10:10::/64 (IPv6)
- Leak-proof iptables/ip6tables NAT
- Server keys in `/etc/wireguard/`

### AmneziaWG (awg0) - Stealth Mode
- Port: 51821/udp
- Subnet: 10.11.0.0/24
- Obfuscation: Jc=3, Jmin=40, Jmax=70, H1=1, H2=2, H3=3, H4=4
- For censorship resistance (DPI bypass)

### Prometheus Node Exporter
- Port: 9100/tcp
- Metrics: `/metrics`
- Systemd service with security hardening

## Local Observability Stack

```bash
cd infra/docker/observability
cp .env.example .env
# Edit .env with your passwords/webhooks
docker compose up -d
```

Services:
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin / $GRAFANA_ADMIN_PASSWORD)
- **Alertmanager**: http://localhost:9093

### Configure Prometheus Targets

Edit `prometheus/prometheus.yml` and replace:
- `ORACLE_SGP_IP` → Oracle Singapore public IP
- `HETZNER_FSN_IP` → Hetzner Falkenstein public IP

### Grafana Dashboard

Import `grafana/dashboards/snowradar-vpn-overview.json` or use auto-provisioning (already configured).

## Server Key Locations

After bootstrap, keys are saved to `/root/snowradar-keys/`:
- `wg0_public.key`, `wg0_private.key`
- `awg0_public.key`, `awg0_private.key`

## Verification

```bash
# Check WireGuard
wg show wg0
wg show awg0

# Check services
systemctl status wg-quick@wg0 wg-quick@awg0 node_exporter

# Check metrics
curl http://localhost:9100/metrics | grep node_network

# Check UFW
ufw status verbose
```

## Next Phase

After both servers pass verification, proceed to [Phase 2: Control Plane](../api/) - Go API for dynamic peer management.