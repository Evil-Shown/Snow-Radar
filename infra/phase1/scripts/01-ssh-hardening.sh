#!/bin/bash
# SSH Hardening Script for Snow Radar VPN Servers
# Run as root on both Oracle (ubuntu user) and Hetzner (root user)

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[-]${NC} $*"; exit 1; }

# Detect cloud provider
detect_provider() {
    if curl -s -H "Authorization: Bearer Oracle" -L http://169.254.169.254/opc/v2/instance/ >/dev/null 2>&1; then
        echo "oracle"
    elif curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/ >/dev/null 2>&1; then
        echo "gcp"
    elif curl -s http://169.254.169.254/hetzner/v1/ >/dev/null 2>&1; then
        echo "hetzner"
    else
        echo "unknown"
    fi
}

PROVIDER=$(detect_provider)
log "Detected provider: $PROVIDER"

# Configuration
ADMIN_USER="snowradar"
SSH_PORT=22
SSH_KEY_FILE="/tmp/authorized_keys_new"

# Create admin user (idempotent)
create_admin_user() {
    if id "$ADMIN_USER" &>/dev/null; then
        log "User $ADMIN_USER already exists"
    else
        log "Creating admin user: $ADMIN_USER"
        useradd -m -s /bin/bash -G sudo "$ADMIN_USER"
        echo "$ADMIN_USER ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/99-$ADMIN_USER
        chmod 440 /etc/sudoers.d/99-$ADMIN_USER
    fi
}

# Setup SSH keys for admin user
setup_ssh_keys() {
    log "Setting up SSH keys for $ADMIN_USER"
    mkdir -p /home/$ADMIN_USER/.ssh
    chmod 700 /home/$ADMIN_USER/.ssh

    # Copy current authorized_keys (from root or ubuntu)
    if [[ -f /root/.ssh/authorized_keys ]]; then
        cp /root/.ssh/authorized_keys /home/$ADMIN_USER/.ssh/authorized_keys
    elif [[ -f /home/ubuntu/.ssh/authorized_keys ]]; then
        cp /home/ubuntu/.ssh/authorized_keys /home/$ADMIN_USER/.ssh/authorized_keys
    fi

    chmod 600 /home/$ADMIN_USER/.ssh/authorized_keys
    chown -R $ADMIN_USER:$ADMIN_USER /home/$ADMIN_USER/.ssh
}

# Harden SSH daemon
harden_sshd() {
    log "Hardening SSH daemon"
    cat > /etc/ssh/sshd_config.d/99-snowradar-hardening.conf <<'EOF'
# Snow Radar SSH Hardening
Port 22
Protocol 2

# Authentication
PermitRootLogin no
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
UsePAM yes

# Key exchange & ciphers (modern only)
KexAlgorithms curve25519-sha256,curve25519-sha256@libssh.org,diffie-hellman-group16-sha512,diffie-hellman-group18-sha512
Ciphers chacha20-poly1305@openssh.com,aes256-gcm@openssh.com,aes128-gcm@openssh.com
MACs hmac-sha2-256-etm@openssh.com,hmac-sha2-512-etm@openssh.com,umac-128-etm@openssh.com

# Security
MaxAuthTries 3
MaxSessions 2
LoginGraceTime 30
ClientAliveInterval 300
ClientAliveCountMax 2
AllowUsers snowradar

# Logging
SyslogFacility AUTH
LogLevel VERBOSE

# Disable forwarding
AllowAgentForwarding no
AllowTcpForwarding no
X11Forwarding no
PermitTunnel no

# Banner
Banner /etc/ssh/banner
EOF

    # Create banner
    cat > /etc/ssh/banner <<'EOF'
**************************************************************
*                  UNAUTHORIZED ACCESS PROHIBITED            *
*                    Snow Radar VPN Server                   *
*         All connections are monitored and logged           *
**************************************************************
EOF

    # Test config
    sshd -t || error "SSH config test failed"
    systemctl reload sshd
    log "SSH daemon reloaded"
}

# Configure UFW (will be expanded in wireguard script)
setup_ufw_base() {
    log "Configuring base UFW rules"
    ufw --force reset
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow 22/tcp comment 'SSH'
    ufw --force enable
    log "UFW enabled with SSH allowed"
}

# Install fail2ban
install_fail2ban() {
    log "Installing fail2ban"
    apt-get update -qq
    apt-get install -y -qq fail2ban
    cat > /etc/fail2ban/jail.d/snowradar.conf <<'EOF'
[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 3600
findtime = 600
backend = systemd
EOF
    systemctl enable --now fail2ban
    log "fail2ban configured and started"
}

# Main
main() {
    log "Starting SSH hardening for $PROVIDER"

    create_admin_user
    setup_ssh_keys
    harden_sshd
    setup_ufw_base
    install_fail2ban

    log "SSH hardening complete!"
    warn "Test SSH access in NEW terminal: ssh -i <key> $ADMIN_USER@<server-ip>"
    warn "Do NOT close this session until verified!"
}

main "$@"