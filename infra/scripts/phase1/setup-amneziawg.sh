#!/bin/bash
# setup-amneziawg.sh - Configure AmneziaWG (wg1) for censorship resistance
# Usage: sudo ./setup-amneziawg.sh [server_ipv4] [awg_port]
# Example: sudo ./setup-amneziawg.sh 1.2.3.4 51821

set -euo pipefail

SERVER_IPV4="${1:-}"
AWG_PORT="${2:-51821}"
AWG_INTERFACE="wg1"
AWG_SUBNET_V4="10.10.0.0/24"
AWG_SERVER_IP_V4="10.10.0.1"

# Auto-detect primary interface
if [[ -z "$SERVER_IPV4" ]]; then
    SERVER_IPV4=$(ip route get 1.1.1.1 | awk '{print $7; exit}')
    echo "Auto-detected IPv4: $SERVER_IPV4"
fi

PHYS_IFACE=$(ip route get 1.1.1.1 | awk '{print $5; exit}')
echo "Physical interface: $PHYS_IFACE"

# Install AmneziaWG (amneziawg-go)
if ! command -v amneziawg-go &> /dev/null; then
    echo "Installing AmneziaWG..."
    ARCH=$(uname -m)
    case $ARCH in
        x86_64) AWG_ARCH="amd64" ;;
        aarch64) AWG_ARCH="arm64" ;;
        *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    
    # Get latest release
    AWG_VERSION=$(curl -s https://api.github.com/repos/amnezia-vpn/amneziawg-go/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
    if [[ -z "$AWG_VERSION" ]]; then
        AWG_VERSION="v0.1.1"  # fallback
    fi
    
    cd /tmp
    wget -q "https://github.com/amnezia-vpn/amneziawg-go/releases/download/${AWG_VERSION}/amneziawg-go-linux-${AWG_ARCH}.tar.gz"
    tar -xzf "amneziawg-go-linux-${AWG_ARCH}.tar.gz"
    mv amneziawg-go /usr/local/bin/
    chmod +x /usr/local/bin/amneziawg-go
    rm -f "amneziawg-go-linux-${AWG_ARCH}.tar.gz"
    echo "AmneziaWG ${AWG_VERSION} installed"
fi

# Create WireGuard directory (keys stored here for both wg0 and wg1)
mkdir -p /etc/wireguard
chmod 700 /etc/wireguard

# Generate keys if they don't exist
if [[ ! -f /etc/wireguard/wg1_private.key ]]; then
    echo "Generating AmneziaWG server keys..."
    amneziawg-go genkey | tee /etc/wireguard/wg1_private.key | amneziawg-go pubkey > /etc/wireguard/wg1_public.key
    chmod 600 /etc/wireguard/wg1_private.key
fi

SERVER_PRIVATE_KEY=$(cat /etc/wireguard/wg1_private.key)
SERVER_PUBLIC_KEY=$(cat /etc/wireguard/wg1_public.key)

echo "AmneziaWG Server Public Key: $SERVER_PUBLIC_KEY"

# AmneziaWG parameters (Jc, Jmin, Jmax, S1, S2, H1-H4)
# These are the obfuscation parameters - clients MUST use matching values
JC=3
JMIN=40
JMAX=70
S1=0
S2=0
H1=1
H2=2
H3=3
H4=4

# Create wg1.conf in /etc/wireguard/
cat > /etc/wireguard/${AWG_INTERFACE}.conf <<EOF
[Interface]
Address = ${AWG_SERVER_IP_V4}/24
ListenPort = ${AWG_PORT}
PrivateKey = ${SERVER_PRIVATE_KEY}
Jc = ${JC}
Jmin = ${JMIN}
Jmax = ${JMAX}
S1 = ${S1}
S2 = ${S2}
H1 = ${H1}
H2 = ${H2}
H3 = ${H3}
H4 = ${H4}
SaveConfig = false

PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -t nat -A POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -t nat -D POSTROUTING -o ${PHYS_IFACE} -j MASQUERADE

PostUp = iptables -A FORWARD -i %i -o ${PHYS_IFACE} -j ACCEPT; iptables -A FORWARD -i ${PHYS_IFACE} -o %i -m state --state RELATED,ESTABLISHED -j ACCEPT
PostDown = iptables -D FORWARD -i %i -o ${PHYS_IFACE} -j ACCEPT; iptables -D FORWARD -i ${PHYS_IFACE} -o %i -m state --state RELATED,ESTABLISHED -j ACCEPT
EOF

chmod 600 /etc/wireguard/${AWG_INTERFACE}.conf

# Create systemd service for amneziawg-go
cat > /etc/systemd/system/amneziawg@.service <<'EOF'
[Unit]
Description=AmneziaWG tunnel %I
After=network.target
Wants=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/amneziawg-go -f %i
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable amneziawg@${AWG_INTERFACE}
systemctl restart amneziawg@${AWG_INTERFACE}

# Verify
sleep 2
echo "=== AmneziaWG Status ==="
systemctl status amneziawg@${AWG_INTERFACE} --no-pager

echo "=== iptables NAT rules ==="
iptables -t nat -L POSTROUTING -v -n | grep -E "(MASQUERADE|${AWG_SUBNET_V4})"

echo ""
echo "AmneziaWG Server Public Key: $SERVER_PUBLIC_KEY"
echo "Server Endpoint: ${SERVER_IPV4}:${AWG_PORT}"
echo "Obfuscation Parameters:"
echo "  Jc=${JC} Jmin=${JMIN} Jmax=${JMAX}"
echo "  S1=${S1} S2=${S2}"
echo "  H1=${H1} H2=${H2} H3=${H3} H4=${H4}"
echo ""
echo "Client config template:"
cat <<CLIENTEOF
[Interface]
PrivateKey = <CLIENT_PRIVATE_KEY>
Address = 10.10.0.X/24
DNS = 1.1.1.1, 1.0.0.1
Jc = ${JC}
Jmin = ${JMIN}
Jmax = ${JMAX}
S1 = ${S1}
S2 = ${S2}
H1 = ${H1}
H2 = ${H2}
H3 = ${H3}
H4 = ${H4}

[Peer]
PublicKey = ${SERVER_PUBLIC_KEY}
Endpoint = ${SERVER_IPV4}:${AWG_PORT}
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
CLIENTEOF