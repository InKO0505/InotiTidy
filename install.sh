#!/bin/bash
set -e

BIN_NAME="inotitidy"
TARGET_USER="${SUDO_USER:-$USER}"
TARGET_HOME="$(getent passwd "$TARGET_USER" | cut -d: -f6)"

if [ -z "$TARGET_HOME" ]; then
  TARGET_HOME="$HOME"
fi

BIN_DIR="$TARGET_HOME/.local/bin"
CFG_DIR="$TARGET_HOME/.config/inotitidy"
SERVICE_FILE="/etc/systemd/system/inotitidy.service"

go build -o "$BIN_NAME" ./cmd/inotitidy

mkdir -p "$BIN_DIR"
mkdir -p "$CFG_DIR"

cp "$BIN_NAME" "$BIN_DIR/"
[ ! -f "$CFG_DIR/config.yaml" ] && cp config.yaml "$CFG_DIR/"

if [ "$(id -u)" -eq 0 ]; then
  chown -R "$TARGET_USER":"$TARGET_USER" "$BIN_DIR" "$CFG_DIR"
fi

sudo tee "$SERVICE_FILE" > /dev/null <<EOF_SERVICE
[Unit]
Description=InotiTidy File Organizer
After=local-fs.target

[Service]
User=$TARGET_USER
WorkingDirectory=$CFG_DIR
ExecStart=$BIN_DIR/$BIN_NAME --daemon
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF_SERVICE

sudo systemctl daemon-reload
sudo systemctl enable inotitidy.service
sudo systemctl restart inotitidy.service
