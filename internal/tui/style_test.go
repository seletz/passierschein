package tui

import (
	"image/color"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/seletz/passierschein/internal/auditstats"
)

// lines returns the lines of the screen as plain text.
func lines(m *Model) []string {
	return strings.Split(screen(m), "\n")
}

func TestChrome(t *testing.T) {
	// The frame of each screen: the header with the program name and the
	// tabs, a rule, the body, a rule and the footer with the key hints.
	entries := demo(t)
	tests := []struct {
		name   string
		keys   []string
		tabs   string
		footer []string
	}{
		{"overview", nil, "[1 overview]", []string{"t range", "f filter", "1/2/3 view", "? help", "q quit"}},
		{"events", []string{"2"}, "[2 events]", []string{"enter detail", "/ search", "1/2/3 view", "? help", "q quit"}},
		{"sessions", []string{"3"}, "[3 sessions]", []string{"enter timeline", "1/2/3 view", "? help"}},
		{"detail", []string{"2", "enter"}, "[2 events] › entry 1040", []string{"esc back", "q quit"}},
		{"timeline", []string{"3", "enter"}, "[3 sessions] › session demo-review", []string{"esc back"}},
		{"help", []string{"?"}, "[1 overview] › help", []string{"esc back", "q quit"}},
		{"search input", []string{"2", "/"}, "[2 events]", []string{"enter apply", "esc cancel", "^c quit"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(entries, intact(entries), 100, 30, auditstats.Range7Days)
			press(m, tc.keys...)
			ls := lines(m)
			if !strings.HasPrefix(ls[0], " a38 tui") || !strings.Contains(ls[0], tc.tabs) {
				t.Errorf("header %q: want the name and %q", ls[0], tc.tabs)
			}
			for _, i := range []int{1, len(ls) - 2} {
				if ls[i] != strings.Repeat("─", 100) {
					t.Errorf("line %d is not a rule: %q", i+1, ls[i])
				}
			}
			footer := ls[len(ls)-1]
			for _, h := range tc.footer {
				if !strings.Contains(footer, h) {
					t.Errorf("footer %q: no %q", footer, h)
				}
			}
			if !strings.Contains(footer, " • ") {
				t.Errorf("footer %q: no separator", footer)
			}
		})
	}
}

func TestChromeNarrow(t *testing.T) {
	// The footer never runs past the width; a hint that does not fit
	// gives way to an ellipsis.
	entries := demo(t)
	m := newModel(entries, intact(entries), 40, 12, auditstats.Range7Days)
	press(m, "2")
	s := screen(m)
	checkSize(t, s, 40, 12)
	ls := strings.Split(s, "\n")
	if !strings.Contains(ls[len(ls)-1], "…") {
		t.Errorf("footer %q: no ellipsis", ls[len(ls)-1])
	}
}

func TestSelectionMark(t *testing.T) {
	// The selected row has the mark ▸, so it reads without colour too.
	entries := demo(t)
	for _, v := range []string{"2", "3"} {
		m := newModel(entries, intact(entries), 100, 30, auditstats.Range7Days)
		press(m, v, "j")
		marked := 0
		for _, l := range lines(m) {
			if strings.HasPrefix(l, "▸") {
				marked++
			}
		}
		if marked != 1 {
			t.Errorf("view %s: %d marked rows, want 1", v, marked)
		}
	}
}

func TestHelpPanel(t *testing.T) {
	// "?" shows a panel with a rounded border over the body; "?" and esc
	// close it.
	entries := demo(t)
	m := newModel(entries, intact(entries), 100, 40, auditstats.Range7Days)
	press(m, "2", "?")
	s := screen(m)
	for _, want := range []string{"╭", "╰", "keys: events", "esc or ? closes this help", "R", "reload"} {
		if !strings.Contains(s, want) {
			t.Errorf("help: no %q in\n%s", want, s)
		}
	}
	press(m, "?")
	if strings.Contains(screen(m), "╭") {
		t.Error("? does not close the help")
	}
}

func TestReloadKey(t *testing.T) {
	// R reads the audit log again, as in the other TUIs of the owner.
	entries := demo(t)
	m := newModel(entries, intact(entries), 80, 24, auditstats.Range7Days)
	if _, cmd := m.Update(keyMsg("R")); cmd == nil {
		t.Error("R: no command")
	}
	// Tab does not switch the view: the views switch by number only.
	press(m, "tab")
	if m.view != viewOverview {
		t.Errorf("tab switched to view %d", m.view)
	}
}

// rgb returns the parameters of an SGR sequence for a 24-bit colour.
func rgb(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return strings.Join([]string{strconv.Itoa(int(r >> 8)), strconv.Itoa(int(g >> 8)), strconv.Itoa(int(b >> 8))}, ";")
}

func TestThemeFollowsBackground(t *testing.T) {
	// The view starts with the dark theme and switches when the terminal
	// names a light background; the active tab is on the accent colour.
	entries := demo(t)
	m := newModel(entries, intact(entries), 80, 24, auditstats.Range7Days)
	dark := newTheme(true)
	if !strings.Contains(m.View().Content, "48;2;"+rgb(dark.Accent)) {
		t.Errorf("dark: the active tab is not on the accent %s", rgb(dark.Accent))
	}
	m.Update(tea.BackgroundColorMsg{Color: lipgloss.Color("#ffffff")})
	light := newTheme(false)
	if m.th.Dark {
		t.Fatal("the theme stays dark on a light background")
	}
	if rgb(light.Accent) == rgb(dark.Accent) {
		t.Fatal("the light accent is the same as the dark one")
	}
	if !strings.Contains(m.View().Content, "48;2;"+rgb(light.Accent)) {
		t.Errorf("light: the active tab is not on the accent %s", rgb(light.Accent))
	}
}

func TestThemeRoles(t *testing.T) {
	// The roles of the colours: green for a delivery, red for a denial,
	// orange for a timeout, each also with its word.
	for _, dark := range []bool{true, false} {
		th := newTheme(dark)
		seen := map[string]bool{}
		for _, c := range []color.Color{th.Accent, th.Good, th.Bad, th.Caution} {
			if seen[rgb(c)] {
				t.Errorf("dark=%v: two roles share the colour %s", dark, rgb(c))
			}
			seen[rgb(c)] = true
		}
	}
}

func TestHelpBorderColour(t *testing.T) {
	// The border of the help is in the accent colour; the text in it
	// keeps its own colour.
	th := newTheme(true)
	box := th.Border.Border(lipgloss.RoundedBorder()).Render("text")
	if !strings.Contains(box, "38;2;"+rgb(th.Accent)+"m╭") {
		t.Errorf("the border is not in the accent colour: %q", box)
	}
	if strings.Contains(box, "38;2;"+rgb(th.Accent)+"mtext") {
		t.Errorf("the text in the border takes the accent colour: %q", box)
	}
}
