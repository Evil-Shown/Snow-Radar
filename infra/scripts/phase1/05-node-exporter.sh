#!/bin/bash
# Prometheus Node Exporter Installation Script
# Run as root on both VPN servers.
# SECURITY (audit #4): binds ONLY to the node's WireGuard IP so metrics are
# unreachable from the public internet. No firewall hole for 9100 is opened.
# Usage: sudo ./05-node-exporter.sh [wg_server_ipv4]
#   If omitted, tries to detect the wg0 interface address.

set -euo pipefail

log() { echo -e "\033[0;32m[+]\033[0m $*"; }
error() { echo -e "\033[0;31m[-]\033[0m $*"; exit 1; }

NODE_EXPORTER_VERSION="1.8.1"
ARCH="amd64"

if [[ $(uname -m) == "aarch64" ]]; then
    ARCH="arm64"
fi

# Resolve listen address: arg > wg0 address > fail loudly (never fall back to public bind)
LISTEN_IP="${1:-}"
if [[ -z "$LISTEN_IP" ]]; then
    LISTEN_IP=$(ip -4 -o addr show dev wg0 2>/dev/null | awk '{split($4,a,"/"); print a[1]; exit}' || true)
fi
if [[ -z "$LISTEN_IP" ]]; then
    error "Could not determine WireGuard IPv4 for metrics binding. Run WireGuard setup first or pass the wg0 IP as argument."
fi
log "Node Exporter will listen on ${LISTEN_IP}:9100 (WireGuard-only, not public)"

if ! id -u node_exporter >/dev/null 2>&1; then
    useradd --no-create-home --shell /bin/false node_exporter
    log "Created node_exporter user"
fi

cd /tmp
curl -fsSL "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
    | tar xz

mv "node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}/node_exporter" /usr/local/bin/
chown node_exporter:node_exporter /usr/local/bin/node_exporter
chmod 755 /usr/local/bin/node_exporter

rm -rf "node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}"

cat > /etc/systemd/system/node_exporter.service <<EOF
[Unit]
Description=Prometheus Node Exporter (WireGuard-only listener)
After=network-online.target wg-quick@wg0.service
Wants=network-online.target

[Service]
User=node_exporter
Group=node_exporter
Type=simple
ExecStart=/usr/local/bin/node_exporter \\
    --collector.systemd \\
    --collector.processes \\
    --collector.tcpstat \\
    --collector.udpstat \\
    --collector.netdev \\
    --collector.filesystem \\
    --collector.diskstats \\
    --collector.netclass \\
    --no-collector.wifi \\
    --no-collector.hwmon \\
    --web.listen-address=${LISTEN_IP}:9100
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadOnlyPaths=/
ReadWritePaths=/proc /sys /run

[Install]
WantedBy=multi-user.target
EOF

# NOTE: intentionally NO 'ufw allow 9100' — metrics are reachable only from
# inside the VPN mesh. Scraping config must use the WG IPs (see prometheus.yml).

systemctl daemon-reload
systemctl enable --now node_exporter

log "Node Exporter installed and started on ${LISTEN_IP}:9100"
sleep 2
systemctl status node_exporter --no-pager || true
curl -sf "http://${LISTEN_IP}:9100/metrics" | head -20 || error "Metrics endpoint did not respond"
