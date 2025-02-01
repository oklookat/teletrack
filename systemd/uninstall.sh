#!/bin/bash

INSTALL_DIR="/opt/teletrack"
UNIT="teletrack.service"
UNIT_PATHS=("/etc/systemd/system/")

# Удаление установленного каталога
sudo rm -rf "$INSTALL_DIR"

# Проверка наличия юнита и удаление, если существует
UNIT_EXISTS=false
for UNIT_PATH in "${UNIT_PATHS[@]}"; do
  if [ -f "$UNIT_PATH$UNIT" ]; then
    UNIT_EXISTS=true
    UNIT_PATH=$UNIT_PATH
    break
  fi
done

if [ "$UNIT_EXISTS" = true ]; then
  echo "'$UNIT' найден в $UNIT_PATH. Остановка и удаление..."
  sudo systemctl stop "$UNIT"
  sudo systemctl disable "$UNIT"
  sudo rm "$UNIT_PATH$UNIT"
  sudo systemctl daemon-reload
  echo "'$UNIT' успешно удалён."
else
  echo "'$UNIT' не существует."
fi
