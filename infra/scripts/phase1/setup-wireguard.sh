#!/bin/bash
# setup-wireguard.sh - Configure WireGuard wg0 with dual-stack (IPv4 + IPv6) and leak-proof iptables
# Usage: sudo ./setup-wireguard.sh [sgp|fsn] [server_ipv4] [server_ipv6]
# Subnets/ports come from infra/configs/subnets.env (ADR-004). Do not edit values here.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUBNETS_FILE="${SCRIPT_DIR}/../../configs/subnets.env"

if [[ ! -f "$SUBNETS_FILE" ]]; then
    echo "FATAL: canonical subnet config not found at $SUBNETS_FILE" >&2
    exit 1
fi
# shellcheck source=../../configs/subnets.env
source "$SUBNETS_FILE"

NODE="${1:-}"
SERVER_IPV4="${2:-}"
SERVER_IPV6="${3:-}"
WG_PORT="${WG_PORT:-51820}"
WG_INTERFACE="${WG_IFACE_NAME:-wg0}"

# Resolve node block
resolve_node() {
    if [[ -n "$NODE" && "$NODE" != "sgp" && "$NODE" != "fsn" ]]; then
        echo "FATAL: first arg must be 'sgp' or 'fsn' (got '$NODE')" >&2
        exit 1
    fi
    if [[ -z "$NODE" ]]; then
        if curl -sf --max-time 3 -H "Authorization: Bearer Oracle" http://169.254.169.254/opc/v2/instance/ >/dev/null 2>&1; then
            NODE="sgp"
        elif curl -sf --max-time 3 http://169.254.169.254/hetzner/v1/ >/dev/null 2>&1; then
            NODE="fsn"
        else
            echo "FATAL: could not detect node (pass 'sgp' or 'fsn' as first argument)" >&2
            exit 1
        fi
    fi
}
resolve_node

WG_SUBNET_V4="${NODE}_WG_SUBNET_V4"; WG_SUBNET_V4="${!WG_SUBNET_V4:?missing in subnets.env}"
WG_SERVER_IP_V4="${NODE}_WG_SERVER_V4"; WG_SERVER_IP_V4="${!WG_SERVER_IP_V4:?missing in subnets.env}"
WG_SUBNET_V6="${NODE}_WG_SUBNET_V6"; WG_SUBNET_V6="${!WG_SUBNET_V6:?missing in subnets.env}"
WG_SERVER_IP_V6="${NODE}_WG_SERVER_V6"; WG_SERVER_IP_V6="${!WG_SERVER_IP_V6:?missing in subnets.env}"

echo "Node: $NODE  wg-subnet: $WG_SUBNET_V4  awg-sibling present (do NOT reuse this range)"

# Auto-detect primary IPv4 if not provided
if [[ -z "$SERVER_IPV4" ]]; then
    SERVER_IPV4=$(ip route get 1.1.1.1 | grep -oP 'src \K\S+' || ip route get 1.1.1.1 | awk '{print $7; exit}')
    echo "Auto-detected IPv4: $SERVER_IPV4"
fi
if [[ -z "$SERVER_IPV6" ]]; then
    # audit #9: field position in `ip -6 route get` output is not stable
    # across versions; match the src token explicitly instead of $7.
    SERVER_IPV6=$(ip -6 route get 2001:4860:4860::8888 2>/dev/null | grep -oP 'src \K\S+' || true)
    if [[ -n "$SERVER_IPV6" ]]; then
        echo "Auto-detected IPv6: $SERVER_IPV6"
    else
        echo "No IPv6 detected, continuing with IPv4 only"
    fi
fi

PHYS_IFACE=$(ip route get 1.1.1.1 | awk '{print $5; exit}')
echo "Physical interface: $PHYS_IFACE"

if ! command -v wg &> /dev/null; then
    echo "Installing WireGuard..."
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq && apt-get install -y -qq wireguard wireguard-tools qrencode
fi

mkdir -p /etc/wireguard
chmod 700 /etc/wireguard

if [[ ! -f /etc/wireguard/server_private.key ]]; then
    echo "Generating server keys..."
    umask 077
    wg genkey | tee /etc/wireguard/server_private.key | wg pubkey > /etc/wireguard/server_public.key
fi

SERVER_PRIVATE_KEY=$(cat /etc/wireguard/server_private.key)
SERVER_PUBLIC_KEY=$(cat /etc/wireguard/server_public.key)
echo "Server Public Key: $SERVER_PUBLIC_KEY"

cat > "/etc/wireguard/${WG_INTERFACE}.conf" <<EOF
[Interface]
Address = ${WG_SERVER_IP_V4}/24, ${WG_SERVER_IP_V6}/64
ListenPort = ${WG_PORT}
PrivateKey = ${SERVER_PRIVATE_KEY}
SaveConfig = false

PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -t nat -A POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE; ip6tables -A FORWARD -i %i -j ACCEPT; ip6tables -t nat -A POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -t nat -D POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE; ip6tables -D FORWARD -i %i -j ACCEPT; ip6tables -t nat -D POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE
EOF

chmod 600 "/etc/wireguard/${WG_INTERFACE}.conf"

systemctl enable "wg-quick@${WG_INTERFACE}"
systemctl restart "wg-quick@${WG_INTERFACE}"

echo "=== WireGuard Status ==="
wg show "${WG_INTERFACE}"

echo ""
echo "Server Public Key (for clients): $SERVER_PUBLIC_KEY"
echo "Server Endpoint: ${SERVER_IPV4}:${WG_PORT}"
[[ -n "$SERVER_IPV6" ]] && echo "Server Endpoint IPv6: [${SERVER_IPV6}]:${WG_PORT}"
echo ""
echo "Client config template (replace X with an allocated host octet, .2-.254):"
cat <<CLIENTEOF
[Interface]
PrivateKey = <CLIENT_PRIVATE_KEY>
Address = ${WG_SUBNET_V4%.*}.X/${WG_SUBNET_V4##*/}, ${WG_SUBNET_V6%::*}::X/${WG_SUBNET_V6##*/}
DNS = 1.1.1.1, 1.0.0.1

[Peer]
PublicKey = ${SERVER_PUBLIC_KEY}
Endpoint = ${SERVER_IPV4}:${WG_PORT}
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
CLIENTEOF
