#!/bin/bash
set -euo pipefail

# Run uninstall if present
if [[ -f "./uninstall.sh" ]]; then
    # shellcheck source=/dev/null
    source ./uninstall.sh
fi

BASE_DIR="$PWD/teletrack-prod"
BIN="$BASE_DIR/teletrack"
CFG="$BASE_DIR/config.json"
UNIT="$BASE_DIR/teletrack.service"

INSTALL_DIR="/opt/teletrack"
BIN_DIR="$INSTALL_DIR/bin"
CFG_DIR="$INSTALL_DIR/bin"  # keeping config with binary for simplicity

echo "📁 Creating directories in $INSTALL_DIR..."
sudo install -d -m 755 -o root -g root "$BIN_DIR" "$CFG_DIR"

echo "📦 Copying binary file: $BIN"
sudo install -m 755 "$BIN" "$BIN_DIR/"

echo "📝 Copying configuration file: $CFG"
sudo install -m 644 "$CFG" "$CFG_DIR/"

echo "⚙️ Copying systemd unit: $UNIT"
sudo install -m 644 "$UNIT" "/etc/systemd/system/"

echo "🔄 Reloading systemd and starting service..."
sudo systemctl daemon-reload
sudo systemctl enable teletrack.service
sudo systemctl restart teletrack.service

echo "✅ Installation complete."
