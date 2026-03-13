# ⚡ InotiTidy

**InotiTidy** is a Linux daemon written in Go that automatically sorts files from selected folders (for example, `Downloads` and `Desktop`) into destination directories based on extension rules.

It is designed to run as a `systemd` service and continuously scan watched folders at a short interval.

---

## 🌟 Features

- **Polling Watcher**: Scans directories every ~1 second (no `inotify` overhead).
- **Premium TUI**: A beautiful "Tokyo Night" themed terminal interface for configuration.
- **2-Pane Layout**: Modern Sidebar + Content Area design for intuitive navigation.
- **Stability Check**: Only moves files after they stop growing (safe for large downloads).
- **Exclude Keywords**: Skip specific files using case-insensitive ignore lists.
- **Extension Routing**: Map multi-part extensions (like `.tar.gz`) to target folders.
- **Collision Safety**: Appends timestamps to filenames instead of overwriting existing files.

---

## 🏗️ Project Structure

- `cmd/inotitidy/main.go` — App entrypoint and lifecycle.
- `cmd/inotitidy/tui.go` — Terminal UI implementation.
- `internal/config` — Config management and parser.
- `internal/watcher` — Core sorting logic and polling loop.
- `install.sh` — Automated installation and systemd setup script.
- `config.yaml` — Default template for the configuration file.

---

## 🚀 Installation & Build

### 1) Prerequisites
- Linux with `systemd`
- Go `1.21+`
- `sudo` access (for service installation)

### 2) Quick Install
```bash
sudo make install
```
This command builds the binary, moves it to `~/.local/bin/`, sets up the config directory, and starts the `systemd` service.

### 3) Manual Build
To just compile the binary locally:
```bash
make build
# or manual
go build -o inotitidy ./cmd/inotitidy
```

---

## ⚙️ Configuration (TUI)

InotiTidy features a professional Terminal User Interface to manage your settings without editing YAML manually.

### How to Launch the TUI

Depending on how you want to run it:

| Command | Description |
| :--- | :--- |
| `inotitidy tui` | Launch TUI from an installed binary (in your PATH). |
| `go run ./cmd/inotitidy tui` | Launch TUI directly from source (development mode). |
| `inotitidy config` | Alias for launching the TUI. |

> [!IMPORTANT]
> Running the command **without arguments** (i.e., just `inotitidy`) starts the **daemon** process, which runs in the background.

### TUI Navigation
- **Arrows (↑/↓)**: Navigate through lists in the active pane.
- **Tab**: Switch focus between the **Sidebar** and the **Main Pane**.
- **Enter**: Select an item or "focus" into a list to begin editing/removing.
- **Esc**: Exit the current form or return focus to the Sidebar.
- **Ctrl+C**: Abort and exit the application entirely.

---

## 🧰 Service Management

Since the installer creates a system service, you can manage the background daemon using standard tools:

```bash
# Check if it's running
sudo systemctl status inotitidy.service

# Restart after making changes in TUI (required for daemon to reload config)
sudo systemctl restart inotitidy.service

# View live logs
sudo journalctl -u inotitidy.service -f
```

---

## 🔍 Troubleshooting

### TUI doesn't open
- Ensure you passed the `tui` argument.
- If you just ran `go run ./cmd/inotitidy`, the daemon started instead. Kill it with `Ctrl+C` and use `go run ./cmd/inotitidy tui`.

### Changes don't take effect
- The TUI saves the config to `~/.config/inotitidy/config.yaml`.
- The background daemon reads this file only on startup. **Always restart the service** after saving changes in the TUI:
  `sudo systemctl restart inotitidy.service`

### "Error reading <dir>"
- Check that the paths added in the TUI actually exist. Use absolute paths where possible.

---

## 🤝 Contributing
1. Fork it.
2. Create your feature branch.
3. Open a Pull Request!
