# ⚡ InotiTidy

**InotiTidy** is a professional, high-performance Linux system daemon written in Go. It monitors your directories (like `Downloads` or `Desktop`) using the kernel's **inotify** subsystem and automatically sorts incoming files into their designated folders based on real-time events.

No more manual cleaning. No more cluttered "Downloads" folders. Just pure, automated order.

---

## 🌟 Why InotiTidy?

Most file organizers use "Cron jobs" that scan folders every hour, killing your disk I/O. **InotiTidy is different.**

* **Event-Driven (Zero Idle Load):** It sits silently in the background and only wakes up when the Linux kernel notifies it of a new file.
* **Race Condition Protection:** It polls file sizes before moving. If a file is still being downloaded or copied, InotiTidy waits until the transfer is 100% complete.
* **Blazing Fast:** Built with Go's concurrency model. It handles hundreds of file events simultaneously without breaking a sweat.
* **Native Systemd Integration:** Designed to run as a robust background service that starts automatically on boot.

---

## 🏗️ Technical Architecture

InotiTidy follows the **Clean Architecture** pattern to ensure maintainability:

* **`cmd/`**: The entry point. Manages application lifecycle and OS signals (Graceful Shutdown).
* **`internal/config`**: Handles YAML parsing, path normalization (e.g., converting `~` to absolute paths), and validation.
* **`internal/watcher`**: The core engine. Implements the `fsnotify` loop and the logic for atomic file operations.

---

## 🚀 Installation

### 1. Prerequisites

* A Linux distribution with `systemd`.
* [Go 1.21+](https://www.google.com/search?q=https://go.dev/doc/install) installed and configured in your `$PATH`.

### 2. One-Command Setup

Clone the repository and run the automated installer:

```bash
git clone https://github.com/yourusername/InotiTidy.git
cd InotiTidy
make install

```

**What the installer does:**

1. Compiles the Go source code into a native binary.
2. Moves the binary to `~/.local/bin/`.
3. Initializes a default configuration in `~/.config/inotitidy/config.yaml`.
4. Generates and registers a `systemd` service for your user.
5. Starts the service immediately.

---

## ⚙️ Configuration

The logic of the "Janitor" is controlled entirely via `~/.config/inotitidy/config.yaml`.

```yaml
# Directories to monitor for new files
watch_directories:
  - "~/Downloads"
  - "~/Desktop"

# Global filters: If a filename contains these (case-insensitive), it is ignored
exclude_keywords:
  - "KEEP"
  - "IMPORTANT"
  - "DIPLOM"

# Routing rules
rules:
  - extensions: [".pdf", ".docx", ".txt"]
    target: "~/Documents/Work"
  
  - extensions: [".jpg", ".png", ".svg"]
    target: "~/Pictures/Media"
    
  - extensions: [".mp3", ".flac"]
    target: "~/Music/Library"

  - extensions: [".zip", ".tar.gz", ".rar"]
    target: "~/Downloads/Archives"

```

---

## 📊 Management & Logs

Since InotiTidy runs as a `systemd` unit, you manage it using standard Linux commands:

| Action | Command |
| --- | --- |
| **Check Status** | `systemctl --user status inotitidy` |
| **View Live Logs** | `journalctl --user -u inotitidy -f` |
| **Restart (Apply Config)** | `systemctl --user restart inotitidy` |
| **Stop Service** | `systemctl --user stop inotitidy` |

---

## 🧠 Advanced Features Explained

### Smart Move Logic

If a file with the same name already exists in the target directory, InotiTidy won't overwrite it. It automatically appends a Unix timestamp to the filename (e.g., `report.pdf` becomes `report_1710321456.pdf`).

### Integrity Check

To prevent moving "ghost" or "empty" files, the daemon monitors the file size over a short period. It only triggers the move once the file size stabilizes, ensuring your downloads are finished.

---

## 🤝 Contributing

1. Fork the Project.
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`).
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`).
4. Push to the Branch (`git push origin feature/AmazingFeature`).
5. Open a Pull Request.

---

