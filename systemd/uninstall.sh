#!/bin/bash

set -e

SERVICE_NAME="teletrack.service"
INSTALL_DIR="/opt/teletrack"
BIN_DIR="$INSTALL_DIR/bin"
CFG_DIR="$INSTALL_DIR/bin"

echo "Остановка службы $SERVICE_NAME..."
sudo systemctl stop "$SERVICE_NAME" || true

echo "Отключение автозапуска службы $SERVICE_NAME..."
sudo systemctl disable "$SERVICE_NAME" || true

echo "Удаление systemd юнита..."
sudo rm -f "/etc/systemd/system/$SERVICE_NAME"

echo "Перезагрузка systemd..."
sudo systemctl daemon-reload

echo "Удаление бинарников и конфигураций..."
sudo rm -rf "$INSTALL_DIR"

echo "✅ Удаление завершено."
