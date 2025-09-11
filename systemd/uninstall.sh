#!/bin/bash
set -euo pipefail

SERVICE_NAME="teletrack.service"
INSTALL_DIR="/opt/teletrack"
BIN_DIR="$INSTALL_DIR/bin"
CFG_DIR="$INSTALL_DIR/bin"  # keeping config with binary

echo "🛑 Stopping service $SERVICE_NAME..."
sudo systemctl stop "$SERVICE_NAME" || true

echo "⛔ Disabling service autostart..."
sudo systemctl disable "$SERVICE_NAME" || true

echo "🗑️ Removing systemd unit..."
sudo rm -f "/etc/systemd/system/$SERVICE_NAME"

echo "🔄 Reloading systemd daemon..."
sudo systemctl daemon-reload

echo "🗂️ Removing binaries and configurations..."
sudo rm -rf "$INSTALL_DIR"

echo "✅ Uninstallation complete."
