package main

import (
	"InotiTidy/internal/config"
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

func handleTUI() error {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}

	app := tview.NewApplication()
	pages := tview.NewPages()

	var populateMainMenu func()

	// --- Menu Constructors ---

	createMainMenu := func() *tview.List {
		list := tview.NewList().
			AddItem("Watch Directories", "Manage directories monitored by InotiTidy", '1', func() { pages.SwitchToPage("DirsList") }).
			AddItem("Exclude Keywords", "Manage ignored keywords in filenames", '2', func() { pages.SwitchToPage("ExcList") }).
			AddItem("Routing Rules", "Manage sorting rules based on extensions", '3', func() { pages.SwitchToPage("RulesList") }).
			AddItem("Save & Exit", "Save configuration and close the setup", 's', func() {
				if err := cfg.Save(config.GetConfigPath()); err != nil {
					app.Stop()
					fmt.Printf("Failed to save config: %v\n", err)
				} else {
					app.Stop()
					fmt.Println("Configuration saved successfully. Systemd service restart recommended.")
				}
			}).
			AddItem("Quit", "Discard changes and exit", 'q', func() { app.Stop() })

		list.SetBorder(true).SetTitle(" InotiTidy Setup Menu ")
		return list
	}

	createDirsList := func() *tview.List {
		list := tview.NewList()
		for i, dir := range cfg.WatchDirs {
			idx := i
			list.AddItem(dir, "Remove this directory", rune(49+i), func() {
				cfg.WatchDirs = append(cfg.WatchDirs[:idx], cfg.WatchDirs[idx+1:]...)
				populateMainMenu()
				pages.SwitchToPage("DirsList")
			})
		}
		list.AddItem("Add New Directory", "", 'a', func() { pages.SwitchToPage("AddDirForm") })
		list.AddItem("Back", "Return to main menu", 'b', func() { pages.SwitchToPage("Menu") })
		list.SetBorder(true).SetTitle(" Watch Directories ")
		return list
	}

	createExcList := func() *tview.List {
		list := tview.NewList()
		for i, exc := range cfg.Excludes {
			idx := i
			list.AddItem(exc, "Remove this keyword", rune(49+i), func() {
				cfg.Excludes = append(cfg.Excludes[:idx], cfg.Excludes[idx+1:]...)
				populateMainMenu()
				pages.SwitchToPage("ExcList")
			})
		}
		list.AddItem("Add New Keyword", "", 'a', func() { pages.SwitchToPage("AddExcForm") })
		list.AddItem("Back", "Return to main menu", 'b', func() { pages.SwitchToPage("Menu") })
		list.SetBorder(true).SetTitle(" Exclude Keywords ")
		return list
	}

	createRulesList := func() *tview.List {
		list := tview.NewList()
		for i, rule := range cfg.Rules {
			idx := i
			exts := strings.Join(rule.Extensions, ", ")
			list.AddItem(fmt.Sprintf("%s -> %s", exts, rule.Target), "Remove this rule", rune(49+i), func() {
				cfg.Rules = append(cfg.Rules[:idx], cfg.Rules[idx+1:]...)
				populateMainMenu()
				pages.SwitchToPage("RulesList")
			})
		}
		list.AddItem("Add New Rule", "", 'a', func() { pages.SwitchToPage("AddRuleForm") })
		list.AddItem("Back", "Return to main menu", 'b', func() { pages.SwitchToPage("Menu") })
		list.SetBorder(true).SetTitle(" Routing Rules ")
		return list
	}

	// --- Forms Constructors ---

	createAddDirForm := func() *tview.Form {
		form := tview.NewForm()
		form.AddInputField("Directory Path", "", 40, nil, nil).
			AddButton("Save", func() {
				path := form.GetFormItemByLabel("Directory Path").(*tview.InputField).GetText()
				if path != "" {
					cfg.WatchDirs = append(cfg.WatchDirs, path)
				}
				populateMainMenu()
				pages.SwitchToPage("DirsList")
			}).
			AddButton("Cancel", func() { pages.SwitchToPage("DirsList") })
		form.SetBorder(true).SetTitle(" Add Directory ").SetTitleAlign(tview.AlignLeft)
		return form
	}

	createAddExcForm := func() *tview.Form {
		form := tview.NewForm()
		form.AddInputField("Keyword", "", 40, nil, nil).
			AddButton("Save", func() {
				kw := form.GetFormItemByLabel("Keyword").(*tview.InputField).GetText()
				if kw != "" {
					cfg.Excludes = append(cfg.Excludes, kw)
				}
				populateMainMenu()
				pages.SwitchToPage("ExcList")
			}).
			AddButton("Cancel", func() { pages.SwitchToPage("ExcList") })
		form.SetBorder(true).SetTitle(" Add Exclude Keyword ").SetTitleAlign(tview.AlignLeft)
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
				}
				populateMainMenu()
				pages.SwitchToPage("RulesList")
			}).
			AddButton("Cancel", func() { pages.SwitchToPage("RulesList") })
		form.SetBorder(true).SetTitle(" Add Rule ").SetTitleAlign(tview.AlignLeft)
		return form
	}

	// Dynamic population logic to re-render pages with fresh config state
	populateMainMenu = func() {
		pages.AddPage("Menu", createMainMenu(), true, true)
		pages.AddPage("DirsList", createDirsList(), true, false)
		pages.AddPage("ExcList", createExcList(), true, false)
		pages.AddPage("RulesList", createRulesList(), true, false)
		pages.AddPage("AddDirForm", createAddDirForm(), true, false)
		pages.AddPage("AddExcForm", createAddExcForm(), true, false)
		pages.AddPage("AddRuleForm", createAddRuleForm(), true, false)
	}

	populateMainMenu()

	app.SetRoot(pages, true).EnableMouse(true)
	return app.Run()
}
