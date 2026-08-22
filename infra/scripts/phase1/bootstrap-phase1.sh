#!/bin/bash
# bootstrap-phase1.sh - Master script to run Phase 1 on a fresh server
# Run as root on BOTH Oracle (SGP) and Hetzner (FSN) servers.
#
# Fix history (audit #5): previous version called ./scripts/setup-wireguard.sh
# relative to this file's directory, but the setup scripts live elsewhere.
# All scripts were consolidated into infra/scripts/phase1/ (one canonical tree)
# and all addressing comes from infra/configs/subnets.env (ADR-004).

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log() { echo -e "${GREEN}[PHASE1]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
step() { echo -e "${BLUE}[STEP]${NC} $*"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_INFRA_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CANONICAL_SCRIPTS="${SCRIPT_DIR}"
SUBNETS_FILE="${REPO_INFRA_DIR}/configs/subnets.env"
SYSCTL_CONF="${REPO_INFRA_DIR}/configs/99-snowradar-sysctl.conf"

[[ -f "$SUBNETS_FILE" ]] || error "Canonical subnet config missing: $SUBNETS_FILE"
[[ -f "$SYSCTL_CONF" ]] || error "Sysctl config missing: $SYSCTL_CONF"
[[ -f "${CANONICAL_SCRIPTS}/setup-wireguard.sh" ]] || error "setup-wireguard.sh not found in $CANONICAL_SCRIPTS"
[[ -f "${CANONICAL_SCRIPTS}/setup-amneziawg.sh" ]] || error "setup-amneziawg.sh not found in $CANONICAL_SCRIPTS"

# shellcheck source=../configs/subnets.env
source "$SUBNETS_FILE"

NODE="${1:-}"

detect_server() {
    if curl -sf --max-time 3 -H "Authorization: Bearer Oracle" http://169.254.169.254/opc/v2/instance/ >/dev/null 2>&1; then
        echo "sgp"
    elif curl -sf --max-time 3 http://169.254.169.254/hetzner/v1/ >/dev/null 2>&1; then
        echo "fsn"
    else
        echo "unknown"
    fi
}

if [[ -z "$NODE" ]]; then
    NODE=$(detect_server)
fi
[[ "$NODE" == "sgp" || "$NODE" == "fsn" ]] || error "Could not determine node. Pass 'sgp' or 'fsn' as first argument."
log "Target node: $NODE"

SERVER_IPV4="${2:-}"
WG_PORT="${WG_PORT:-51820}"
AWG_PORT="${AWG_PORT:-51821}"
WG_SERVER_V4_VAR="${NODE}_WG_SERVER_V4"; WG_SERVER_IP="${!WG_SERVER_V4_VAR:?missing in subnets.env}"

main() {
    log "=== Snow Radar Phase 1: Bare-Metal VPN & Observability ($NODE) ==="

    step "1/7: Updating system and installing base packages"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get upgrade -y -qq
    apt-get install -y -qq \
        wireguard wireguard-tools qrencode \
        ufw fail2ban curl wget unattended-upgrades \
        htop iftop iotop \
        ca-certificates gnupg lsb-release jq

    # SECURITY.md claim made real: automatic security updates
    dpkg-reconfigure -f noninteractive unattended-upgrades

    step "2/7: Applying kernel sysctl hardening"
    cp "$SYSCTL_CONF" /etc/sysctl.d/99-snowradar-sysctl.conf
    sysctl --system

    step "3/7: Hardening SSH"
    bash "${CANONICAL_SCRIPTS}/01-ssh-hardening.sh"

    step "4/7: Setting up WireGuard (wg0)"
    bash "${CANONICAL_SCRIPTS}/setup-wireguard.sh" "$NODE" "$SERVER_IPV4" ""

    step "5/7: Setting up AmneziaWG (awg0) for DPI bypass"
    bash "${CANONICAL_SCRIPTS}/setup-amneziawg.sh" "$NODE" "$SERVER_IPV4"

    step "6/7: Configuring UFW for VPN ports"
    ufw allow "${WG_PORT}"/udp comment 'WireGuard'
    ufw allow "${AWG_PORT}"/udp comment 'AmneziaWG'
    ufw reload

    step "7/7: Installing Prometheus Node Exporter (WireGuard-only bind)"
    bash "${CANONICAL_SCRIPTS}/05-node-exporter.sh" "$WG_SERVER_IP"

    log "=== Phase 1 Complete ($NODE) ==="
    echo ""
    echo "Next steps:"
    echo "1. Test WireGuard:   wg show wg0"
    echo "2. Test AmneziaWG:   awg show awg0 && systemctl status awg-quick@awg0"
    echo "3. Verify exporter:  curl http://${WG_SERVER_IP}:9100/metrics | head"
    echo "4. Update Prometheus scrape targets to the WireGuard IPs:"
    echo "     ${NODE}: ${WG_SERVER_IP}:9100"
    echo "5. Run monitoring stack locally: infra/docker/observability && docker compose up -d"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
