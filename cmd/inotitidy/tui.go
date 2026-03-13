package main

import (
	"InotiTidy/internal/config"
	"InotiTidy/internal/watcher"
	"context"
	"fmt"
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
	logView.SetBorder(true).SetTitle(" [white::b]Live Activity Feed[-:-:-] ")
	logView.SetBackgroundColor(tcell.NewHexColor(0x16161e))

	logToUI := func(msg string) {
		timestamp := time.Now().Format("15:04:05")
		fmt.Fprintf(logView, "[#565f89]%s[-] %s\n", timestamp, msg)
		logView.ScrollToEnd()
	}

	// --- Service Management ---
	var cancelWatcher context.CancelFunc
	var isRunning bool

	updateDashboard := func() {
		// This will be called to refresh the Dashboard content
	}

	startWatcher := func() {
		if isRunning {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancelWatcher = cancel
		isRunning = true

		watcherApp := &watcher.App{
			Config: cfg,
			Logger: logToUI,
		}

		go func() {
			if err := watcherApp.Start(ctx); err != nil {
				logToUI(fmt.Sprintf("[red]Watcher Error: %v[-]", err))
			}
			isRunning = false
			app.QueueUpdateDraw(func() {
				updateDashboard()
			})
		}()
		updateDashboard()
		logToUI("[#9ece6a]Service started manually from TUI[-]")
	}

	stopWatcher := func() {
		if !isRunning {
			return
		}
		if cancelWatcher != nil {
			cancelWatcher()
		}
		isRunning = false
		updateDashboard()
		logToUI("[#f7768e]Service stopped manually from TUI[-]")
	}

	// --- Page Constructors ---
	var populateMainPages func()

	createDashboard := func() *tview.Flex {
		statusText := "[#f7768e]STOPPED[-]"
		if isRunning {
			statusText = "[#9ece6a]RUNNING[-]"
		}

		info := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText(fmt.Sprintf("\n[white::b]Service Status:[-] %s\n\n[#a9b1d6]Manage the background sorting process from here.[-]", statusText))

		buttons := tview.NewFlex().
			AddItem(tview.NewBox(), 0, 1, false).
			AddItem(tview.NewButton("Start Service").SetSelectedFunc(startWatcher), 20, 1, !isRunning).
			AddItem(tview.NewBox(), 2, 0, false).
			AddItem(tview.NewButton("Stop Service").SetSelectedFunc(stopWatcher), 20, 1, isRunning).
			AddItem(tview.NewBox(), 0, 1, false)

		flex := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox(), 1, 0, false).
			AddItem(info, 5, 1, false).
			AddItem(buttons, 1, 1, true).
			AddItem(tview.NewBox(), 2, 0, false).
			AddItem(logView, 0, 2, false)

		flex.SetBorder(true).SetTitle(" [white::b]Control Dashboard[-:-:-] ")
		return flex
	}

	updateDashboard = func() {
		mainPages.AddPage("Dashboard", createDashboard(), true, true)
	}

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
		list.AddItem("Add New Directory", "Press 'a' or Enter to add", 'a', func() { 
			mainPages.SwitchToPage("AddDirForm") 
			app.SetFocus(mainPages)
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

	createAddDirForm := func() *tview.Form {
		form := tview.NewForm()
		form.AddInputField("Directory Path", "", 40, nil, nil).
			AddButton("Save", func() {
				path := form.GetFormItemByLabel("Directory Path").(*tview.InputField).GetText()
				if path != "" {
					cfg.WatchDirs = append(cfg.WatchDirs, path)
					form.GetFormItemByLabel("Directory Path").(*tview.InputField).SetText("")
				}
				populateMainPages()
				mainPages.SwitchToPage("DirsList")
				app.SetFocus(mainPages)
			}).
			AddButton("Cancel", func() {
				form.GetFormItemByLabel("Directory Path").(*tview.InputField).SetText("")
				mainPages.SwitchToPage("DirsList")
				app.SetFocus(mainPages)
			})
		form.SetBorder(true).SetTitle(" [white::b]Add Directory[-:-:-] ").SetTitleAlign(tview.AlignLeft)
		form.SetButtonBackgroundColor(bgPanel).SetButtonTextColor(cyanColor)
		form.SetFieldBackgroundColor(bgPanel).SetFieldTextColor(fgPrimary)
		return form
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
		form.SetBorder(true).SetTitle(" [white::b]Add Exclude Keyword[-:-:-] ").SetTitleAlign(tview.AlignLeft)
		form.SetButtonBackgroundColor(bgPanel).SetButtonTextColor(cyanColor)
		form.SetFieldBackgroundColor(bgPanel).SetFieldTextColor(fgPrimary)
		return form
	}

	createAddRuleForm := func() *tview.Form {
		form := tview.NewForm()
		form.AddInputField("Extensions (comma-separated)", "", 40, nil, nil).
			AddInputField("Target Directory", "", 40, nil, nil).
			AddButton("Save", func() {
				exts := form.GetFormItemByLabel("Extensions (comma-separated)").(*tview.InputField).GetText()
				target := form.GetFormItemByLabel("Target Directory").(*tview.InputField).GetText()
				if exts != "" && target != "" {
					extList := strings.Split(exts, ",")
					for i := range extList {
						extList[i] = strings.ToLower(strings.TrimSpace(extList[i]))
					}
					cfg.Rules = append(cfg.Rules, config.Rule{Extensions: extList, Target: target})
					form.GetFormItemByLabel("Extensions (comma-separated)").(*tview.InputField).SetText("")
					form.GetFormItemByLabel("Target Directory").(*tview.InputField).SetText("")
				}
				populateMainPages()
				mainPages.SwitchToPage("RulesList")
				app.SetFocus(mainPages)
			}).
			AddButton("Cancel", func() {
				form.GetFormItemByLabel("Extensions (comma-separated)").(*tview.InputField).SetText("")
				form.GetFormItemByLabel("Target Directory").(*tview.InputField).SetText("")
				mainPages.SwitchToPage("RulesList")
				app.SetFocus(mainPages)
			})
		form.SetBorder(true).SetTitle(" [white::b]Add Rule[-:-:-] ").SetTitleAlign(tview.AlignLeft)
		form.SetButtonBackgroundColor(bgPanel).SetButtonTextColor(cyanColor)
		form.SetFieldBackgroundColor(bgPanel).SetFieldTextColor(fgPrimary)
		return form
	}

	mainPages.AddPage("AddDirForm", createAddDirForm(), true, false)
	mainPages.AddPage("AddExcForm", createAddExcForm(), true, false)
	mainPages.AddPage("AddRuleForm", createAddRuleForm(), true, false)
	populateMainPages()

	sidebar.AddItem("Dashboard", "Control sorting service", 'd', func() {
		mainPages.SwitchToPage("Dashboard")
		app.SetFocus(mainPages)
	}).
		AddItem("Watch Directories", "Monitor incoming files", '1', func() {
			mainPages.SwitchToPage("DirsList")
			app.SetFocus(mainPages)
		}).
		AddItem("Exclude Keywords", "Skip files containing words", '2', func() {
			mainPages.SwitchToPage("ExcList")
			app.SetFocus(mainPages)
		}).
		AddItem("Routing Rules", "Map file extensions to folders", '3', func() {
			mainPages.SwitchToPage("RulesList")
			app.SetFocus(mainPages)
		}).
		AddItem("Save Config", "Write to config.yaml", 's', func() {
			if err := cfg.Save(config.GetConfigPath()); err != nil {
				logToUI(fmt.Sprintf("[red]Save Error: %v[-]", err))
			} else {
				logToUI("[#9ece6a]Configuration saved successfully![-]")
			}
		}).
		AddItem("Quit", "Stop service and exit", 'q', func() {
			stopWatcher()
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
		SetText("  [#bb9af7::b]⚡ InotiTidy Management Console[-:-:-] [#565f89]v1.1[-:-:-]")

	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[#a9b1d6]Navigate: [#7dcfff]\u2191\u2193[#a9b1d6] \u2022 Focus: [#7dcfff]Enter/Tab[#a9b1d6] \u2022 Back: [#7dcfff]Esc[#a9b1d6] \u2022 Exit: [#7dcfff]q/Ctrl+C[-:-:-]")

	contentFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox(), 1, 0, false). // left margin
		AddItem(sidebar, 35, 1, true).
		AddItem(tview.NewBox(), 2, 0, false). // gap
		AddItem(mainPages, 0, 2, false).
		AddItem(tview.NewBox(), 1, 0, false) // right margin

	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 1, 0, false). // top margin
		AddItem(header, 1, 1, false).
		AddItem(tview.NewBox(), 1, 0, false). // gap
		AddItem(contentFlex, 0, 1, true).
		AddItem(tview.NewBox(), 1, 0, false). // gap
		AddItem(footer, 1, 1, false).
		AddItem(tview.NewBox(), 1, 0, false) // bottom margin

	mainFlex.SetBackgroundColor(bgBody)

	// Start by default?
	startWatcher()

	app.SetRoot(mainFlex, true).EnableMouse(true)
	return app.Run()
}
