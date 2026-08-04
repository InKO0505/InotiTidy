#!/usr/bin/env bash
# Build InotiTidy and install it as a per-user systemd service.
# No root required: the daemon runs under `systemctl --user`.
set -euo pipefail

BIN_NAME="inotitidy"
BIN_DIR="$HOME/.local/bin"
CFG_DIR="$HOME/.config/inotitidy"

echo "==> Building $BIN_NAME"
CGO_ENABLED=0 go build -ldflags "-s -w" -o "$BIN_NAME" ./cmd/inotitidy

echo "==> Installing to $BIN_DIR"
mkdir -p "$BIN_DIR" "$CFG_DIR"
install -m755 "$BIN_NAME" "$BIN_DIR/$BIN_NAME"

if [ ! -f "$CFG_DIR/config.yaml" ]; then
  echo "==> Writing default config to $CFG_DIR/config.yaml"
  cp config.yaml "$CFG_DIR/config.yaml"
fi

echo "==> Installing and enabling the systemd --user service"
"$BIN_DIR/$BIN_NAME" --install

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "NOTE: add $BIN_DIR to your PATH to run '$BIN_NAME' directly." ;;
esac

echo "Done. Launch the console with: $BIN_NAME"
echo "Follow the daemon logs with: journalctl --user -u inotitidy.service -f"
