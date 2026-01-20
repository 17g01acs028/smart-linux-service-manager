#!/bin/bash
set -e

# Usage: sudo ./install.sh [path_to_binary]
# If no path provided, assumes ./lsm-linux exists in current dir.

BINARY=${1:-"./lsm-linux"}
DEST="/usr/local/bin/lsm"
SERVICE_FILE="/etc/systemd/system/lsm.service"
LOG_DIR="/var/log/lsm"
LOG_FILE="$LOG_DIR/lsm.log"

# 0. Check Root
if [ "$EUID" -ne 0 ]; then
  echo "Error: Please run as root (sudo ./install.sh)"
  exit 1
fi

if [ ! -f "$BINARY" ]; then
    echo "Error: Binary '$BINARY' not found."
    exit 1
fi

echo "Installing LSM..."

# 0.5 Setup Logs (Readable by all)
echo "-> Configuring log directory..."
mkdir -p "$LOG_DIR"
touch "$LOG_FILE"
chmod 755 "$LOG_DIR"
chmod 644 "$LOG_FILE"
# Ensure future rotated logs are also readable?
# Lumberjack uses default umask. Root usually has 0022 -> 644. Should be fine.

# 1. Install Binary
echo "-> Stopping existing service (if running)..."
systemctl stop lsm || true # Ignore error if not running

echo "-> Moving binary to $DEST..."
cp "$BINARY" "$DEST"
chmod +x "$DEST"

# 2. Create Service File
echo "-> Creating systemd service ($SERVICE_FILE)..."
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Linux Service Manager Daemon
After=network.target

[Service]
User=root
ExecStart=$DEST daemon
WorkingDirectory=/root
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 3. Enable and Start
echo "-> Enabling and starting lsm.service..."
systemctl daemon-reload
systemctl enable lsm
systemctl restart lsm

echo "SUCCESS! LSM installed and running."
echo "Check status with: systemctl status lsm"
echo "View logs with:    journalctl -u lsm -f"
