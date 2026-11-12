package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/auditdemo"
	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/tui"
)

// demoAuditFile writes the synthetic audit log of 30 days up to now.
func demoAuditFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.db")
	end := time.Now()
	clk := clock.NewFake(end.AddDate(0, 0, -31))
	s, err := sqlite.Open(path, sqlite.WithClock(clk))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := auditdemo.Generate(context.Background(), s, clk, auditdemo.Options{End: end, Days: 30, Location: time.Local}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAuditStats(t *testing.T) {
	// F13, G12 (proposal): "a38 audit stats" prints the figures of the TUI.
	path := demoAuditFile(t)
	code, out, errOut := runCLI("audit", "stats", "--file", path, "--range", "all", "--json")
	if code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, errOut)
	}
	var st auditstats.Stats
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatalf("JSON: %v\n%s", err, out)
	}
	if st.Popups == 0 || st.Denied == 0 || st.Deviations == 0 || st.Revocations != 1 || st.LockBegins != 1 || len(st.TopSessions) != 4 {
		t.Errorf("figures: %+v", st)
	}
	code, out, _ = runCLI("audit", "stats", "--file", path, "--range", "30d", "--project", "demo-projekt", "--top", "1")
	if code != 0 || !strings.Contains(out, "Popups:") || !strings.Contains(out, "Project:                demo-projekt") ||
		strings.Contains(out, "beispielprojekt") {
		t.Errorf("text output (code %d):\n%s", code, out)
	}
	tests := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"audit", "stats", "--file", path, "--range", "1y"}, 2, "unknown time range"},
		{[]string{"audit", "stats", "--file", path, "--top", "-1"}, 2, "--top must not be negative"},
		{[]string{"audit", "stats", "--file", path, "extra"}, 2, "unexpected argument"},
		{[]string{"audit", "stats", "--file", filepath.Join(t.TempDir(), "missing.db")}, 1, "cannot read the audit file"},
	}
	for _, tc := range tests {
		code, _, errOut := runCLI(tc.args...)
		if code != tc.code || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: code %d, stderr %q", tc.args, code, errOut)
		}
	}
}

func TestAuditStatsIsReadOnly(t *testing.T) {
	// F13: the figures come from a read-only file.
	path := demoAuditFile(t)
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := runCLI("audit", "stats", "--file", path); code != 0 {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("the audit file changed")
	}
}

// renderAll drives the TUI over the audit file and returns the plain text
// of each view.
func renderAll(t *testing.T, path string) []string {
	t.Helper()
	r := auditReader{path: path}
	m := tui.New(tui.Options{
		Source:   tui.Source{Name: displayPath(path), Load: r.Load, Verify: r.Verify},
		Clock:    clock.System{},
		Location: time.Local,
		Range:    auditstats.RangeAll,
	})
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 200, Height: 80})
	var runCmd func(tea.Cmd)
	runCmd = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				runCmd(c)
			}
			return
		}
		model, _ = model.Update(msg)
	}
	runCmd(model.Init())
	var screens []string
	for _, keys := range [][]string{nil, {"2"}, {"enter"}, {"esc", "3"}, {"enter"}} {
		for _, k := range keys {
			msg := tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
			switch k {
			case "enter":
				msg = tea.KeyPressMsg{Code: tea.KeyEnter}
			case "esc":
				msg = tea.KeyPressMsg{Code: tea.KeyEscape}
			}
			model, _ = model.Update(msg)
		}
		screens = append(screens, ansi.Strip(model.View().Content))
	}
	return screens
}

func TestTUIShowsNoValue(t *testing.T) {
	// D34, F13: a real daemon run with the fake backend delivers a value
	// with a marker. No screen of the TUI over the audit log holds the
	// marker; the screens show the reference.
	const marker = "A38-TUI-WERT-MARKER"
	cfg, socketDir, fakeFile := writeConfig(t)
	fake := strings.ReplaceAll(fakeBackend, "beispiel-wert-serve", marker)
	if err := os.WriteFile(fakeFile, []byte(fake), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	log := &syncBuffer{}
	done := make(chan int, 1)
	go func() {
		done <- serve(ctx, []string{"--config", cfg, "--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve"}, log, log)
	}()
	stopped := false
	stop := func() {
		if !stopped {
			stopped = true
			cancel()
			<-done
		}
	}
	t.Cleanup(stop)
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(log.String(), "daemon started") {
		if time.Now().After(deadline) {
			t.Fatalf("the daemon did not start; log:\n%s", log.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	rctx, rcancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer rcancel()
	agent, err := client.DialAgent(rctx, filepath.Join(socketDir, "agent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Hello(rctx, "beispiel-koordinator", "coding-agent"); err != nil {
		t.Fatal(err)
	}
	ref, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	for i := 0; i < 2; i++ {
		creds, err := agent.RequestCredentials(rctx, &protocol.RequestCredentials{
			References: []protocol.Reference{ref}, Task: "Run the tests", Reason: "The tests need the login.",
		})
		if err != nil || len(creds.Entries) != 1 || creds.Entries[0].Value == nil || creds.Entries[0].Value.Reveal() != marker {
			t.Fatalf("request %d: %+v, %v", i, creds, err)
		}
	}
	_ = agent.Close()
	stop()

	path := filepath.Join(filepath.Dir(cfg), "audit.db")
	screens := renderAll(t, path)
	for i, s := range screens {
		if strings.Contains(s, marker) {
			t.Errorf("screen %d holds the value", i)
		}
	}
	if !strings.Contains(screens[1], "delivery") || !strings.Contains(screens[2], ref.String()) || !strings.Contains(screens[4], "TIMELINE") {
		t.Errorf("the screens do not show the run:\n%s", strings.Join(screens, "\n----\n"))
	}
	code, out, _ := runCLI("audit", "stats", "--file", path, "--json")
	if code != 0 || strings.Contains(out, marker) || !strings.Contains(out, `"deliveries_under_grant": 1`) || !strings.Contains(out, `"deliveries_after_popup": 1`) {
		t.Errorf("stats (code %d):\n%s", code, out)
	}
}
