#!/bin/bash

set -e 

if [[ -f "./uninstall.sh" ]]; then
    source ./uninstall.sh
fi

BASE_DIR="$PWD/teletrack-prod"
BIN="$BASE_DIR/teletrack"
CFG="$BASE_DIR/config.json"
UNIT="$BASE_DIR/teletrack.service"

INSTALL_DIR="/opt/teletrack"
BIN_DIR="$INSTALL_DIR/bin"
CFG_DIR="$INSTALL_DIR/bin"

echo "Создание каталогов в $INSTALL_DIR..."
sudo install -d -m 755 -o root -g root "$BIN_DIR" "$CFG_DIR"

echo "Копирование бинарного файла $BIN..."
sudo install -m 755 "$BIN" "$BIN_DIR/"

echo "Копирование конфигурационного файла $CFG..."
sudo install -m 644 "$CFG" "$CFG_DIR/"

echo "Копирование systemd юнита $UNIT..."
sudo install -m 644 "$UNIT" "/etc/systemd/system/"

echo "Перезагрузка systemd и запуск службы..."
sudo systemctl daemon-reload
sudo systemctl enable teletrack.service
sudo systemctl restart teletrack.service

echo "✅ Установка завершена."
