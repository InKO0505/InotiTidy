package main

import (
	"InotiTidy/internal/config"
	"testing"

	"github.com/rivo/tview"
)

func TestRuleDraftToRule(t *testing.T) {
	cases := []struct {
		name    string
		draft   ruleDraft
		wantErr bool
		check   func(config.Rule) bool
	}{
		{
			name:  "basic move",
			draft: ruleDraft{name: "Docs", exts: "pdf, .txt", action: "move", target: "/docs", onConflict: "rename"},
			check: func(r config.Rule) bool {
				return len(r.Extensions) == 2 && r.Extensions[0] == ".pdf" && r.Target == "/docs"
			},
		},
		{
			name:  "trash without target is ok",
			draft: ruleDraft{glob: "*.tmp", action: "trash"},
			check: func(r config.Rule) bool { return r.Action == "trash" && r.OnConflict == "rename" },
		},
		{
			name:  "advanced size parsed",
			draft: ruleDraft{exts: "mkv", action: "move", target: "/v", minSize: "100MB", olderThan: "24h"},
			check: func(r config.Rule) bool { return r.MinSize == 100*1024*1024 && r.OlderThan.Hours() == 24 },
		},
		{name: "no conditions", draft: ruleDraft{action: "move", target: "/x"}, wantErr: true},
		{name: "move without target", draft: ruleDraft{exts: "pdf", action: "move"}, wantErr: true},
		{name: "bad size", draft: ruleDraft{exts: "pdf", action: "move", target: "/x", minSize: "huge"}, wantErr: true},
		{name: "bad duration", draft: ruleDraft{exts: "pdf", action: "move", target: "/x", olderThan: "soon"}, wantErr: true},
	}
	for _, c := range cases {
		r, err := c.draft.toRule()
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
			continue
		}
		if err == nil && c.check != nil && !c.check(r) {
			t.Errorf("%s: rule check failed: %+v", c.name, r)
		}
	}
}

// TestModalOpen verifies the predicate that stops global Tab/Esc from stealing
// focus while a form is open (the reported Tab bug).
func TestModalOpen(t *testing.T) {
	u := &ui{rootPages: tview.NewPages(), mainPages: tview.NewPages()}
	u.rootPages.AddPage("Main", tview.NewBox(), true, true)
	u.mainPages.AddPage("Dashboard", tview.NewBox(), true, true)

	if u.modalOpen() {
		t.Fatal("no modal should be open on the dashboard")
	}

	u.mainPages.AddPage("RuleForm", tview.NewBox(), true, true)
	if !u.modalOpen() {
		t.Fatal("rule form must count as a modal so Tab stays inside it")
	}
	u.mainPages.RemovePage("RuleForm")
	if u.modalOpen() {
		t.Fatal("closing the form should release the modal state")
	}

	u.rootPages.AddPage("Confirm", tview.NewModal(), true, true)
	if !u.modalOpen() {
		t.Fatal("a confirm dialog must count as a modal")
	}
}
