#!/bin/bash
# Phase 1 Master Bootstrap Script
# Run on EACH VPN server after Terraform provisions them
# Usage: sudo ./phase1-bootstrap.sh [oracle|hetzner]

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[+]${NC} $*"; }
info() { echo -e "${BLUE}[i]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[-]${NC} $*"; exit 1; }

PROVIDER="${1:-auto}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Auto-detect provider
if [[ "$PROVIDER" == "auto" ]]; then
    if curl -s -H "Authorization: Bearer Oracle" -L http://169.254.169.254/opc/v2/instance/ >/dev/null 2>&1; then
        PROVIDER="oracle"
    elif curl -s http://169.254.169.254/hetzner/v1/ >/dev/null 2>&1; then
        PROVIDER="hetzner"
    else
        error "Could not auto-detect provider. Specify 'oracle' or 'hetzner'"
    fi
fi

info "Running Phase 1 bootstrap for provider: $PROVIDER"

# 1. System updates
log "Updating system packages..."
apt-get update -qq && apt-get upgrade -y -qq
apt-get install -y -qq curl wget gnupg2 software-properties-common ufw fail2ban qrencode

# 2. Run SSH hardening
log "Running SSH hardening..."
bash "$SCRIPT_DIR/01-ssh-hardening.sh"

# 3. Apply sysctl hardening
log "Applying kernel sysctl hardening..."
cp "$SCRIPT_DIR/../configs/99-snowradar-sysctl.conf" /etc/sysctl.d/
sysctl --system

# 4. Setup WireGuard (wg0)
log "Setting up WireGuard (wg0)..."
bash "$SCRIPT_DIR/setup-wireguard.sh"

# 5. Setup AmneziaWG (awg0)
log "Setting up AmneziaWG (awg0)..."
bash "$SCRIPT_DIR/setup-amneziawg.sh"

# 6. Install Node Exporter
log "Installing Prometheus Node Exporter..."
bash "$SCRIPT_DIR/05-node-exporter.sh"

# 7. Final UFW rules for VPN ports
log "Adding VPN ports to UFW..."
ufw allow 51820/udp comment 'WireGuard wg0'
ufw allow 51821/udp comment 'AmneziaWG awg0'
ufw reload

# 8. Save server keys for reference
log "Saving server keys..."
mkdir -p /root/snowradar-keys
cp /etc/wireguard/server_public.key /root/snowradar-keys/wg0_public.key
cp /etc/wireguard/server_private.key /root/snowradar-keys/wg0_private.key
cp /etc/amneziawg/server_public.key /root/snowradar-keys/awg0_public.key
cp /etc/amneziawg/server_private.key /root/snowradar-keys/awg0_private.key
chmod 600 /root/snowradar-keys/*

# 9. Display summary
echo ""
echo "=========================================="
echo "  Phase 1 Complete - Server Summary"
echo "=========================================="
echo ""
echo "Provider: $PROVIDER"
echo "Hostname: $(hostname)"
echo "Public IP: $(curl -s ifconfig.me || curl -s ipinfo.io/ip)"
echo ""
echo "WireGuard (wg0):"
echo "  Port: 51820/udp"
echo "  Public Key: $(cat /etc/wireguard/server_public.key)"
echo "  Subnet: 10.10.0.0/24, fd00:10:10::/64"
echo ""
echo "AmneziaWG (awg0):"
echo "  Port: 51821/udp"
echo "  Public Key: $(cat /etc/amneziawg/server_public.key)"
echo "  Subnet: 10.11.0.0/24"
echo "  Params: Jc=3 Jmin=40 Jmax=70 S1=0 S2=0 H1=1 H2=2 H3=3 H4=4"
echo ""
echo "Node Exporter:"
echo "  Port: 9100/tcp"
echo "  Metrics: http://$(hostname -I | awk '{print $1}'):9100/metrics"
echo ""
echo "Keys saved to: /root/snowradar-keys/"
echo ""
warn "IMPORTANT: Test SSH access in NEW terminal before closing this one!"
warn "ssh -i ~/.ssh/snow_radar_key snowradar@<this-ip>"
echo ""
info "Next steps:"
info "1. Verify both servers are up: wg show; systemctl status wg-quick@wg0 wg-quick@awg0 node_exporter"
info "2. From local machine, start Prometheus/Grafana: cd infra/docker/observability && docker compose up -d"
info "3. Update prometheus.yml with actual server IPs"
info "4. Import Grafana dashboard from grafana/dashboards/snowradar-vpn-overview.json"