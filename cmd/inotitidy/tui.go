package main

import (
	"InotiTidy/internal/config"
	"InotiTidy/internal/watcher"
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func handleTUI() error {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}

	app := tview.NewApplication()
	rootPages := tview.NewPages()

	// Tokyo Night Colors
	bgBody := tcell.NewHexColor(0x1a1b26)
	bgPanel := tcell.NewHexColor(0x24283b)
	fgAccent := tcell.NewHexColor(0xbb9af7)
	fgPrimary := tcell.NewHexColor(0xc0caf5)
	fgSecondary := tcell.NewHexColor(0xa9b1d6)
	borderColor := tcell.NewHexColor(0x565f89)
	cyanColor := tcell.NewHexColor(0x7dcfff)

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

	mainPages := tview.NewPages()

	sidebar := tview.NewList().ShowSecondaryText(true)
	sidebar.SetBorder(true).SetTitle(" [white::b]Main Menu[-:-:-] ")
	sidebar.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(cyanColor)
	sidebar.SetMainTextColor(fgPrimary).SetSecondaryTextColor(fgSecondary)

	// --- Log View ---
	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	logView.SetBorder(true).SetTitle(" [white::b]System Service Logs (journalctl)[-:-:-] ")
	logView.SetBackgroundColor(tcell.NewHexColor(0x16161e))

	logToUI := func(msg string) {
		timestamp := time.Now().Format("15:04:05")
		fmt.Fprintf(logView, "[#565f89]%s[-] %s\n", timestamp, msg)
		logView.ScrollToEnd()
	}

	// --- Service Helpers ---
	isServiceActive := func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "inotitidy.service")
		err := cmd.Run()
		return err == nil && ctx.Err() == nil
	}

	runWithElevation := func(commandName string, args ...string) error {
		type runner struct {
			name    string
			args    []string
			label   string
			timeout time.Duration
		}

		runners := []runner{
			{name: "pkexec", args: append([]string{commandName}, args...), label: "pkexec", timeout: 8 * time.Second},
			{name: "sudo", args: append([]string{"-n", commandName}, args...), label: "sudo", timeout: 5 * time.Second},
			{name: commandName, args: args, label: "direct", timeout: 5 * time.Second},
		}

		var errors []string
		for _, r := range runners {
			if _, err := exec.LookPath(r.name); err != nil {
				errors = append(errors, fmt.Sprintf("%s unavailable", r.label))
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
			cmd := exec.CommandContext(ctx, r.name, r.args...)
			out, err := cmd.CombinedOutput()
			cancel()

			if err == nil {
				return nil
			}

			outStr := strings.TrimSpace(string(out))
			if ctx.Err() == context.DeadlineExceeded {
				errors = append(errors, fmt.Sprintf("%s timed out", r.label))
				continue
			}
			if outStr == "" {
				errors = append(errors, fmt.Sprintf("%s: %v", r.label, err))
			} else {
				errors = append(errors, fmt.Sprintf("%s: %v (%s)", r.label, err, outStr))
			}
		}

		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	var stopJournalOnce = func() {} // To be assigned later for log piping

	// --- Dashboard Stats Refresh ---
	var updateDashboard func()

	startWatcher := func() {
		go func() {
			app.QueueUpdateDraw(func() {
				logToUI("[yellow]Attempting to start systemd service...[-]")
			})

			if err := runWithElevation("systemctl", "start", "inotitidy.service"); err != nil {
				app.QueueUpdateDraw(func() {
					logToUI(fmt.Sprintf("[red]Service failed to start: %v[-]", err))
					updateDashboard()
				})
				return
			}

			serviceActive := isServiceActive()
			app.QueueUpdateDraw(func() {
				if serviceActive {
					logToUI("[#9ece6a]Service started successfully[-]")
				} else {
					logToUI("[yellow]Start command ran, but service is still not active. Check journal logs.[-]")
				}
				updateDashboard()
			})
		}()
	}

	stopWatcher := func() {
		go func() {
			app.QueueUpdateDraw(func() {
				logToUI("[yellow]Attempting to stop systemd service...[-]")
			})

			if err := runWithElevation("systemctl", "stop", "inotitidy.service"); err != nil {
				app.QueueUpdateDraw(func() {
					logToUI(fmt.Sprintf("[red]Service failed to stop: %v[-]", err))
					updateDashboard()
				})
				return
			}

			serviceActive := isServiceActive()
			app.QueueUpdateDraw(func() {
				if serviceActive {
					logToUI("[yellow]Stop command ran, but service is still active.[-]")
				} else {
					logToUI("[#f7768e]Service stopped successfully[-]")
				}
				updateDashboard()
			})
		}()
	}

	// --- Log Piping (Journalctl) ---
	go func() {
		for {
			ctx, cancel := context.WithCancel(context.Background())
			stopJournalOnce = cancel

			cmd := exec.CommandContext(ctx, "journalctl", "-u", "inotitidy.service", "-f", "-n", "20", "--no-hostname")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			if err := cmd.Start(); err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				// Clean up journalctl output a bit if needed
				app.QueueUpdateDraw(func() {
					fmt.Fprintf(logView, "[#a9b1d6]%s[-]\n", line)
					logView.ScrollToEnd()
				})
			}

			if err := scanner.Err(); err != nil {
				app.QueueUpdateDraw(func() {
					logToUI(fmt.Sprintf("[yellow]journalctl stream interrupted: %v[-]", err))
				})
			}

			cancel()
			_ = cmd.Wait()
			time.Sleep(2 * time.Second) // Wait before retry if crashed
		}
	}()

	// --- Service Installation ---
	installService := func() {
		exePath, _ := os.Executable()
		absExePath, _ := filepath.Abs(exePath)

		serviceContent := fmt.Sprintf(`[Unit]
Description=InotiTidy File Organizer
After=network.target

[Service]
Type=simple
ExecStart=%s --daemon
Restart=always
User=%s

[Install]
WantedBy=multi-user.target
`, absExePath, os.Getenv("USER"))

		tmpPath := "/tmp/inotitidy.service"
		if err := os.WriteFile(tmpPath, []byte(serviceContent), 0644); err != nil {
			logToUI(fmt.Sprintf("[red]Failed to prepare service file: %v[-]", err))
			return
		}

		modal := tview.NewModal().
			SetText("Install inotitidy.service to /etc/systemd/system/?\n(Requires sudo)").
			AddButtons([]string{"Install", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				if buttonLabel == "Install" {
					go func() {
						app.QueueUpdateDraw(func() {
							logToUI("[yellow]Asking for permission to install service...[-]")
						})
						if err := runWithElevation("cp", tmpPath, "/etc/systemd/system/inotitidy.service"); err != nil {
							app.QueueUpdateDraw(func() {
								logToUI(fmt.Sprintf("[red]Installation failed: %v[-]", err))
							})
							return
						}

						if err := runWithElevation("systemctl", "daemon-reload"); err != nil {
							app.QueueUpdateDraw(func() {
								logToUI(fmt.Sprintf("[red]Service copied but daemon-reload failed: %v[-]", err))
							})
							return
						}

						app.QueueUpdateDraw(func() {
							logToUI("[#9ece6a]Service installed and daemon reloaded successfully[-]")
							updateDashboard()
						})
					}()
				}
				rootPages.RemovePage("InstallModal")
			})
		modal.SetBackgroundColor(bgPanel).SetTextColor(fgPrimary)
		rootPages.AddPage("InstallModal", modal, true, true)
		app.SetFocus(modal)
	}

	// --- Directory Picker Component ---
	showDirPicker := func(onSelect func(string)) {
		currentPath, _ := os.Getwd()
		list := tview.NewList()

		var updateList func(string)
		updateList = func(targetPath string) {
			list.Clear()
			currentPath = targetPath
			list.SetTitle(fmt.Sprintf(" [white::b]Select Directory:[-] %s ", currentPath))

			list.AddItem(".. [Select Parent]", "", '.', func() {
				updateList(filepath.Dir(currentPath))
			})

			entries, _ := os.ReadDir(currentPath)
			for _, entry := range entries {
				if entry.IsDir() {
					name := entry.Name()
					full := filepath.Join(currentPath, name)
					list.AddItem(name, "Navigate into folder", 0, func() {
						updateList(full)
					})
				}
			}

			list.AddItem("[#9ece6a]SELECT THIS DIRECTORY[-]", "Choose current path", 's', func() {
				onSelect(currentPath)
				mainPages.RemovePage("Picker")
			})
			list.AddItem("[#f7768e]CANCEL[-]", "Go back", 'q', func() {
				mainPages.RemovePage("Picker")
			})
		}

		updateList(currentPath)
		list.SetBorder(true)
		list.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(cyanColor)

		modal := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(list, 20, 1, true).
				AddItem(nil, 0, 1, false), 60, 1, true).
			AddItem(nil, 0, 1, false)

		mainPages.AddPage("Picker", modal, true, true)
		app.SetFocus(list)
	}

	// --- Dashboard Construction ---
	createDashboard := func() *tview.Flex {
		isRunning := isServiceActive()
		statusText := "[#f7768e]STOPPED[-]"
		if isRunning {
			statusText = "[#9ece6a]RUNNING[-]"
		}

		// Stats (Still from watcherApp for now)
		watcherApp := &watcher.App{Config: cfg}
		watcherApp.LoadStats()

		mostCommon := "N/A"
		maxCount := 0
		for ext, count := range watcherApp.Stats.ExtensionCounts {
			if count > maxCount {
				maxCount = count
				mostCommon = ext
			}
		}

		statsView := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText(fmt.Sprintf(
				"\n[white::b]Total Sorted:[-] [#bb9af7]%d[-]   [white::b]Today:[-] [#9ece6a]%d[-]   [white::b]Top Type:[-] [#7dcfff]%s[-]\n",
				watcherApp.Stats.TotalSorted, watcherApp.Stats.TodaySorted, mostCommon,
			))

		info := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText(fmt.Sprintf("\n[white::b]System Service Status:[-] %s\n\n[#a9b1d6]Manage the background systemd service.[-]", statusText))

		actions := tview.NewList().
			AddItem("Start Service", "sudo systemctl start", '1', startWatcher).
			AddItem("Stop Service", "sudo systemctl stop", '2', stopWatcher).
			AddItem("Install/Setup Service", "Create inotitidy.service", 'i', installService).
			AddItem("Clean All Now", "Process files manually in TUI", '3', func() {
				go func() {
					app.QueueUpdateDraw(func() {
						logToUI("[#bb9af7]Manually triggered clean (Internal)...[-]")
					})
					w := &watcher.App{Config: cfg, Logger: func(msg string) {
						app.QueueUpdateDraw(func() { logToUI(msg) })
					}}
					w.ScanAll()
				}()
			})
		actions.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(cyanColor)
		actions.SetMainTextColor(fgPrimary).SetSecondaryTextColor(fgSecondary)

		flex := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(statsView, 3, 1, false).
			AddItem(info, 4, 1, false).
			AddItem(actions, 9, 1, true).
			AddItem(tview.NewBox(), 1, 0, false).
			AddItem(logView, 0, 2, false)

		flex.SetBorder(true).SetTitle(" [white::b]Control Dashboard[-:-:-] ")
		return flex
	}

	updateDashboard = func() {
		mainPages.AddPage("Dashboard", createDashboard(), true, true)
	}

	var populateMainPages func()

	createDirsList := func() *tview.List {
		list := tview.NewList()
		list.SetBorder(true).SetTitle(" [white::b]Watch Directories[-:-:-] ")
		list.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(fgAccent)

		for i, dir := range cfg.WatchDirs {
			idx := i
			list.AddItem(dir, "Enter/Digit to remove", rune(49+i), func() {
				cfg.WatchDirs = append(cfg.WatchDirs[:idx], cfg.WatchDirs[idx+1:]...)
				populateMainPages()
				mainPages.SwitchToPage("DirsList")
				app.SetFocus(mainPages)
			})
		}
		list.AddItem("Browse & Add Directory", "Use picker to add new path", 'a', func() {
			showDirPicker(func(path string) {
				cfg.WatchDirs = append(cfg.WatchDirs, path)
				populateMainPages()
				mainPages.SwitchToPage("DirsList")
				app.SetFocus(mainPages)
			})
		})
		return list
	}

	createExcList := func() *tview.List {
		list := tview.NewList()
		list.SetBorder(true).SetTitle(" [white::b]Exclude Keywords[-:-:-] ")
		list.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(fgAccent)

		for i, exc := range cfg.Excludes {
			idx := i
			list.AddItem(exc, "Enter/Digit to remove", rune(49+i), func() {
				cfg.Excludes = append(cfg.Excludes[:idx], cfg.Excludes[idx+1:]...)
				populateMainPages()
				mainPages.SwitchToPage("ExcList")
				app.SetFocus(mainPages)
			})
		}
		list.AddItem("Add New Keyword", "Press 'a' or Enter to add", 'a', func() {
			mainPages.SwitchToPage("AddExcForm")
			app.SetFocus(mainPages)
		})
		return list
	}

	createRulesList := func() *tview.List {
		list := tview.NewList()
		list.SetBorder(true).SetTitle(" [white::b]Routing Rules[-:-:-] ")
		list.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(fgAccent)

		for i, rule := range cfg.Rules {
			idx := i
			exts := strings.Join(rule.Extensions, ", ")
			list.AddItem(fmt.Sprintf("%s -> %s", exts, rule.Target), "Enter/Digit to remove", rune(49+i), func() {
				cfg.Rules = append(cfg.Rules[:idx], cfg.Rules[idx+1:]...)
				populateMainPages()
				mainPages.SwitchToPage("RulesList")
				app.SetFocus(mainPages)
			})
		}
		list.AddItem("Add New Rule", "Press 'a' or Enter to add", 'a', func() {
			mainPages.SwitchToPage("AddRuleForm")
			app.SetFocus(mainPages)
		})
		return list
	}

	populateMainPages = func() {
		mainPages.RemovePage("DirsList")
		mainPages.RemovePage("ExcList")
		mainPages.RemovePage("RulesList")

		mainPages.AddPage("DirsList", createDirsList(), true, false)
		mainPages.AddPage("ExcList", createExcList(), true, false)
		mainPages.AddPage("RulesList", createRulesList(), true, false)
		updateDashboard()
	}

	createAddExcForm := func() *tview.Form {
		form := tview.NewForm()
		form.AddInputField("Keyword", "", 40, nil, nil).
			AddButton("Save", func() {
				kw := form.GetFormItemByLabel("Keyword").(*tview.InputField).GetText()
				if kw != "" {
					cfg.Excludes = append(cfg.Excludes, kw)
					form.GetFormItemByLabel("Keyword").(*tview.InputField).SetText("")
				}
				populateMainPages()
				mainPages.SwitchToPage("ExcList")
				app.SetFocus(mainPages)
			}).
			AddButton("Cancel", func() {
				form.GetFormItemByLabel("Keyword").(*tview.InputField).SetText("")
				mainPages.SwitchToPage("ExcList")
				app.SetFocus(mainPages)
			})
		form.SetBorder(true).SetTitle(" [white::b]Add Exclude Keyword[-:-:-] ")
		form.SetButtonBackgroundColor(bgPanel).SetButtonTextColor(cyanColor)
		form.SetFieldBackgroundColor(bgPanel).SetFieldTextColor(fgPrimary)
		return form
	}

	createAddRuleForm := func() *tview.Form {
		form := tview.NewForm()
		form.AddInputField("Extensions (comma-sep)", "", 40, nil, nil).
			AddInputField("Target Path", "", 40, nil, nil).
			AddButton("Browse Target", func() {
				showDirPicker(func(path string) {
					form.GetFormItemByLabel("Target Path").(*tview.InputField).SetText(path)
					app.SetFocus(form)
				})
			}).
			AddButton("Save", func() {
				exts := form.GetFormItemByLabel("Extensions (comma-sep)").(*tview.InputField).GetText()
				target := form.GetFormItemByLabel("Target Path").(*tview.InputField).GetText()
				if exts != "" && target != "" {
					extList := strings.Split(exts, ",")
					for i := range extList {
						extList[i] = strings.ToLower(strings.TrimSpace(extList[i]))
					}
					cfg.Rules = append(cfg.Rules, config.Rule{Extensions: extList, Target: target})
					form.GetFormItemByLabel("Extensions (comma-sep)").(*tview.InputField).SetText("")
					form.GetFormItemByLabel("Target Path").(*tview.InputField).SetText("")
				}
				populateMainPages()
				mainPages.SwitchToPage("RulesList")
				app.SetFocus(mainPages)
			}).
			AddButton("Cancel", func() {
				form.GetFormItemByLabel("Extensions (comma-sep)").(*tview.InputField).SetText("")
				form.GetFormItemByLabel("Target Path").(*tview.InputField).SetText("")
				mainPages.SwitchToPage("RulesList")
				app.SetFocus(mainPages)
			})
		form.SetBorder(true).SetTitle(" [white::b]Add Rule[-:-:-] ")
		form.SetButtonBackgroundColor(bgPanel).SetButtonTextColor(cyanColor)
		form.SetFieldBackgroundColor(bgPanel).SetFieldTextColor(fgPrimary)
		return form
	}

	mainPages.AddPage("AddExcForm", createAddExcForm(), true, false)
	mainPages.AddPage("AddRuleForm", createAddRuleForm(), true, false)
	populateMainPages()

	sidebar.AddItem("Dashboard", "Control & Stats", 'd', func() {
		mainPages.SwitchToPage("Dashboard")
		app.SetFocus(mainPages)
	}).
		AddItem("Watch Directories", "Monitor folders", '1', func() {
			mainPages.SwitchToPage("DirsList")
			app.SetFocus(mainPages)
		}).
		AddItem("Exclude Keywords", "Skip files", '2', func() {
			mainPages.SwitchToPage("ExcList")
			app.SetFocus(mainPages)
		}).
		AddItem("Routing Rules", "Map extensions", '3', func() {
			mainPages.SwitchToPage("RulesList")
			app.SetFocus(mainPages)
		}).
		AddItem("Save Config", "Write to disk", 's', func() {
			modal := tview.NewModal()
			dismiss := func() {
				rootPages.RemovePage("SaveModal")
				app.SetFocus(sidebar)
			}

			if err := cfg.Save(config.GetConfigPath()); err != nil {
				modal.SetText(fmt.Sprintf("Error saving configuration:\n%v", err)).
					AddButtons([]string{"OK"})
			} else {
				modal.SetText("Configuration saved successfully!\n\nDon't forget to restart service to apply changes.\n\n(Press any key to close)").
					AddButtons([]string{"OK"})
			}
			modal.SetBackgroundColor(bgPanel).SetTextColor(fgPrimary)
			modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				dismiss()
			})
			modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				dismiss()
				return nil
			})
			rootPages.AddPage("SaveModal", modal, true, true)
			app.SetFocus(modal)
		}).
		AddItem("Quit", "Exit app", 'q', func() {
			stopJournalOnce()
			app.Stop()
		})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			app.SetFocus(sidebar)
			return nil
		}
		if event.Key() == tcell.KeyTab {
			if sidebar.HasFocus() {
				app.SetFocus(mainPages)
			} else {
				app.SetFocus(sidebar)
			}
			return nil
		}
		return event
	})

	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("  [#bb9af7::b]⚡ InotiTidy System Console[-:-:-] [#565f89]v1.5[-:-:-]")

	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[#a9b1d6]Navigate: [#7dcfff]\u2191\u2193[#a9b1d6] \u2022 Focus: [#7dcfff]Enter/Tab[#a9b1d6] \u2022 Back: [#7dcfff]Esc[#a9b1d6] \u2022 Exit: [#7dcfff]q/Ctrl+C[-:-:-]")

	contentFlex := tview.NewFlex().
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(sidebar, 35, 1, true).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(mainPages, 0, 2, false).
		AddItem(tview.NewBox(), 1, 0, false)

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(header, 1, 1, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(footer, 1, 1, false).
		AddItem(tview.NewBox(), 1, 0, false)

	mainFlex.SetBackgroundColor(bgBody)
	rootPages.AddPage("Main", mainFlex, true, true)

	app.SetRoot(rootPages, true).EnableMouse(true)
	return app.Run()
}
