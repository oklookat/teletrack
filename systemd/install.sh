#!/bin/bash

source uninstall.sh

BIN="teletrack"
CFG="config.json"
UNIT="teletrack.service"
INSTALL_DIR="/opt/teletrack"

# Пути установки
BIN_DIR="$INSTALL_DIR/bin"
ETC_DIR="$INSTALL_DIR/bin"

# Создание директорий
echo "Создание каталогов в $INSTALL_DIR..."
sudo install -d -m 755 -o root -g root "$BIN_DIR" "$ETC_DIR"

# Установка бинарного файла
echo "Копирование бинарного файла $BIN..."
sudo install -m 755 "$BIN" "$BIN_DIR/"

# Установка конфигурационного файла
echo "Копирование конфигурационного файла $CFG..."
sudo install -m 644 "$CFG" "$ETC_DIR/"

# Установка systemd юнита
echo "Копирование systemd юнита $UNIT..."
sudo install -m 644 "$UNIT" "/etc/systemd/system/"

# Обновление systemd и запуск сервиса
echo "Перезагрузка systemd и запуск службы..."
sudo systemctl daemon-reload
sudo systemctl enable "$UNIT"
sudo systemctl start "$UNIT"

echo "Установка завершена."