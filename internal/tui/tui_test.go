package tui

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/auditdemo"
	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

var update = flag.Bool("update", false, "write the golden files")

// now is the fixed time of the tests.
var now = time.Date(2026, 11, 12, 17, 30, 0, 0, time.UTC)

var (
	demoOnce    sync.Once
	demoEntries []audit.Entry
	demoErr     error
)

// demo returns the entries of the synthetic 30-day log. The log is written
// once through the real store.
func demo(t *testing.T) []audit.Entry {
	t.Helper()
	demoOnce.Do(func() {
		dir, err := os.MkdirTemp("", "a38-tui-")
		if err != nil {
			demoErr = err
			return
		}
		defer func() { _ = os.RemoveAll(dir) }()
		clk := clock.NewFake(now.AddDate(0, 0, -31))
		s, err := sqlite.Open(filepath.Join(dir, "audit.db"), sqlite.WithClock(clk))
		if err != nil {
			demoErr = err
			return
		}
		defer func() { _ = s.Close() }()
		if demoErr = auditdemo.Generate(context.Background(), s, clk, auditdemo.Options{End: now, Days: 30}); demoErr != nil {
			return
		}
		demoEntries, demoErr = s.List(context.Background(), audit.Filter{})
	})
	if demoErr != nil {
		t.Fatal(demoErr)
	}
	return demoEntries
}

// newModel returns a model with the entries, the check of the chain and
// the size of the screen, without commands.
func newModel(entries []audit.Entry, v audit.Verification, w, h int, rng auditstats.Range) *Model {
	m := New(Options{
		Source:   Source{Name: "~/.local/state/a38/audit.db"},
		Clock:    clock.NewFake(now),
		Location: time.UTC,
		Range:    rng,
	})
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.Update(loadedMsg{entries: entries})
	m.Update(verifiedMsg{v: v})
	return m
}

func intact(entries []audit.Entry) audit.Verification {
	return audit.Verification{OK: true, Entries: uint64(len(entries))}
}

// keyMsg returns the message of a key: a name such as "enter" or a text.
func keyMsg(s string) tea.KeyPressMsg {
	special := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "backspace": tea.KeyBackspace,
		"up": tea.KeyUp, "down": tea.KeyDown, "pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
	}
	if s == "shift+tab" {
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	}
	if c, ok := special[s]; ok {
		return tea.KeyPressMsg{Code: c}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

// press sends the keys; a word in the list is typed as text.
func press(m *Model, keys ...string) {
	for _, k := range keys {
		if len([]rune(k)) > 1 && keyMsg(k).Text != "" {
			for _, r := range k {
				m.Update(keyMsg(string(r)))
			}
			continue
		}
		m.Update(keyMsg(k))
	}
}

// screen returns the view as plain text.
func screen(m *Model) string {
	return ansi.Strip(m.View().Content)
}

// golden compares the screen with the file testdata/<name>.golden.
func golden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/tui -update)", err)
	}
	if string(want) != got {
		t.Errorf("screen %s differs from the golden file:\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}

// checkSize checks that the screen has exactly h lines of at most w cells.
func checkSize(t *testing.T, s string, w, h int) {
	t.Helper()
	lines := strings.Split(s, "\n")
	if len(lines) != h {
		t.Errorf("%d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if n := ansi.StringWidth(l); n > w {
			t.Errorf("line %d has %d cells, more than %d: %q", i+1, n, w, l)
		}
	}
}

func TestScreens(t *testing.T) {
	// F13, UC6: each view and the important states at a fixed size.
	entries := demo(t)
	broken := audit.Verification{OK: false, Entries: 56, FirstBadSeq: 57, Finding: "the hash does not match the content"}
	tests := []struct {
		name    string
		entries []audit.Entry
		v       audit.Verification
		w, h    int
		rng     auditstats.Range
		keys    []string
	}{
		{"overview-30d-80x24", entries, intact(entries), 80, 24, auditstats.Range30Days, nil},
		{"overview-30d-120x50", entries, intact(entries), 120, 50, auditstats.Range30Days, nil},
		{"overview-7d-scrolled", entries, intact(entries), 80, 24, auditstats.Range7Days, []string{"pgdown"}},
		{"overview-project-filter", entries, intact(entries), 100, 40, auditstats.Range30Days, []string{"f", "project=demo-projekt", "enter"}},
		{"events-80x24", entries, intact(entries), 80, 24, auditstats.Range7Days, []string{"2"}},
		{"events-120x40", entries, intact(entries), 120, 40, auditstats.Range7Days, []string{"2", "j", "j"}},
		{"events-filter", entries, intact(entries), 100, 30, auditstats.Range30Days, []string{"2", "f", "type=denial,expiry code=denied", "enter"}},
		{"events-search", entries, intact(entries), 100, 30, auditstats.Range30Days, []string{"2", "/", "fault", "enter"}},
		{"filter-input", entries, intact(entries), 80, 24, auditstats.Range7Days, []string{"2", "f", "session=beispiel-tests"}},
		{"detail", entries, intact(entries), 100, 40, auditstats.Range7Days, []string{"2", "j", "j", "enter"}},
		{"detail-80x24", entries, intact(entries), 80, 24, auditstats.Range7Days, []string{"2", "enter"}},
		{"sessions", entries, intact(entries), 100, 30, auditstats.Range7Days, []string{"3"}},
		{"timeline", entries, intact(entries), 100, 40, auditstats.Range7Days, []string{"3", "j", "enter"}},
		{"help", entries, intact(entries), 80, 24, auditstats.Range7Days, []string{"?"}},
		{"help-events-100x40", entries, intact(entries), 100, 40, auditstats.Range7Days, []string{"2", "?"}},
		{"chain-broken", entries, broken, 80, 24, auditstats.Range30Days, nil},
		{"chain-broken-detail", entries, broken, 100, 40, auditstats.RangeAll, []string{"2", "seq=57", "enter"}},
		{"empty-log", nil, audit.Verification{OK: true}, 80, 24, auditstats.Range7Days, nil},
		{"empty-log-events", nil, audit.Verification{OK: true}, 80, 24, auditstats.Range7Days, []string{"2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(tc.entries, tc.v, tc.w, tc.h, tc.rng)
			for _, k := range tc.keys {
				if seq, ok := strings.CutPrefix(k, "seq="); ok {
					// Move the cursor to the entry with the number.
					for i, e := range m.events {
						if strconv.FormatUint(e.Seq, 10) == seq {
							m.evCursor = i
						}
					}
					continue
				}
				press(m, k)
			}
			s := screen(m)
			checkSize(t, s, tc.w, tc.h)
			golden(t, tc.name, s)
		})
	}
}

func TestStartAndErrors(t *testing.T) {
	m := New(Options{Source: Source{Name: "audit.db"}, Clock: clock.NewFake(now), Location: time.UTC})
	if s := screen(m); !strings.Contains(s, "Reading the audit log") || !strings.Contains(s, "chain: checking") {
		t.Errorf("before the load:\n%s", s)
	}
	m.Update(loadedMsg{err: os.ErrPermission})
	if s := screen(m); !strings.Contains(s, "Cannot read the audit log") || !strings.Contains(s, "permission denied") {
		t.Errorf("after a failed load:\n%s", s)
	}
	m.Update(verifiedMsg{err: os.ErrPermission})
	if s := screen(m); !strings.Contains(s, "CHECK FAILED") {
		t.Errorf("after a failed check:\n%s", s)
	}
}

func TestKeys(t *testing.T) {
	entries := demo(t)
	m := newModel(entries, intact(entries), 80, 24, auditstats.Range7Days)
	tests := []struct {
		keys []string
		want string
	}{
		{[]string{"2"}, "EVENTS"},
		{[]string{"3"}, "SESSIONS"},
		{[]string{"1"}, "POPUPS PER DAY"},
		{[]string{"t"}, "Range 30 days"},
		{[]string{"t"}, "Range all"},
		{[]string{"t"}, "Range today"},
		{[]string{"t"}, "Range 7 days"},
		{[]string{"f", "nonsense", "enter"}, "Filter not changed"},
		{[]string{"f", "type=unknown", "enter"}, "unknown event type"},
		{[]string{"f", "project=demo-projekt", "enter"}, "filter: project=demo-projekt"},
		{[]string{"c"}, "Filter and search are cleared"},
		{[]string{"/", "x", "esc"}, "EVENTS"},
		{[]string{"?"}, "keys: events"},
		{[]string{"esc"}, "EVENTS"},
		{[]string{"3", "enter"}, "TIMELINE"},
		{[]string{"esc", "1"}, "POPUPS PER DAY"},
	}
	for _, tc := range tests {
		press(m, tc.keys...)
		if s := screen(m); !strings.Contains(s, tc.want) {
			t.Errorf("after %v: no %q in\n%s", tc.keys, tc.want, s)
		}
	}
	// "/" opens the search in the events; esc keeps the old search.
	if m.search != "" {
		t.Errorf("search = %q after esc", m.search)
	}
	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q: no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q does not quit")
	}
	// In the input line, q is text.
	press(m, "/", "q")
	if m.inputText != "q" {
		t.Errorf("input = %q", m.inputText)
	}
}

func TestRefreshCommands(t *testing.T) {
	// The view reads the audit log on "r" and on the timer, and never
	// writes: the source has no write function.
	var loads, verifies int
	clk := clock.NewFake(now)
	m := New(Options{
		Source: Source{
			Name: "audit.db",
			Load: func(context.Context) ([]audit.Entry, error) { loads++; return nil, nil },
			Verify: func(context.Context) (audit.Verification, error) {
				verifies++
				return audit.Verification{OK: true}, nil
			},
		},
		Clock: clk, Location: time.UTC, Refresh: time.Minute,
	})
	run := func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			for _, c := range msg {
				if c == nil {
					continue
				}
				done := make(chan tea.Msg, 1)
				go func() { done <- c() }()
				select {
				case got := <-done:
					m.Update(got)
				case <-time.After(50 * time.Millisecond):
					// The timer waits for the fake clock.
					go func() { <-done }()
				}
			}
		default:
			m.Update(msg)
		}
	}
	run(m.Init())
	if loads != 1 || verifies != 1 {
		t.Fatalf("after Init: %d loads, %d verifies", loads, verifies)
	}
	_, cmd := m.Update(keyMsg("r"))
	run(cmd)
	if loads != 2 || verifies != 2 {
		t.Errorf("after r: %d loads, %d verifies", loads, verifies)
	}
	// The timer of the refresh.
	tick := m.tick()
	done := make(chan tea.Msg, 1)
	go func() { done <- tick() }()
	// Two timers wait: the one of Init and this one.
	if err := clk.BlockUntil(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	_, cmd = m.Update(<-done)
	run(cmd)
	if loads != 3 {
		t.Errorf("after the timer: %d loads", loads)
	}
}

func TestUntrustedText(t *testing.T) {
	// Each string from the audit log is untrusted: control and bidi
	// characters must not reach the terminal, and long values are cut.
	ref := protocol.Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: "password"}
	evil := "beispiel\x1b]52;c;ZXZpbA==\x07-\x1b[2J\u202ekoordinator\u2066"
	long := strings.Repeat("lang ", 400)
	e := audit.Entry{
		Seq: 1, Time: now.Add(-time.Hour),
		Event: audit.Event{
			Type:       audit.EventRequest,
			Verified:   session.Facts{SessionID: "s-1", Origin: session.OriginLocal, Project: "beispielprojekt\x1b[31m"},
			Claims:     session.Claims{SessionName: evil, Agent: evil, Task: long, Reason: evil + long},
			References: []protocol.Reference{ref},
			Result:     audit.Result{Detail: evil},
		},
		PrevHash: strings.Repeat("0", 64), Hash: strings.Repeat("a", 64),
	}
	entries := []audit.Entry{e}
	for _, keys := range [][]string{nil, {"2"}, {"2", "enter"}, {"3"}, {"3", "enter"}} {
		m := newModel(entries, intact(entries), 80, 24, auditstats.Range7Days)
		press(m, keys...)
		raw := m.View().Content
		// Only the styles of the view itself may hold ESC: strip them and
		// look for what is left.
		plain := ansi.Strip(raw)
		for _, bad := range []string{"\x1b", "\x07", "\u202e", "\u2066"} {
			if strings.Contains(plain, bad) {
				t.Errorf("keys %v: the screen holds %q", keys, bad)
			}
		}
		// The OSC 52 sequence of the claim must not survive as a sequence.
		if strings.Contains(raw, "\x1b]52") {
			t.Errorf("keys %v: the screen holds an OSC 52 sequence", keys)
		}
		checkSize(t, plain, 80, 24)
	}
}

func TestNoValueOnScreen(t *testing.T) {
	// D34, F13: the audit log has no field for a value, so the view
	// cannot show one. A marker in the backend of a real daemon run is
	// checked in cmd/a38 (TestTUIShowsNoValue). Here: no screen of the
	// demo log holds the word of a value field.
	entries := demo(t)
	for _, keys := range [][]string{nil, {"2"}, {"2", "enter"}, {"3"}, {"3", "enter"}, {"?"}} {
		m := newModel(entries, intact(entries), 120, 50, auditstats.RangeAll)
		press(m, keys...)
		if s := strings.ToLower(screen(m)); strings.Contains(s, "value:") || strings.Contains(s, "secret:") {
			t.Errorf("keys %v: the screen names a value", keys)
		}
	}
}
