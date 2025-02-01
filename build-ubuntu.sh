#!/bin/sh

USER_HOME="/home/$USER"
TELETRACK_DIR="$USER_HOME/teletrack"

rm -rf $TELETRACK_DIR
mkdir -p $TELETRACK_DIR

go build
mv teletrack $TELETRACK_DIR
cp config.prod.json $TELETRACK_DIR/config.json
cp ./systemd/* $TELETRACK_DIR

tar czf $USER_HOME/teletrack.tar.gz $TELETRACK_DIR
rm -rf $TELETRACK_DIR
