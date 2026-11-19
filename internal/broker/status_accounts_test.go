package broker

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
)

// accountsBackend is a backend with more than one account. Its accounts
// have fixed states; Vaults must not be called for the status.
type accountsBackend struct {
	backend.Backend
	states []backend.AccountState
	limit  time.Duration
	vaults int
}

func (a *accountsBackend) Check(_ context.Context, limit time.Duration) []backend.AccountState {
	a.limit = limit
	return a.states
}

func (a *accountsBackend) Vaults(ctx context.Context) ([]string, error) {
	a.vaults++
	return a.Backend.Vaults(ctx)
}

var (
	accountOK       = backend.AccountState{Name: "privat", Configured: 2, Visible: 2}
	accountLocked   = backend.AccountState{Name: "firma", Configured: 1, Err: fmt.Errorf("1password backend: list vaults: the account %q has no token (no_token): %w", "firma", backend.ErrLocked)}
	accountHanging  = backend.AccountState{Name: "labor", Configured: 1, Err: fmt.Errorf("1password backend: list vaults: the account %q gave no answer within 3s (timeout): %w", "labor", backend.ErrUnavailable)}
	accountDeadline = backend.AccountState{Name: "labor", Configured: 1, Err: context.DeadlineExceeded}
)

func TestStatusEachAccount(t *testing.T) {
	// T49: the status names the state of each 1Password account. A locked
	// or hanging account does not change the state of a healthy account;
	// the backend is ok while one account answers (D78, G45).
	tests := []struct {
		name      string
		states    []backend.AccountState
		wantState string
		want      []protocol.AccountStatus
	}{
		{"locked beside healthy", []backend.AccountState{accountLocked, accountOK}, protocol.BackendOK, []protocol.AccountStatus{
			{Name: "firma", State: protocol.BackendLocked, VaultsConfigured: 1},
			{Name: "privat", State: protocol.BackendOK, VaultsConfigured: 2, VaultsVisible: 2},
		}},
		{"hanging, locked and healthy", []backend.AccountState{accountHanging, accountLocked, accountOK}, protocol.BackendOK, []protocol.AccountStatus{
			{Name: "labor", State: protocol.BackendUnreachable, VaultsConfigured: 1},
			{Name: "firma", State: protocol.BackendLocked, VaultsConfigured: 1},
			{Name: "privat", State: protocol.BackendOK, VaultsConfigured: 2, VaultsVisible: 2},
		}},
		{"no account answers, one is locked", []backend.AccountState{accountHanging, accountLocked}, protocol.BackendLocked, []protocol.AccountStatus{
			{Name: "labor", State: protocol.BackendUnreachable, VaultsConfigured: 1},
			{Name: "firma", State: protocol.BackendLocked, VaultsConfigured: 1},
		}},
		{"no account answers in time", []backend.AccountState{accountDeadline}, protocol.BackendUnreachable, []protocol.AccountStatus{
			{Name: "labor", State: protocol.BackendUnreachable, VaultsConfigured: 1},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			be := &accountsBackend{Backend: h.be, states: tt.states}
			h.b = New(h.cfg, be, h.rec, h.store, h.clk, slog.New(slog.DiscardHandler), WithBackendKind(backend.KindOnePassword))
			st, e := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
			if e != nil || st.Backend == nil {
				t.Fatalf("status = %+v, %v", st, e)
			}
			if st.Backend.State != tt.wantState {
				t.Errorf("state = %q, want %q", st.Backend.State, tt.wantState)
			}
			if (tt.wantState == protocol.BackendOK) != (st.Backend.Reason == "") {
				t.Errorf("reason = %q", st.Backend.Reason)
			}
			if be.vaults != 0 {
				t.Errorf("the status called Vaults %d times; it asks each account with its own time limit", be.vaults)
			}
			if be.limit != accountStatusLimit {
				t.Errorf("limit = %s, want %s", be.limit, accountStatusLimit)
			}
			got := st.Backend.Accounts
			for i := range got {
				if (got[i].State == protocol.BackendOK) != (got[i].Reason == "") {
					t.Errorf("account %s: reason %q for state %s", got[i].Name, got[i].Reason, got[i].State)
				}
				got[i].Reason = ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accounts = %+v, want %+v", got, tt.want)
			}
			if err := st.Validate(); err != nil {
				t.Errorf("status is not valid: %v", err)
			}
		})
	}

	t.Run("the reason names the finding", func(t *testing.T) {
		h := newHarness(t)
		be := &accountsBackend{Backend: h.be, states: []backend.AccountState{accountHanging, accountLocked, accountDeadline}}
		h.b = New(h.cfg, be, h.rec, h.store, h.clk, slog.New(slog.DiscardHandler), WithBackendKind(backend.KindOnePassword))
		st, _ := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
		for i, want := range []string{"gave no answer within 3s", "has no token", "no answer within"} {
			if r := st.Backend.Accounts[i].Reason; !strings.Contains(r, want) {
				t.Errorf("account %d: reason %q lacks %q", i, r, want)
			}
		}
	})

	t.Run("a backend with one account has no list", func(t *testing.T) {
		h := newHarness(t)
		st, _ := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
		if st.Backend == nil || st.Backend.Accounts != nil {
			t.Errorf("backend = %+v, want no accounts", st.Backend)
		}
	})
}
