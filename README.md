# ⚡ InotiTidy

**InotiTidy** is a Linux daemon written in Go that automatically sorts files from selected folders (for example, `Downloads` and `Desktop`) into destination directories by extension rules from config.

It is designed to run as a `systemd` service and continuously scan watched folders on a short interval.

---

## 🌟 Current behavior (important)

In the current implementation, InotiTidy:

- Uses a **polling watcher** (directory scan every ~1 second), not `inotify/fsnotify`.
- Features an **Interactive Terminal UI (TUI)** (`inotitidy tui`) to safely manage configuration natively.
- Waits until a file size becomes stable before moving it (helps avoid moving incomplete downloads/copies).
- Ignores files containing configured exclude keywords (case-insensitive).
- Routes files by extension rules using case-insensitive suffix matching (supports multi-part extensions like `.tar.gz`).
- Avoids overwrite conflicts by appending a Unix timestamp to the filename.

---

## 🏗️ Project structure

- `cmd/inotitidy/main.go` — app entrypoint, startup/shutdown lifecycle.
- `internal/config` — config loading, `~` expansion, and built-in parser for supported config format.
- `internal/watcher` — polling loop, stability check, filtering, and file move logic.
- `install.sh` — install helper: build binary, place config, generate/restart `systemd` service.
- `config.yaml` — default config template copied on first install.

---

## 🚀 Installation

## 1) Prerequisites

- Linux with `systemd`
- Go `1.21+`
- `sudo` access for service installation into `/etc/systemd/system`

## 2) Install command

```bash
sudo make install
```

What this does:

1. Builds `inotitidy`.
2. Detects target user (`SUDO_USER` fallback to `USER`) and uses that user home.
3. Copies binary to `~/.local/bin/inotitidy` (for target user).
4. Creates `~/.config/inotitidy/config.yaml` if missing.
5. Writes system service file `/etc/systemd/system/inotitidy.service` with `User=<target-user>`.
6. Runs `systemctl daemon-reload`, `enable`, and `restart` for `inotitidy.service`.

---

## ⚙️ Configuration

InotiTidy provides a built-in interactive Terminal User Interface (TUI) to easily manage your sorting rules, watch directories, and excluded keywords.

To open the TUI configuration menu, simply run:

```bash
inotitidy tui
# or
inotitidy config
```

*Note: If `inotitidy` is not in your `PATH` or you installed via `make install`, use `~/.local/bin/inotitidy tui`.*

Using the TUI, you can:
- Add and remove directories to monitor.
- Add ignore keywords to prevent moving specific files.
- Create mapping rules (e.g. `[".png", ".jpg"] -> "~/Pictures"`).

The configuration is saved natively to `~/.config/inotitidy/config.yaml`. 
If you modify the configuration while the daemon is running, restart the systemd service to apply changes.

### Manual Configuration (Optional)

Config path:

```text
~/.config/inotitidy/config.yaml
```

Example (supported by current parser):

```yaml
watch_directories:
  - "~/Downloads"
  - "~/Desktop"

exclude_keywords:
  - "KEEP"
  - "IMPORTANT"

rules:
  - extensions: [".pdf", ".doc", ".docx", ".txt"]
    target: "~/Documents"

  - extensions: [".jpg", ".jpeg", ".png", ".gif"]
    target: "~/Pictures"

  - extensions: [".mp3", ".wav", ".flac"]
    target: "~/Music"

  - extensions: [".zip", ".tar", ".gz", ".tar.gz", ".rar"]
    target: "~/Downloads/Archives"
```

### Notes

- `watch_directories` is required.
- `~` in paths is expanded to current user home at runtime.
- Rule extensions are matched by filename suffix, case-insensitive.

---

## 🧰 Service management

Because installer creates a **system** service, use:

```bash
sudo systemctl status inotitidy.service
sudo systemctl restart inotitidy.service
sudo systemctl stop inotitidy.service
sudo journalctl -u inotitidy.service -f
```

---

## 🔍 Troubleshooting

### Service logs show `/root/Downloads` or `/root/Desktop`

Usually this means the service runs as `root` or config points to root home.

Checklist:

1. Reinstall using `sudo make install` from your normal user.
2. Verify service user:
   ```bash
   systemctl cat inotitidy.service | grep '^User='
   ```
3. Verify config path/content for that same user:
   ```bash
   cat ~/.config/inotitidy/config.yaml
   ```
4. Restart service after config changes:
   ```bash
   sudo systemctl restart inotitidy.service
   ```

### "Error reading <dir>: no such file or directory"

Create missing watch directories or update `watch_directories` in config.

---

## 🧪 Local development

```bash
make build
go test ./...
```

---

## 🤝 Contributing

1. Fork repository
2. Create feature branch
3. Commit changes
4. Open PR
