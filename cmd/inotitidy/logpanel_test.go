package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func screenText(s tcell.SimulationScreen) string {
	cells, w, h := s.GetContents()
	var b strings.Builder
	for y := range h {
		for x := range w {
			c := cells[y*w+x]
			if len(c.Runes) > 0 && c.Runes[0] != 0 {
				b.WriteRune(c.Runes[0])
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// TestLogPanelVisible guards the regression where the dashboard packed so many
// fixed-height rows that the flexible log panel was starved to zero height and
// never rendered. On a normal-size terminal the "Service Logs" panel must show.
func TestLogPanelVisible(t *testing.T) {
	u := buildUI()
	defer u.stopJournalStream()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(100, 24)
	u.app.SetScreen(screen)

	go u.app.Run()
	defer u.app.Stop()
	time.Sleep(700 * time.Millisecond)

	out := screenText(screen)
	if !strings.Contains(out, "Control Dashboard") {
		t.Fatalf("dashboard did not render:\n%s", out)
	}
	if !strings.Contains(out, "Service Logs") {
		t.Fatalf("log panel is not visible (starved height regression):\n%s", out)
	}
}
