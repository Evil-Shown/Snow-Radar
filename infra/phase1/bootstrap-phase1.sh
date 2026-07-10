#!/bin/bash
# bootstrap-phase1.sh - Master script to run Phase 1 on a fresh server
# Run as root on BOTH Oracle (SGP) and Hetzner (FSN) servers

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[PHASE1]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
step() { echo -e "${BLUE}[STEP]${NC} $*"; }

# Configuration - EDIT THESE FOR YOUR SERVERS
# Oracle Singapore (ARM)
ORACLE_SGP_IPV4=""
ORACLE_SGP_IPV6=""

# Hetzner Falkenstein (x86)
HETZNER_FSN_IPV4=""
HETZNER_FSN_IPV6=""

# WireGuard settings
WG_PORT=51820
AWG_PORT=51821
WG_SUBNET_V4="10.10.0.0/24"
WG_SUBNET_V6="fd00:10:10::/64"
AWG_SUBNET_V4="10.11.0.0/24"

# Detect which server we're on
detect_server() {
    if curl -s -H "Authorization: Bearer Oracle" -L http://169.254.169.254/opc/v2/instance/ >/dev/null 2>&1; then
        echo "oracle-sgp"
    elif curl -s http://169.254.169.254/hetzner/v1/ >/dev/null 2>&1; then
        echo "hetzner-fsn"
    else
        echo "unknown"
    fi
}

SERVER_TYPE=$(detect_server)
log "Detected server type: $SERVER_TYPE"

if [[ "$SERVER_TYPE" == "unknown" ]]; then
    warn "Could not auto-detect server type. Assuming manual run."
fi

# Main execution
main() {
    log "=== Snow Radar Phase 1: Bare-Metal VPN & Observability ==="
    log "Server: $SERVER_TYPE"

    # Step 1: System update and base packages
    step "1/7: Updating system and installing base packages"
    apt-get update -qq
    apt-get upgrade -y -qq
    apt-get install -y -qq \
        wireguard wireguard-tools qrencode \
        ufw fail2ban curl wget \
        htop iftop iotop net-tools \
        ca-certificates gnupg lsb-release \
        jq vim git

    # Step 2: Apply sysctl hardening
    step "2/7: Applying kernel sysctl hardening"
    cp "$(dirname "$0")/configs/99-snowradar-sysctl.conf" /etc/sysctl.d/99-snowradar-sysctl.conf
    sysctl --system

    # Step 3: SSH hardening
    step "3/7: Hardening SSH"
    bash "$(dirname "$0")/scripts/01-ssh-hardening.sh"

    # Step 4: WireGuard (wg0) setup
    step "4/7: Setting up WireGuard (wg0)"
    if [[ "$SERVER_TYPE" == "oracle-sgp" ]]; then
        SERVER_IPV4="$ORACLE_SGP_IPV4"
        SERVER_IPV6="$ORACLE_SGP_IPV6"
    elif [[ "$SERVER_TYPE" == "hetzner-fsn" ]]; then
        SERVER_IPV4="$HETZNER_FSN_IPV4"
        SERVER_IPV6="$HETZNER_FSN_IPV6"
    fi
    bash "$(dirname "$0")/scripts/setup-wireguard.sh" "$SERVER_IPV4" "$SERVER_IPV6" "$WG_PORT"

    # Step 5: AmneziaWG (awg0) setup
    step "5/7: Setting up AmneziaWG (awg0) for DPI bypass"
    bash "$(dirname "$0")/scripts/setup-amneziawg.sh" "$SERVER_IPV4" "$AWG_PORT"

    # Step 6: UFW rules for VPN ports
    step "6/7: Configuring UFW for VPN ports"
    ufw allow "$WG_PORT"/udp comment 'WireGuard'
    ufw allow "$AWG_PORT"/udp comment 'AmneziaWG'
    ufw reload

    # Step 7: Prometheus Node Exporter
    step "7/7: Installing Prometheus Node Exporter"
    bash "$(dirname "$0")/scripts/05-node-exporter.sh"

    log "=== Phase 1 Complete! ==="
    echo ""
    echo "Next steps:"
    echo "1. Test WireGuard: wg show wg0"
    echo "2. Test AmneziaWG: systemctl status amneziawg@awg0"
    echo "3. Verify Node Exporter: curl localhost:9100/metrics | head"
    echo "4. Update Docker Compose prometheus.yml with this server's IP"
    echo "5. Run local monitoring stack: cd infra/docker/observability && docker compose up -d"
    echo ""
    echo "Server keys saved to:"
    echo "  WireGuard: /etc/wireguard/server_public.key"
    echo "  AmneziaWG: /etc/amneziawg/server_public.key"
}

# Run main if not sourced
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi