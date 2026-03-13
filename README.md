# ⚡ InotiTidy

**InotiTidy** is a modern file organizer for Linux. It combines a powerful background file sorter with a premium "Tokyo Night" themed Management Console. 

With InotiTidy, you don't need to manually configure YAML files or manage complex `systemd` services — everything is handled directly from the terminal interface.

---

## 🌟 Features

- **Unified Management Console**: Start, stop, and monitor your file sorting service from a single dashboard.
- **Live Activity Feed**: See real-time sorting events directly in the TUI window.
- **Premium Tokyo Night Theme**: A beautiful, high-contrast terminal interface.
- **Intelligent Sorting**: Moves files based on extensions only after they stop growing (safe for large downloads).
- **Custom Filters**: Exclude files by keywords and map specific extensions to target folders.
- **Conflict Management**: Automatically handles filename collisions by appending timestamps.

---

## 🚀 Getting Started

### 1) Prerequisites
- Linux
- Go `1.21+`

### 2) Build & Launch
To get the latest version of InotiTidy running:

```bash
# Build the project
make build

# Launch the Management Console
./inotitidy
```

---

## ⚙️ How to Use (TUI Console)

Once you launch `./inotitidy`, you'll enter the **Management Console**.

### The Dashboard
- **Start/Stop Service**: Use the buttons on the dashboard to control the background file monitor.
- **Service Status**: Instantly see if the sorter is active (`RUNNING`) or inactive (`STOPPED`).
- **Activity Log**: Watch the bottom pane for live feedback on files being moved.

### Configuration Sections
- **Watch Directories**: Add/remove folders you want the app to monitor (e.g., `~/Downloads`).
- **Exclude Keywords**: Define words that, if found in a filename, will cause it to be ignored.
- **Routing Rules**: Map extensions (e.g., `.pdf`, `.jpg`) to destination folders.

### Console Navigation
- **Arrows (↑/↓)**: Navigate through lists and menus.
- **Tab**: Switch focus between the **Sidebar** and the **Main Area**.
- **Enter**: Confirm an action, edit a field, or "focus" into a list.
- **Esc**: Return focus to the Sidebar or cancel a form.
- **q / Ctrl+C**: Stop the service and exit the console.

---

## 🧪 Development & Manual Build

If you are working on the code, you can run the console directly from source:
```bash
go run ./cmd/inotitidy
```

---

## 🔍 Troubleshooting

### Changes don't apply immediately?
If you add new Watch Directories or Rules while the service is **RUNNING**, you should **Stop** and then **Start** the service again via the Dashboard to reload the new configuration.

### Where is the config file?
InotiTidy saves all settings to `~/.config/inotitidy/config.yaml`. The TUI manages this file for you automatically.

---

## 🤝 Contributing
1. Fork it.
2. Create your feature branch.
3. Open a Pull Request!
