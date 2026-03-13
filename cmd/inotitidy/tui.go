package main

import (
	"InotiTidy/internal/config"
	"fmt"
	"strings"

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

	welcome := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n\n\n\n[#7dcfff::b]Welcome to InotiTidy Configuration[-:-:-]\n\n[#a9b1d6]Select an option from the sidebar to begin.\n\nPress [#bb9af7]Enter[-:-:-] to focus the right pane.\nPress [#bb9af7]Esc[-:-:-] to return to the sidebar.")
	welcome.SetBorder(true).SetTitle(" [white::b]Dashboard[-:-:-] ")
	mainPages.AddPage("Welcome", welcome, true, true)

	sidebar.AddItem("Watch Directories", "Monitor incoming files", '1', func() {
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
		AddItem("Save & Exit", "Write to config.yaml and close", 's', func() {
			if err := cfg.Save(config.GetConfigPath()); err != nil {
				app.Stop()
				fmt.Printf("Failed to save config: %v\n", err)
			} else {
				app.Stop()
				fmt.Println("Configuration saved successfully. Don't forget to restart: systemctl --user restart inotitidy")
			}
		}).
		AddItem("Quit", "Discard unsaved changes", 'q', func() { app.Stop() })

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
		SetText("  [#bb9af7::b]⚡ InotiTidy[-:-:-] [#565f89]v1.0[-:-:-]")

	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[#a9b1d6]Navigate: [#7dcfff]\u2191\u2193/Arrows[#a9b1d6] \u2022 Focus: [#7dcfff]Enter/Tab[#a9b1d6] \u2022 Back: [#7dcfff]Esc[#a9b1d6] \u2022 Abort: [#7dcfff]Ctrl+C[-:-:-]")

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

	app.SetRoot(mainFlex, true).EnableMouse(true)
	return app.Run()
}
