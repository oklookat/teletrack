#!/bin/bash
set -euo pipefail

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "Script dir: $SCRIPT_DIR"

# Load environment variables
if [[ -f "$SCRIPT_DIR/.env" ]]; then
    # shellcheck source=/dev/null
    source "$SCRIPT_DIR/.env"
else
    echo ".env file not found in $SCRIPT_DIR"
    exit 1
fi

TAR_GZ="teletrack.tar.gz"
TELETRACK_DIR="teletrack-prod"

# Clean old build
rm -rf "$TELETRACK_DIR" "$TAR_GZ"

# Build structure
mkdir -p "$TELETRACK_DIR"
go build -o "$TELETRACK_DIR/teletrack"
cp config.prod.json "$TELETRACK_DIR/config.json"
cp -r "$SCRIPT_DIR/systemd/"* "$TELETRACK_DIR/"

# Create archive
tar czf "$TAR_GZ" "$TELETRACK_DIR"

# Clean temporary folder
rm -rf "$TELETRACK_DIR"

# SSH helper functions
remote_cmd() {
    local cmd="$*"
    ssh -t -i "$SSH_KEY" -p "$SSH_PORT" "$SSH_USER@$SSH_HOST" "$cmd"
}

remote_copy() {
    local local_file="$1"
    local remote_path="$2"
    scp -i "$SSH_KEY" -P "$SSH_PORT" "$local_file" "$SSH_USER@$SSH_HOST:$remote_path"
}

# Deploy steps
echo "Uninstalling old version..."
remote_cmd "~/$TELETRACK_DIR/uninstall.sh || true"  # ignore errors if uninstall script missing
remote_cmd "rm -rf ~/$TELETRACK_DIR"

echo "Copying archive..."
remote_copy "$TAR_GZ" "~"

echo "Extracting and installing..."
remote_cmd "tar xzf $TAR_GZ"
remote_cmd "chmod +x $TELETRACK_DIR/*.sh && $TELETRACK_DIR/install.sh && rm $TAR_GZ"

# Cleanup local archive
rm -f "$TAR_GZ"

echo "Deployment finished successfully!"
