#!/bin/sh

source ./.env

TAR_GZ="teletrack.tar.gz"
TAR_GZ_LOCAL_PATH="$TAR_GZ"
TELETRACK_DIR="teletrack-prod"

# Удаляем старую папку и архив
rm -rf "$TELETRACK_DIR" "$TAR_GZ"

# Собираем структуру
mkdir -p "$TELETRACK_DIR"
go build
mv teletrack "$TELETRACK_DIR"
cp config.prod.json "$TELETRACK_DIR/config.json"
cp ./systemd/* "$TELETRACK_DIR"

#
tar czf "$TAR_GZ" "$TELETRACK_DIR"

# Чистим временную папку
rm -rf "$TELETRACK_DIR"

# Copy to server
remote_cmd() {
    local remote_cmd="$*"
    ssh -t -i "$SSH_KEY" -p "$SSH_PORT" "$SSH_USER@$SSH_HOST" "$remote_cmd"
}
remote_copy() {
    local local_file="$1"
    local remote_path="$2"
    scp -i "$SSH_KEY" -P "$SSH_PORT" "$local_file" "$SSH_USER@$SSH_HOST:$remote_path"
}

#
remote_cmd "~/$TELETRACK_DIR/uninstall.sh"
remote_cmd "rm -rf ~/$TELETRACK_DIR"
remote_copy $TAR_GZ_LOCAL_PATH "~"
remote_cmd "tar xzf $TAR_GZ"
remote_cmd "chmod +x $TELETRACK_DIR/*.sh && $TELETRACK_DIR/install.sh && rm $TAR_GZ"

#
rm $TAR_GZ_LOCAL_PATH
