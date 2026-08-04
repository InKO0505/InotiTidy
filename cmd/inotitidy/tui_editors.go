package main

import (
	"InotiTidy/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func listShortcut(i int) rune {
	if i < 9 {
		return rune('1' + i)
	}
	return 0
}

func styledList(title string) *tview.List {
	l := tview.NewList()
	l.SetBorder(true).SetTitle(" [white::b]" + title + "[-:-:-] ")
	l.SetSelectedBackgroundColor(bgPanel).SetSelectedTextColor(fgAccent)
	l.SetMainTextColor(fgPrimary).SetSecondaryTextColor(fgSecondary)
	return l
}

// deleteOn wires 'd'/Delete on a list to remove the currently selected item.
func deleteOn(list *tview.List, remove func(idx int)) {
	list.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyDelete || ev.Rune() == 'd' {
			remove(list.GetCurrentItem())
			return nil
		}
		return ev
	})
}

// rebuildEditors regenerates the list pages that reflect cfg contents.
func (u *ui) rebuildEditors() {
	for _, name := range []string{"DirsList", "ExcList", "RulesList"} {
		u.mainPages.RemovePage(name)
	}
	u.mainPages.AddPage("DirsList", u.createDirsList(), true, false)
	u.mainPages.AddPage("ExcList", u.createExcList(), true, false)
	u.mainPages.AddPage("RulesList", u.createRulesList(), true, false)
	u.refreshDashboard()
}

func (u *ui) createDirsList() *tview.List {
	list := styledList("Watch Directories")
	for i, w := range u.cfg.Watch {
		idx := i
		label := w.Path
		if w.Recursive {
			label += "  [#7dcfff](recursive)[-]"
		}
		list.AddItem(label, "Enter: toggle recursive · d: remove", listShortcut(i), func() {
			u.cfg.Watch[idx].Recursive = !u.cfg.Watch[idx].Recursive
			u.markDirty()
			u.rebuildEditors()
			u.setPage("DirsList")
		})
	}
	if len(u.cfg.Watch) == 0 {
		list.AddItem("[#565f89]No directories yet — press 'a' to add one[-]", "", 0, nil)
	}
	list.AddItem("[#9ece6a]+ Add directory[-]", "Browse and add a folder", 'a', func() {
		u.showDirPicker(func(path string) {
			u.cfg.Watch = append(u.cfg.Watch, config.WatchDir{Path: path})
			u.markDirty()
			u.rebuildEditors()
			u.setPage("DirsList")
		})
	})
	deleteOn(list, func(i int) {
		if i >= 0 && i < len(u.cfg.Watch) {
			u.cfg.Watch = append(u.cfg.Watch[:i], u.cfg.Watch[i+1:]...)
			u.markDirty()
			u.rebuildEditors()
			u.setPage("DirsList")
		}
	})
	return list
}

func (u *ui) createExcList() *tview.List {
	list := styledList("Exclude Keywords (global)")
	for i, kw := range u.cfg.GlobalExclude {
		list.AddItem(kw, "d: remove", listShortcut(i), nil)
	}
	if len(u.cfg.GlobalExclude) == 0 {
		list.AddItem("[#565f89]No keywords — press 'a' to add one[-]", "", 0, nil)
	}
	list.AddItem("[#9ece6a]+ Add keyword[-]", "Files containing it are skipped", 'a', func() {
		u.showTextForm("Add Exclude Keyword", "Keyword", "", func(v string) {
			if v = strings.TrimSpace(v); v != "" {
				u.cfg.GlobalExclude = append(u.cfg.GlobalExclude, v)
				u.markDirty()
			}
			u.rebuildEditors()
			u.setPage("ExcList")
		})
	})
	deleteOn(list, func(i int) {
		if i >= 0 && i < len(u.cfg.GlobalExclude) {
			u.cfg.GlobalExclude = append(u.cfg.GlobalExclude[:i], u.cfg.GlobalExclude[i+1:]...)
			u.markDirty()
			u.rebuildEditors()
			u.setPage("ExcList")
		}
	})
	return list
}

func (u *ui) createRulesList() *tview.List {
	list := styledList("Routing Rules")
	for i, r := range u.cfg.Rules {
		idx := i
		list.AddItem(ruleSummary(r), "Enter: edit · d: remove", listShortcut(i), func() {
			u.showRuleForm(idx)
		})
	}
	if len(u.cfg.Rules) == 0 {
		list.AddItem("[#565f89]No rules — press 'a' to add one[-]", "", 0, nil)
	}
	list.AddItem("[#9ece6a]+ Add rule[-]", "Define a condition and action", 'a', func() {
		u.showRuleForm(-1)
	})
	deleteOn(list, func(i int) {
		if i >= 0 && i < len(u.cfg.Rules) {
			u.cfg.Rules = append(u.cfg.Rules[:i], u.cfg.Rules[i+1:]...)
			u.markDirty()
			u.rebuildEditors()
			u.setPage("RulesList")
		}
	})
	return list
}

func ruleSummary(r config.Rule) string {
	name := r.Name
	if name == "" {
		name = "(unnamed)"
	}
	cond := strings.Join(r.Extensions, ",")
	if r.NameRegex != "" {
		cond = "re:" + r.NameRegex
	} else if r.NameGlob != "" {
		cond = r.NameGlob
	}
	if cond == "" {
		cond = "*"
	}
	dest := r.Target
	if r.Action == config.ActionTrash {
		dest = "trash"
	}
	return fmt.Sprintf("[#c0caf5]%s[-]  [#565f89]%s[-] %s [#7dcfff]->[-] %s", name, r.Action, cond, dest)
}

// ---- forms -----------------------------------------------------------------

// modalFlex centers a primitive of the given size over the main pages.
func modalFlex(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

func (u *ui) showTextForm(title, label, initial string, onSave func(string)) {
	form := tview.NewForm()
	form.AddInputField(label, initial, 40, nil, nil)
	form.SetBorder(true).SetTitle(" [white::b]" + title + "[-:-:-] ")
	form.SetButtonBackgroundColor(bgPanel).SetButtonTextColor(cyanColor)
	form.SetFieldBackgroundColor(bgPanel).SetFieldTextColor(fgPrimary)

	close := func() { u.mainPages.RemovePage("Form"); u.app.SetFocus(u.mainPages) }
	form.AddButton("Save", func() {
		v := form.GetFormItemByLabel(label).(*tview.InputField).GetText()
		close()
		onSave(v)
	})
	form.AddButton("Cancel", close)

	u.mainPages.AddPage("Form", modalFlex(form, 60, 7), true, true)
	u.app.SetFocus(form)
}

func (u *ui) showRuleForm(idx int) {
	var r config.Rule
	title := "Add Rule"
	if idx >= 0 && idx < len(u.cfg.Rules) {
		r = u.cfg.Rules[idx]
		title = "Edit Rule"
	} else {
		r.Action = config.ActionMove
		r.OnConflict = config.ConflictRename
	}

	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" [white::b]" + title + "[-:-:-] ")
	form.SetButtonBackgroundColor(bgPanel).SetButtonTextColor(cyanColor)
	form.SetFieldBackgroundColor(bgPanel).SetFieldTextColor(fgPrimary)

	actions := []string{config.ActionMove, config.ActionCopy, config.ActionTrash}
	conflicts := []string{config.ConflictRename, config.ConflictSkip, config.ConflictOverwrite, config.ConflictTrash}

	form.AddInputField("Name", r.Name, 40, nil, nil)
	form.AddInputField("Extensions (comma)", strings.Join(r.Extensions, ", "), 40, nil, nil)
	form.AddInputField("Name glob", r.NameGlob, 40, nil, nil)
	form.AddInputField("Name regex", r.NameRegex, 40, nil, nil)
	form.AddInputField("Min size", sizeStr(r.MinSize), 20, nil, nil)
	form.AddInputField("Max size", sizeStr(r.MaxSize), 20, nil, nil)
	form.AddInputField("Older than", durStr(r.OlderThan), 20, nil, nil)
	form.AddInputField("Exclude (comma)", strings.Join(r.Exclude, ", "), 40, nil, nil)
	form.AddDropDown("Action", actions, indexOf(actions, r.Action), nil)
	form.AddInputField("Target", r.Target, 40, nil, nil)
	form.AddDropDown("On conflict", conflicts, indexOf(conflicts, r.OnConflict), nil)

	get := func(label string) string {
		return strings.TrimSpace(form.GetFormItemByLabel(label).(*tview.InputField).GetText())
	}
	dropdown := func(label string) string {
		_, opt := form.GetFormItemByLabel(label).(*tview.DropDown).GetCurrentOption()
		return opt
	}
	close := func() { u.mainPages.RemovePage("RuleForm"); u.app.SetFocus(u.mainPages) }

	form.AddButton("Browse target", func() {
		u.showDirPicker(func(path string) {
			form.GetFormItemByLabel("Target").(*tview.InputField).SetText(path)
			u.app.SetFocus(form)
		})
	})
	form.AddButton("Save", func() {
		nr, err := buildRule(get, dropdown)
		if err != nil {
			u.notify("Invalid rule: " + err.Error())
			return
		}
		if idx >= 0 && idx < len(u.cfg.Rules) {
			u.cfg.Rules[idx] = nr
		} else {
			u.cfg.Rules = append(u.cfg.Rules, nr)
		}
		u.markDirty()
		close()
		u.rebuildEditors()
		u.setPage("RulesList")
	})
	form.AddButton("Cancel", func() {
		close()
		u.setPage("RulesList")
	})

	u.mainPages.AddPage("RuleForm", modalFlex(form, 66, 26), true, true)
	u.app.SetFocus(form)
}

// buildRule assembles and validates a rule from form getters.
func buildRule(get func(string) string, dropdown func(string) string) (config.Rule, error) {
	r := config.Rule{
		Name:       get("Name"),
		Extensions: normExts(get("Extensions (comma)")),
		NameGlob:   get("Name glob"),
		NameRegex:  get("Name regex"),
		Exclude:    splitComma(get("Exclude (comma)")),
		Action:     dropdown("Action"),
		Target:     get("Target"),
		OnConflict: dropdown("On conflict"),
	}
	var err error
	if r.MinSize, err = config.ParseSize(get("Min size")); err != nil {
		return r, err
	}
	if r.MaxSize, err = config.ParseSize(get("Max size")); err != nil {
		return r, err
	}
	if s := get("Older than"); s != "" {
		d, derr := time.ParseDuration(s)
		if derr != nil {
			return r, fmt.Errorf("older than: %v", derr)
		}
		r.OlderThan = d
	}

	tmp := config.Config{Watch: []config.WatchDir{{Path: "/"}}, Rules: []config.Rule{r}}
	if verr := tmp.Validate(); verr != nil {
		return r, verr
	}
	return r, nil
}

// ---- directory picker ------------------------------------------------------

func (u *ui) showDirPicker(onSelect func(string)) {
	current, err := os.UserHomeDir()
	if err != nil || current == "" {
		current, _ = os.Getwd()
	}
	list := styledList("")

	var render func(string)
	render = func(path string) {
		list.Clear()
		current = path
		list.SetTitle(fmt.Sprintf(" [white::b]Select:[-] %s ", current))
		list.AddItem(".. [parent]", "", 0, func() { render(filepath.Dir(current)) })
		entries, _ := os.ReadDir(current)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				full := filepath.Join(current, e.Name())
				list.AddItem(e.Name(), "", 0, func() { render(full) })
			}
		}
		list.AddItem("[#9ece6a]✓ Select this directory[-]", "", 's', func() {
			u.mainPages.RemovePage("Picker")
			onSelect(current)
		})
		list.AddItem("[#f7768e]✗ Cancel[-]", "", 'q', func() {
			u.mainPages.RemovePage("Picker")
			u.app.SetFocus(u.mainPages)
		})
	}
	render(current)

	u.mainPages.AddPage("Picker", modalFlex(list, 70, 24), true, true)
	u.app.SetFocus(list)
}

// ---- preview / undo / save / confirm ---------------------------------------

func (u *ui) showPreview() {
	u.engine.SetConfig(u.cfg)
	plans := u.engine.PreviewAll()

	text := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	text.SetBorder(true).SetTitle(" [white::b]Preview (dry-run)[-:-:-] ")
	if len(plans) == 0 {
		text.SetText("\n  [#9ece6a]Nothing to sort — everything is already tidy.[-]")
	} else {
		var b strings.Builder
		fmt.Fprintf(&b, "\n  [white::b]%d file(s) would be processed:[-]\n\n", len(plans))
		for _, p := range plans {
			fmt.Fprintf(&b, "  [#7dcfff]%-7s[-] %s [#565f89]->[-] %s\n",
				p.Action, filepath.Base(p.Src), p.Dest)
		}
		text.SetText(b.String())
	}

	actions := styledList("Actions")
	actions.AddItem("Refresh", "Recompute the plan", 'r', func() { u.showPreview() })
	actions.AddItem("Apply now", "Execute these moves", 'a', func() {
		u.setPage("Dashboard")
		u.scanNow()
	})
	actions.AddItem("Undo last batch", "Reverse the last run", 'u', u.undoLast)
	actions.AddItem("Back", "Return to dashboard", 'b', func() { u.setPage("Dashboard") })

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(text, 0, 3, false).
		AddItem(actions, 6, 0, true)

	u.mainPages.AddPage("Preview", flex, true, true)
	u.mainPages.SwitchToPage("Preview")
	u.app.SetFocus(actions)
	u.setFooter("[#7dcfff]r[-] refresh  [#7dcfff]a[-] apply  [#7dcfff]u[-] undo  [#7dcfff]b[-] back")
}

func (u *ui) undoLast() {
	u.confirm("Undo the most recent batch of moves?", func() {
		go func() {
			n, err := u.engine.UndoLastBatch()
			if err != nil {
				u.logUI("["+redColor+"]Undo failed: %v[-]", err)
				return
			}
			u.logUI("["+greenColor+"]Undid %d operation(s)[-]", n)
			u.app.QueueUpdateDraw(u.refreshDashboard)
		}()
	})
}

func (u *ui) saveConfig() {
	if err := u.cfg.Save(config.GetConfigPath()); err != nil {
		u.notify("Error saving: " + err.Error())
		return
	}
	u.dirty = false
	u.refreshHeader()
	msg := "Configuration saved.\n\nThe running daemon reloads it automatically."
	u.notify(msg)
}

func (u *ui) confirm(text string, onYes func()) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Yes", "Cancel"}).
		SetDoneFunc(func(_ int, label string) {
			u.rootPages.RemovePage("Confirm")
			u.app.SetFocus(u.mainPages)
			if label == "Yes" {
				onYes()
			}
		})
	modal.SetBackgroundColor(bgPanel).SetTextColor(fgPrimary)
	u.rootPages.AddPage("Confirm", modal, true, true)
	u.app.SetFocus(modal)
}

func (u *ui) notify(text string) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) {
			u.rootPages.RemovePage("Notify")
			u.app.SetFocus(u.mainPages)
		})
	modal.SetBackgroundColor(bgPanel).SetTextColor(fgPrimary)
	u.rootPages.AddPage("Notify", modal, true, true)
	u.app.SetFocus(modal)
}

// ---- small helpers ---------------------------------------------------------

func normExts(csv string) []string {
	out := []string{}
	for p := range strings.SplitSeq(csv, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func splitComma(csv string) []string {
	out := []string{}
	for p := range strings.SplitSeq(csv, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func indexOf(opts []string, v string) int {
	for i, o := range opts {
		if o == v {
			return i
		}
	}
	return 0
}

func sizeStr(n int64) string {
	if n == 0 {
		return ""
	}
	return config.Size(n).String()
}

func durStr(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}
