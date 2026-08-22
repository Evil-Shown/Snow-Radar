#!/bin/bash
# setup-amneziawg.sh - Configure AmneziaWG stealth tunnel (awg0) for censorship resistance
#
# Install method (verified against upstream, Aug 2026):
#   - Userspace tools (`awg`, `awg-quick`) + DKMS kernel module from the official
#     Amnezia Launchpad PPA (ppa:amnezia/ppa), fingerprint-checked.
#   - Key generation uses `awg genkey | awg pubkey` (amneziawg-tools).
#     NOTE: `amneziawg-go` is a daemon and has NO genkey/pubkey subcommands —
#     the previous version of this script was broken because of that.
# Usage: sudo ./setup-amneziawg.sh [sgp|fsn] [server_ipv4]
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
AWG_PORT="${AWG_PORT:-51821}"
AWG_INTERFACE="${AWG_IFACE_NAME:-awg0}"

# AmneziaWG obfuscation parameters.
# WARNING: these MUST match the client exactly, or handshakes silently fail.
# AWG 1.x parameter set (Jc/Jmin/Jmax/S1/S2/H1-H4). If you upgrade to an
# AWG 2.0-era server/client, revisit this set upstream before changing it here.
JC=4; JMIN=40; JMAX=70; S1=15; S2=68; H1=1110000001; H2=2220000002; H3=3330000003; H4=4440000004

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

AWG_SUBNET_V4="${NODE}_AWG_SUBNET_V4"; AWG_SUBNET_V4="${!AWG_SUBNET_V4:?missing in subnets.env}"
AWG_SERVER_IP_V4="${NODE}_AWG_SERVER_V4"; AWG_SERVER_IP_V4="${!AWG_SERVER_IP_V4:?missing in subnets.env}"

if [[ -z "$SERVER_IPV4" ]]; then
    SERVER_IPV4=$(ip route get 1.1.1.1 | awk '{print $7; exit}')
    echo "Auto-detected IPv4: $SERVER_IPV4"
fi

PHYS_IFACE=$(ip route get 1.1.1.1 | awk '{print $5; exit}')
echo "Physical interface: $PHYS_IFACE"

install_amneziawg() {
    command -v awg >/dev/null 2>&1 && return 0

    echo "Installing AmneziaWG (tools + DKMS kernel module) from ppa:amnezia/ppa..."
    export DEBIAN_FRONTEND=noninteractive

    apt-get update -qq
    apt-get install -y -qq gnupg lsb-release dkms "linux-headers-$(uname -r)"

    local UBUNTU_CODENAME KEYRING=/usr/share/keyrings/amnezia.gpg LIST=/etc/apt/sources.list.d/amnezia.list
    UBUNTU_CODENAME="$(lsb_release -cs)"

    # Import signing key by FULL FINGERPRINT (never short key IDs over the wire)
    gpg --batch --yes --dearmor \
        --output "$KEYRING" \
        <(curl -fsSL "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x75C9DD72C799870E310542E24166F2C257290828")

    echo "deb [signed-by=${KEYRING}] https://ppa.launchpadcontent.net/amnezia/ppa/ubuntu ${UBUNTU_CODENAME} main" > "$LIST"
    chmod 644 "$LIST"

    apt-get update -qq
    apt-get install -y -qq amneziawg amneziawg-linux-kmod

    # Fail loudly if the kernel module did not load (DKMS build can fail on
    # brand-new kernels until upstream catches up).
    modprobe amneziawg || {
        echo "FATAL: amneziawg kernel module failed to load. Check 'dkms status'." >&2
        exit 1
    }
}

install_amneziawg

mkdir -p /etc/amnezia/awg
chmod 700 /etc/amnezia/awg

if [[ ! -f /etc/amnezia/awg/wg1_private.key ]]; then
    echo "Generating AmneziaWG server keys..."
    umask 077
    awg genkey | tee /etc/amnezia/awg/wg1_private.key | awg pubkey > /etc/amnezia/awg/wg1_public.key
fi

SERVER_PRIVATE_KEY=$(cat /etc/amnezia/awg/wg1_private.key)
SERVER_PUBLIC_KEY=$(cat /etc/amnezia/awg/wg1_public.key)
echo "AmneziaWG Server Public Key: $SERVER_PUBLIC_KEY"

cat > "/etc/amnezia/awg/${AWG_INTERFACE}.conf" <<EOF
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

PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -t nat -A POSTROUTING -s ${AWG_SUBNET_V4} -o ${PHYS_IFACE} -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -t nat -D POSTROUTING -s ${AWG_SUBNET_V4} -o ${PHYS_IFACE} -j MASQUERADE
EOF

chmod 600 "/etc/amnezia/awg/${AWG_INTERFACE}.conf"

systemctl enable "awg-quick@${AWG_INTERFACE}"
systemctl restart "awg-quick@${AWG_INTERFACE}"

sleep 2
echo "=== AmneziaWG Status ==="
systemctl status "awg-quick@${AWG_INTERFACE}" --no-pager || true
awg show "${AWG_INTERFACE}" || true

echo "=== iptables NAT rules ==="
iptables -t nat -L POSTROUTING -v -n | grep MASQUERADE

echo ""
echo "AmneziaWG Server Public Key: $SERVER_PUBLIC_KEY"
echo "Server Endpoint: ${SERVER_IPV4}:${AWG_PORT}"
echo "Obfuscation Parameters:"
echo "  Jc=${JC} Jmin=${JMIN} Jmax=${JMAX} S1=${S1} S2=${S2}"
echo "  H1=${H1} H2=${H2} H3=${H3} H4=${H4}"
echo ""
echo "Client config template:"
cat <<CLIENTEOF
[Interface]
PrivateKey = <CLIENT_PRIVATE_KEY>
Address = ${AWG_SUBNET_V4%.0}.X/${AWG_SUBNET_V4##*/}
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
