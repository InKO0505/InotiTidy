# ⚡ InotiTidy

**InotiTidy** is an event-driven file organizer for Linux. A low-CPU background
daemon watches your folders and sorts files with a small **rules engine**
(conditions, then actions), and a premium *Tokyo Night* TUI lets you manage
everything: rules, the service, previews and undo, all from the terminal.

No root required: the daemon runs as a **systemd `--user` service**.

---

## 🌟 Features

- **Rules engine**: match files by extension, glob, regex, size, age or MIME
  type, then **move**, **copy** or **trash** them.
- **Safe by design**: dry-run **preview**, an **undo** journal, a **trash**
  action (freedesktop.org trash, not `rm`), and never-overwrite conflict handling.
- **Date foldering**: target templates like `~/Pictures/{year}/{month}`.
- **Hot reload**: the daemon re-reads `config.yaml` the moment you save it.
- **No sudo**: installs and runs under `systemctl --user`.
- **Unified TUI**: dashboard, live stats, service control, rule editor, preview
  and log viewer, all keyboard-driven.
- **Persistent stats** and per-day counters.

---

## 🚀 Getting started

### Requirements
- Linux with systemd
- Go `1.24+` (to build from source)

### Install

```bash
make install          # builds, installs to ~/.local/bin, enables the user service
```

or manually:

```bash
make build
./inotitidy --install   # set up + enable the systemd --user service
./inotitidy             # launch the TUI console
```

---

## ⌨️ Command line

```bash
inotitidy               # launch the TUI console
inotitidy --daemon      # run the watcher in the foreground (used by systemd)
inotitidy --scan        # sort existing files once, then exit
inotitidy --dry-run     # print what would be sorted, change nothing
inotitidy --undo        # reverse the most recent batch of moves
inotitidy --install     # install & enable the systemd --user service
inotitidy --version
```

Follow the daemon:

```bash
journalctl --user -u inotitidy.service -f
```

---

## ⚙️ Configuration

Config lives at `~/.config/inotitidy/config.yaml` and is managed by the TUI, but
it is plain YAML you can hand-edit. A rule matches only when **every** condition
you set passes.

```yaml
version: 2
settle_interval: 500ms          # wait until a file stops growing

watch:
  - path: "~/Downloads"
  - path: "~/Projects"
    recursive: true

global_exclude: ["KEEP", "IMPORTANT"]

rules:
  - name: Invoices
    name_regex: "^INV-\\d+"
    action: move
    target: "~/Documents/Invoices"

  - name: Photos by year
    extensions: [jpg, jpeg, png]
    action: move
    target: "~/Pictures/{year}/{month}"

  - name: Big videos
    extensions: [mkv, mp4]
    min_size: 200MB
    action: move
    target: "~/Videos"
    on_conflict: skip

  - name: Clean temp files
    name_glob: "*.tmp"
    older_than: 24h
    action: trash
```

**Conditions:** `extensions`, `name_glob`, `name_regex`, `min_size`, `max_size`,
`older_than`, `newer_than`, `by_content` (match extensions against the detected
MIME type), `exclude`.
**Actions:** `move`, `copy`, `trash`.
**Conflict strategies:** `rename` (default), `skip`, `overwrite`, `trash`.
**Target templates:** `{year}` `{month}` `{day}` `{ext}`.

The old v1 format (`watch_directories`, `exclude_keywords`,
`rules: [{extensions, target}]`) still loads and is upgraded automatically.

---

## 🖥️ TUI navigation

- **↑/↓** move · **Enter** select/edit · **Tab** switch pane · **Esc** back
- **a** add · **d** remove · **p** preview · **u** undo · **s** save · **q** quit
- A **● unsaved** marker appears until you save; quitting asks to confirm.

---

## 🧪 Development

```bash
make race     # go test -race ./...
make lint     # gofmt + vet + race tests
make build VERSION=$(git describe --tags)
```

---

## 🤝 Contributing
1. Fork it.
2. Create your feature branch.
3. Open a Pull Request.
