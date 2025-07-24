#!/bin/bash
# Install systemd services for sirseer-relay
# Run as root or with sudo

set -euo pipefail

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root or with sudo" >&2
   exit 1
fi

echo "=== Installing SirSeer Relay systemd services ==="

# Create user and group
if ! id -u sirseer &>/dev/null; then
    echo "Creating sirseer user..."
    useradd -r -s /bin/false -d /var/lib/sirseer-relay -m sirseer
fi

# Create directories
echo "Creating directories..."
mkdir -p /etc/sirseer-relay
mkdir -p /var/lib/sirseer-relay/state
mkdir -p /var/log/sirseer-relay
mkdir -p /var/data/sirseer-relay/weekly

# Set permissions
chown -R sirseer:sirseer /var/lib/sirseer-relay
chown -R sirseer:sirseer /var/log/sirseer-relay
chown -R sirseer:sirseer /var/data/sirseer-relay

# Copy service files
echo "Installing systemd service files..."
cp sirseer-relay.service /etc/systemd/system/
cp sirseer-relay.timer /etc/systemd/system/
cp sirseer-relay-weekly.service /etc/systemd/system/
cp sirseer-relay-weekly.timer /etc/systemd/system/

# Copy scripts
echo "Installing scripts..."
cp ../cron-daily.sh /usr/local/bin/sirseer-daily.sh
cp ../cron-weekly.sh /usr/local/bin/sirseer-weekly.sh
chmod +x /usr/local/bin/sirseer-daily.sh
chmod +x /usr/local/bin/sirseer-weekly.sh

# Create example configuration files
echo "Creating example configuration..."

# Environment file
cat > /etc/sirseer-relay/environment.example << 'EOF'
# SirSeer Relay Environment Configuration
# Copy to /etc/sirseer-relay/environment and edit

# GitHub token (required) - alternatively use GITHUB_TOKEN_FILE
#GITHUB_TOKEN=ghp_your_token_here

# Custom API endpoint for GitHub Enterprise
#GITHUB_API_URL=https://github.company.com/api/v3

# Slack webhook for notifications (optional)
#SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
EOF

# Repositories file
cat > /etc/sirseer-relay/repositories.txt.example << 'EOF'
# List of repositories to fetch
# One repository per line in format: owner/repo
# Lines starting with # are ignored

# Examples:
kubernetes/kubernetes
prometheus/prometheus
grafana/grafana

# Your repositories here:
EOF

# Config file
cat > /etc/sirseer-relay/config.yaml.example << 'EOF'
# SirSeer Relay Configuration

# Default request timeout in seconds
request_timeout: 300

# Output directory (can be overridden per fetch)
output: /var/data/sirseer-relay

# State directory
state_dir: /var/lib/sirseer-relay/state

# Log level (debug, info, warn, error)
log_level: info
EOF

# GitHub token file (more secure than environment variable)
cat > /etc/sirseer-relay/github-token.example << 'EOF'
ghp_your_github_token_here
EOF
chmod 600 /etc/sirseer-relay/github-token.example

# Reload systemd
echo "Reloading systemd..."
systemctl daemon-reload

echo
echo "=== Installation complete ==="
echo
echo "Next steps:"
echo "1. Configure your GitHub token:"
echo "   - Edit /etc/sirseer-relay/github-token"
echo "   - Or set GITHUB_TOKEN in /etc/sirseer-relay/environment"
echo
echo "2. Configure repositories to fetch:"
echo "   cp /etc/sirseer-relay/repositories.txt.example /etc/sirseer-relay/repositories.txt"
echo "   vim /etc/sirseer-relay/repositories.txt"
echo
echo "3. Review and adjust configuration:"
echo "   cp /etc/sirseer-relay/config.yaml.example /etc/sirseer-relay/config.yaml"
echo "   vim /etc/sirseer-relay/config.yaml"
echo
echo "4. Enable and start timers:"
echo "   systemctl enable --now sirseer-relay.timer"
echo "   systemctl enable --now sirseer-relay-weekly.timer"
echo
echo "5. Check status:"
echo "   systemctl status sirseer-relay.timer"
echo "   systemctl list-timers"
echo "   journalctl -u sirseer-relay"
echo
echo "To run manually:"
echo "   systemctl start sirseer-relay.service"