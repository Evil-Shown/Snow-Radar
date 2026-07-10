#!/bin/bash
# Prometheus Node Exporter Installation Script
# Run as root on both VPN servers

set -euo pipefail

log() { echo -e "\033[0;32m[+]\033[0m $*"; }
error() { echo -e "\033[0;31m[-]\033[0m $*"; exit 1; }

NODE_EXPORTER_VERSION="1.8.1"
ARCH="amd64"

# Detect architecture
if [[ $(uname -m) == "aarch64" ]]; then
    ARCH="arm64"
fi

log "Installing Prometheus Node Exporter ${NODE_EXPORTER_VERSION} (${ARCH})"

# Create user
if ! id -u node_exporter >/dev/null 2>&1; then
    useradd --no-create-home --shell /bin/false node_exporter
    log "Created node_exporter user"
fi

# Download and install
cd /tmp
curl -sSL "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
    | tar xz

mv "node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}/node_exporter" /usr/local/bin/
chown node_exporter:node_exporter /usr/local/bin/node_exporter
chmod 755 /usr/local/bin/node_exporter

# Cleanup
rm -rf "node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}"

# Create systemd service
cat > /etc/systemd/system/node_exporter.service <<'EOF'
[Unit]
Description=Prometheus Node Exporter
After=network.target

[Service]
User=node_exporter
Group=node_exporter
Type=simple
ExecStart=/usr/local/bin/node_exporter \
    --collector.systemd \
    --collector.processes \
    --collector.tcpstat \
    --collector.udpstat \
    --collector.netdev \
    --collector.filesystem \
    --collector.diskstats \
    --collector.netclass \
    --no-collector.wifi \
    --no-collector.hwmon \
    --web.listen-address=:9100
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

# Allow Node Exporter through UFW (from monitoring server only - adjust CIDR)
# For now allow from anywhere, restrict later via cloud firewall
ufw allow 9100/tcp comment 'Prometheus Node Exporter'

# Start and enable
systemctl daemon-reload
systemctl enable --now node_exporter

log "Node Exporter installed and started"
log "Metrics available at: http://$(hostname -I | awk '{print $1}'):9100/metrics"

# Verify
sleep 2
systemctl status node_exporter --no-pager
curl -s http://localhost:9100/metrics | head -20