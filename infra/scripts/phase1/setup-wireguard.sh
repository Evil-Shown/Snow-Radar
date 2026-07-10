#!/bin/bash
# setup-wireguard.sh - Configure WireGuard wg0 with dual-stack (IPv4 + IPv6) and leak-proof iptables
# Usage: sudo ./setup-wireguard.sh [server_ipv4] [server_ipv6] [wg_port]
# Example: sudo ./setup-wireguard.sh 1.2.3.4 2001:db8::1 51820

set -euo pipefail

SERVER_IPV4="${1:-}"
SERVER_IPV6="${2:-}"
WG_PORT="${3:-51820}"
WG_INTERFACE="wg0"
WG_SUBNET_V4="10.0.0.0/24"
WG_SUBNET_V6="fd00:dead:beef::/64"
WG_SERVER_IP_V4="10.0.0.1"
WG_SERVER_IP_V6="fd00:dead:beef::1"

# Auto-detect primary interface if not provided
if [[ -z "$SERVER_IPV4" ]]; then
    SERVER_IPV4=$(ip route get 1.1.1.1 | awk '{print $7; exit}')
    echo "Auto-detected IPv4: $SERVER_IPV4"
fi

# Auto-detect IPv6 if available
if [[ -z "$SERVER_IPV6" ]]; then
    SERVER_IPV6=$(ip -6 route get 2001:4860:4860::8888 2>/dev/null | awk '{print $7; exit}' || true)
    if [[ -n "$SERVER_IPV6" ]]; then
        echo "Auto-detected IPv6: $SERVER_IPV6"
    else
        echo "No IPv6 detected, continuing with IPv4 only"
    fi
fi

# Detect physical interface
PHYS_IFACE=$(ip route get 1.1.1.1 | awk '{print $5; exit}')
echo "Physical interface: $PHYS_IFACE"

# Install WireGuard if not present
if ! command -v wg &> /dev/null; then
    echo "Installing WireGuard..."
    apt update && apt install -y wireguard wireguard-tools qrencode
fi

# Create WireGuard directory
mkdir -p /etc/wireguard
chmod 700 /etc/wireguard

# Generate server keys if they don't exist
if [[ ! -f /etc/wireguard/server_private.key ]]; then
    echo "Generating server keys..."
    wg genkey | tee /etc/wireguard/server_private.key | wg pubkey > /etc/wireguard/server_public.key
    chmod 600 /etc/wireguard/server_private.key
fi

SERVER_PRIVATE_KEY=$(cat /etc/wireguard/server_private.key)
SERVER_PUBLIC_KEY=$(cat /etc/wireguard/server_public.key)

echo "Server Public Key: $SERVER_PUBLIC_KEY"

# Create wg0.conf
cat > /etc/wireguard/${WG_INTERFACE}.conf <<EOF
[Interface]
Address = ${WG_SERVER_IP_V4}/24, ${WG_SERVER_IP_V6}/64
ListenPort = ${WG_PORT}
PrivateKey = ${SERVER_PRIVATE_KEY}
SaveConfig = false

# PostUp/PostDown for dual-stack NAT and leak prevention
PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -t nat -A POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE; ip6tables -A FORWARD -i %i -j ACCEPT; ip6tables -t nat -A POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -t nat -D POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE; ip6tables -D FORWARD -i %i -j ACCEPT; ip6tables -t nat -D POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE

# Additional leak prevention rules (applied via PostUp)
PostUp = iptables -A FORWARD -i %i -o ${PHYS_IFACE} -j ACCEPT; iptables -A FORWARD -i ${PHYS_IFACE} -o %i -m state --state RELATED,ESTABLISHED -j ACCEPT; ip6tables -A FORWARD -i %i -o ${PHYS_IFACE} -j ACCEPT; ip6tables -A FORWARD -i ${PHYS_IFACE} -o %i -m state --state RELATED,ESTABLISHED -j ACCEPT
PostDown = iptables -D FORWARD -i %i -o ${PHYS_IFACE} -j ACCEPT; iptables -D FORWARD -i ${PHYS_IFACE} -o %i -m state --state RELATED,ESTABLISHED -j ACCEPT; ip6tables -D FORWARD -i %i -o ${PHYS_IFACE} -j ACCEPT; ip6tables -D FORWARD -i ${PHYS_IFACE} -o %i -m state --state RELATED,ESTABLISHED -j ACCEPT
EOF

chmod 600 /etc/wireguard/${WG_INTERFACE}.conf

# Enable and start WireGuard
systemctl enable wg-quick@${WG_INTERFACE}
systemctl restart wg-quick@${WG_INTERFACE}

# Verify
echo "=== WireGuard Status ==="
wg show ${WG_INTERFACE}

echo "=== iptables NAT rules ==="
iptables -t nat -L POSTROUTING -v -n | grep -E "(MASQUERADE|${WG_SUBNET_V4})"

if [[ -n "$SERVER_IPV6" ]]; then
    echo "=== ip6tables NAT rules ==="
    ip6tables -t nat -L POSTROUTING -v -n | grep -E "(MASQUERADE|${WG_SUBNET_V6})"
fi

echo ""
echo "Server Public Key (for clients): $SERVER_PUBLIC_KEY"
echo "Server Endpoint: ${SERVER_IPV4}:${WG_PORT}"
if [[ -n "$SERVER_IPV6" ]]; then
    echo "Server Endpoint IPv6: [${SERVER_IPV6}]:${WG_PORT}"
fi
echo ""
echo "Client config template:"
cat <<CLIENTEOF
[Interface]
PrivateKey = <CLIENT_PRIVATE_KEY>
Address = 10.0.0.X/24, fd00:dead:beef::X/64
DNS = 1.1.1.1, 1.0.0.1

[Peer]
PublicKey = ${SERVER_PUBLIC_KEY}
Endpoint = ${SERVER_IPV4}:${WG_PORT}
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
CLIENTEOF