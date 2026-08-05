package main

import (
	"InotiTidy/internal/config"
	"InotiTidy/internal/service"
	"InotiTidy/internal/watcher"
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Tokyo Night palette.
var (
	bgBody      = tcell.NewHexColor(0x1a1b26)
	bgPanel     = tcell.NewHexColor(0x24283b)
	fgAccent    = tcell.NewHexColor(0xbb9af7)
	fgPrimary   = tcell.NewHexColor(0xc0caf5)
	fgSecondary = tcell.NewHexColor(0xa9b1d6)
	borderColor = tcell.NewHexColor(0x565f89)
	cyanColor   = tcell.NewHexColor(0x7dcfff)
)

// Dynamic-color tags (constant so they compose into constant format strings).
const (
	greenColor  = "#9ece6a"
	redColor    = "#f7768e"
	yellowColor = "#e0af68"
)

// ui bundles the mutable widgets shared across the TUI closures.
type ui struct {
	app       *tview.Application
	rootPages *tview.Pages
	mainPages *tview.Pages
	sidebar   *tview.List
	header    *tview.TextView
	footer    *tview.TextView
	logView   *tview.TextView
	statsView *tview.TextView
	statusVw  *tview.TextView

	cfg    *config.Config
	engine *watcher.App
	dirty  bool

	journalMu   sync.Mutex
	stopJournal func()

	// Service status is polled in the background so the UI thread never blocks
	// on `systemctl`; refreshDashboard only reads this cache.
	statusMu     sync.Mutex
	svcActive    bool
	svcInstalled bool
}

func applyTheme() {
	tview.Styles.PrimitiveBackgroundColor = bgBody
	tview.Styles.ContrastBackgroundColor = bgPanel
	tview.Styles.MoreContrastBackgroundColor = bgPanel
	tview.Styles.BorderColor = borderColor
	tview.Styles.TitleColor = cyanColor
	tview.Styles.GraphicsColor = borderColor
	tview.Styles.PrimaryTextColor = fgPrimary
	tview.Styles.SecondaryTextColor = fgSecondary
	tview.Styles.TertiaryTextColor = tcell.ColorGray
	tview.Styles.InverseTextColor = bgBody
	tview.Styles.ContrastSecondaryTextColor = fgAccent
}

func handleTUI() error {
	return buildUI().app.Run()
}

// buildUI constructs the whole TUI (widgets, pages, background goroutines) and
// returns it ready to Run. Split out so tests can drive it on a simulation
// screen.
func buildUI() *ui {
	cfg, err := config.Load()
	if err != nil {
		cfg = defaultConfig()
	}

	applyTheme()
	u := &ui{
		app:         tview.NewApplication(),
		rootPages:   tview.NewPages(),
		mainPages:   tview.NewPages(),
		cfg:         cfg,
		engine:      watcher.New(cfg),
		stopJournal: func() {},
	}
	u.engine.LoadStats()

	u.buildLog()
	u.buildDashboard()
	u.buildSidebar()
	u.buildChrome()

	u.rebuildEditors()
	u.startJournal()
	u.startLiveRefresh()

	u.app.SetInputCapture(u.globalKeys)
	u.app.SetRoot(u.rootPages, true).EnableMouse(true)
	u.setPage("Dashboard")
	return u
}

func defaultConfig() *config.Config {
	home, _ := os.UserHomeDir()
	return &config.Config{
		Watch:          []config.WatchDir{{Path: filepath.Join(home, "Downloads")}},
		SettleInterval: config.DefaultSettleInterval,
	}
}

// ---- logging ---------------------------------------------------------------

func (u *ui) buildLog() {
	u.logView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true).
		SetChangedFunc(func() { u.app.Draw() })
	u.logView.SetBorder(true).SetTitle(" [white::b]Service Logs (journalctl --user)[-:-:-] ")
	u.logView.SetBackgroundColor(tcell.NewHexColor(0x16161e))
}

func (u *ui) logf(format string, a ...any) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(u.logView, "[#565f89]%s[-] %s\n", ts, fmt.Sprintf(format, a...))
	u.logView.ScrollToEnd()
}

// logUI is the same as logf but safe to call from any goroutine.
func (u *ui) logUI(format string, a ...any) {
	u.app.QueueUpdateDraw(func() { u.logf(format, a...) })
}

func (u *ui) startJournal() {
	go func() {
		for {
			ctx, cancel := context.WithCancel(context.Background())
			u.journalMu.Lock()
			u.stopJournal = cancel
			u.journalMu.Unlock()

			cmd := exec.CommandContext(ctx, "journalctl", service.JournalArgs()...)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				cancel()
				time.Sleep(5 * time.Second)
				continue
			}
			if err := cmd.Start(); err != nil {
				cancel()
				time.Sleep(5 * time.Second)
				continue
			}
			sc := bufio.NewScanner(stdout)
			for sc.Scan() {
				line := sc.Text()
				u.app.QueueUpdateDraw(func() {
					fmt.Fprintf(u.logView, "[#a9b1d6]%s[-]\n", line)
					u.logView.ScrollToEnd()
				})
			}
			_ = sc.Err()
			cancel()
			_ = cmd.Wait()
			time.Sleep(2 * time.Second)
		}
	}()
}

func (u *ui) stopJournalStream() {
	u.journalMu.Lock()
	fn := u.stopJournal
	u.journalMu.Unlock()
	if fn != nil {
		fn()
	}
}

// ---- dashboard -------------------------------------------------------------

func (u *ui) buildDashboard() {
	u.statsView = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	u.statusVw = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)

	// Compact, single-line action list so the flexible log panel below always
	// keeps room. (Descriptions moved into the labels.)
	actions := tview.NewList().ShowSecondaryText(false)
	actions.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(cyanColor)
	actions.SetMainTextColor(fgPrimary)
	actions.
		AddItem("Start service", "", '1', func() { u.serviceAction("start") }).
		AddItem("Stop service", "", '2', func() { u.serviceAction("stop") }).
		AddItem("Restart service (reload config)", "", 'r', func() { u.serviceAction("restart") }).
		AddItem("Install / enable service", "", 'i', u.installService).
		AddItem("Scan now", "", '3', u.scanNow)

	// Fixed rows kept small (2+3+5 = 10) so the log panel gets the remaining
	// height instead of being starved to zero on normal-size terminals.
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(u.statsView, 2, 0, false).
		AddItem(u.statusVw, 3, 0, false).
		AddItem(actions, 5, 0, true).
		AddItem(u.logView, 0, 1, false)
	flex.SetBorder(true).SetTitle(" [white::b]Control Dashboard[-:-:-] ")

	u.mainPages.AddPage("Dashboard", flex, true, false)
	u.refreshDashboard()
}

// refreshDashboard renders the cached state. It is cheap and must only run on
// the UI goroutine — no exec, no blocking I/O here.
func (u *ui) refreshDashboard() {
	s := u.engine.Snapshot()

	top, max := "N/A", 0
	for ext, c := range s.ExtensionCounts {
		if c > max {
			max, top = c, ext
		}
	}
	u.statsView.SetText(fmt.Sprintf(
		"\n[white::b]Total:[-] [#bb9af7]%d[-]    [white::b]Today:[-] [#9ece6a]%d[-]    [white::b]Top type:[-] [#7dcfff]%s[-]",
		s.TotalSorted, s.TodaySorted, top))

	u.statusMu.Lock()
	active, installed := u.svcActive, u.svcInstalled
	u.statusMu.Unlock()

	status := "[" + redColor + "]STOPPED[-]"
	if active {
		status = "[" + greenColor + "]RUNNING[-]"
	} else if !installed {
		status = "[" + yellowColor + "]NOT INSTALLED[-]"
	}
	u.statusVw.SetText(fmt.Sprintf(
		"\n[white::b]Service:[-] %s\n[#a9b1d6]%d watch dir(s) · %d rule(s)[-]",
		status, len(u.cfg.Watch), len(u.cfg.Rules)))
}

// pollStatus refreshes the service status and stats off the UI thread, then
// queues a cheap redraw.
func (u *ui) pollStatus() {
	active := service.IsActive()
	installed := service.Installed()
	u.engine.LoadStats()

	u.statusMu.Lock()
	u.svcActive, u.svcInstalled = active, installed
	u.statusMu.Unlock()

	u.app.QueueUpdateDraw(u.refreshDashboard)
}

func (u *ui) startLiveRefresh() {
	go func() {
		u.pollStatus()
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for range t.C {
			u.pollStatus()
		}
	}()
}

func (u *ui) serviceAction(kind string) {
	go func() {
		var err error
		switch kind {
		case "start":
			u.logUI("[yellow]Starting service...[-]")
			err = service.Start()
		case "stop":
			u.logUI("[yellow]Stopping service...[-]")
			err = service.Stop()
		case "restart":
			u.logUI("[yellow]Restarting service...[-]")
			err = service.Restart()
		}
		if err != nil {
			u.logUI("["+redColor+"]%s failed: %v[-]", kind, err)
		} else {
			u.logUI("["+greenColor+"]%s ok[-]", kind)
		}
		u.pollStatus()
	}()
}

func (u *ui) installService() {
	u.confirm("Install inotitidy.service as a systemd --user service?\n\nNo root or sudo required.", func() {
		go func() {
			exe, _ := os.Executable()
			exe, _ = filepath.Abs(exe)
			u.logUI("[yellow]Installing user service...[-]")
			if err := service.Install(exe, config.GetConfigDir()); err != nil {
				u.logUI("["+redColor+"]Install failed: %v[-]", err)
				return
			}
			if err := service.Enable(); err != nil {
				u.logUI("["+redColor+"]Enable failed: %v[-]", err)
				return
			}
			u.logUI("[" + greenColor + "]Service installed and enabled[-]")
			u.pollStatus()
		}()
	})
}

func (u *ui) scanNow() {
	go func() {
		if service.IsActive() {
			u.logUI("[yellow]Service is running; the daemon already sorts these folders.[-]")
			return
		}
		u.logUI("[#bb9af7]Scanning now...[-]")
		u.engine.SetConfig(u.cfg)
		u.engine.LoadStats()
		u.engine.ScanAll()
		u.logUI("[" + greenColor + "]Scan finished[-]")
		u.pollStatus()
	}()
}

// ---- sidebar & chrome ------------------------------------------------------

func (u *ui) buildSidebar() {
	u.sidebar = tview.NewList().ShowSecondaryText(true)
	u.sidebar.SetBorder(true).SetTitle(" [white::b]Menu[-:-:-] ")
	u.sidebar.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(cyanColor)
	u.sidebar.SetMainTextColor(fgPrimary).SetSecondaryTextColor(fgSecondary)

	u.sidebar.
		AddItem("Dashboard", "Control & stats", 'd', func() { u.setPage("Dashboard") }).
		AddItem("Watch Directories", "Folders to monitor", '1', func() { u.setPage("DirsList") }).
		AddItem("Exclude Keywords", "Skip matching files", '2', func() { u.setPage("ExcList") }).
		AddItem("Routing Rules", "Conditions -> actions", '3', func() { u.setPage("RulesList") }).
		AddItem("Preview (dry-run)", "See planned moves", 'p', func() { u.showPreview() }).
		AddItem("Undo last batch", "Reverse recent moves", 'u', u.undoLast).
		AddItem("Save Config", "Write to disk", 's', u.saveConfig).
		AddItem("Quit", "Exit", 'q', u.quit)
}

func (u *ui) buildChrome() {
	u.header = tview.NewTextView().SetDynamicColors(true)
	u.refreshHeader()

	u.footer = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	u.setFooter("[#7dcfff]↑↓[-] navigate  [#7dcfff]Enter[-] select  [#7dcfff]Tab[-] switch pane  [#7dcfff]Esc[-] back  [#7dcfff]q[-] quit")

	content := tview.NewFlex().
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(u.sidebar, 32, 0, true).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(u.mainPages, 0, 2, false).
		AddItem(tview.NewBox(), 1, 0, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(u.header, 1, 0, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(content, 0, 1, true).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(u.footer, 1, 0, false).
		AddItem(tview.NewBox(), 1, 0, false)
	root.SetBackgroundColor(bgBody)

	u.rootPages.AddPage("Main", root, true, true)
}

func (u *ui) refreshHeader() {
	flag := ""
	if u.dirty {
		flag = "  [" + yellowColor + "]● unsaved[-]"
	}
	u.header.SetText(fmt.Sprintf("  [#bb9af7::b]⚡ InotiTidy[-:-:-] [#565f89]%s[-]%s", version, flag))
}

func (u *ui) setFooter(text string) { u.footer.SetText("[#a9b1d6]" + text + "[-]") }

func (u *ui) markDirty() {
	if !u.dirty {
		u.dirty = true
		u.refreshHeader()
	}
}

func (u *ui) setPage(name string) {
	u.mainPages.SwitchToPage(name)
	u.app.SetFocus(u.mainPages)
	switch name {
	case "DirsList":
		u.setFooter("[#7dcfff]a[-] add  [#7dcfff]Enter[-] toggle recursive  [#7dcfff]d[-] remove  [#7dcfff]Esc[-] back")
	case "RulesList":
		u.setFooter("[#7dcfff]a[-] add rule  [#7dcfff]Enter[-] edit  [#7dcfff]d[-] remove  [#7dcfff]Esc[-] back")
	case "ExcList":
		u.setFooter("[#7dcfff]a[-] add keyword  [#7dcfff]d[-] remove  [#7dcfff]Esc[-] back")
	default:
		u.setFooter("[#7dcfff]↑↓[-] navigate  [#7dcfff]Enter[-] select  [#7dcfff]Tab[-] switch pane  [#7dcfff]Esc[-] back")
	}
}

// modalOpen reports whether a form, picker or modal is on top. While one is
// open the global Tab/Esc shortcuts must yield to it, otherwise Tab would jump
// focus out of the form (e.g. right after choosing an Action) instead of moving
// to the next field.
func (u *ui) modalOpen() bool {
	if name, _ := u.rootPages.GetFrontPage(); name != "Main" {
		return true
	}
	switch name, _ := u.mainPages.GetFrontPage(); name {
	case "RuleForm", "RuleFormAdv", "Form", "Picker":
		return true
	}
	return false
}

func (u *ui) globalKeys(event *tcell.EventKey) *tcell.EventKey {
	// Let forms/pickers/modals handle their own Tab and Esc navigation.
	if u.modalOpen() {
		return event
	}
	switch event.Key() {
	case tcell.KeyEsc:
		u.app.SetFocus(u.sidebar)
		return nil
	case tcell.KeyTab:
		if u.sidebar.HasFocus() {
			u.app.SetFocus(u.mainPages)
		} else {
			u.app.SetFocus(u.sidebar)
		}
		return nil
	}
	return event
}

func (u *ui) quit() {
	if u.dirty {
		u.confirm("You have unsaved changes. Quit without saving?", func() {
			u.stopJournalStream()
			u.app.Stop()
		})
		return
	}
	u.stopJournalStream()
	u.app.Stop()
}
