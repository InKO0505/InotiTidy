#!/bin/bash
set -e

BIN_NAME="inotitidy"
BIN_DIR="$HOME/.local/bin"
CFG_DIR="$HOME/.config/inotitidy"
SERVICE_FILE="/etc/systemd/system/inotitidy.service"

go build -o $BIN_NAME cmd/inotitidy/main.go

mkdir -p $BIN_DIR
mkdir -p $CFG_DIR

cp $BIN_NAME $BIN_DIR/
[ ! -f "$CFG_DIR/config.yaml" ] && cp config.yaml $CFG_DIR/

sudo tee $SERVICE_FILE > /dev/null <<EOF
[Unit]
Description=InotiTidy File Organizer
After=local-fs.target

[Service]
User=$USER
WorkingDirectory=$CFG_DIR
ExecStart=$BIN_DIR/$BIN_NAME
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable inotitidy.service
sudo systemctl restart inotitidy.service