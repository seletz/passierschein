package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/clock"
)

func TestAccountStates(t *testing.T) {
	// T49: the overview names the state of each 1Password account from
	// the status of the daemon. A locked account does not hide the state
	// of a healthy account (D78).
	entries := demo(t)
	states := []AccountState{
		{Name: "privat", State: "ok", Configured: 2, Visible: 2},
		{Name: "firma", State: "locked", Reason: "1Password rejects the token\x1b[31m of the account", Configured: 1},
	}
	calls := 0
	m := New(Options{
		Source: Source{Name: "audit.db", Accounts: func(context.Context) ([]AccountState, error) { calls++; return states, nil }},
		Clock:  clock.NewFake(now), Location: time.UTC, Range: auditstats.Range30Days,
	})
	m.Update(loadedMsg{entries: entries})
	m.Update(verifiedMsg{v: intact(entries)})
	cmd := m.accounts()
	if cmd == nil {
		t.Fatal("no command for the accounts")
	}
	m.Update(cmd())
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	s := screen(m)
	for _, w := range []string{"1PASSWORD", "privat ok (2 of 2 vaults)", "firma LOCKED", "1Password rejects the token"} {
		if !strings.Contains(s, w) {
			t.Errorf("no %q in\n%s", w, s)
		}
	}
	if strings.Contains(s, "\x1b[31m of") {
		t.Error("the reason reaches the terminal with a control sequence")
	}

	t.Run("the daemon does not answer", func(t *testing.T) {
		m.Update(accountsMsg{err: errors.New("dial unix /tmp/a38-test/admin.sock: connect: no such file or directory")})
		if s := screen(m); !strings.Contains(s, "1PASSWORD") || !strings.Contains(s, "daemon does not answer") {
			t.Errorf("screen:\n%s", s)
		}
	})

	t.Run("without the source no line", func(t *testing.T) {
		m := newModel(entries, intact(entries), 100, 40, auditstats.Range30Days)
		if m.accounts() != nil {
			t.Error("a command without the source")
		}
		if s := screen(m); strings.Contains(s, "1PASSWORD") {
			t.Errorf("screen:\n%s", s)
		}
	})
}
