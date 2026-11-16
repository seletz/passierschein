package grant

import (
	"maps"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

var t0 = time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

func r(field string) protocol.Reference {
	return protocol.Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: field}
}

func g(id string, sid session.ID, life time.Duration, fields ...string) Grant {
	var refs []protocol.Reference
	for _, f := range fields {
		refs = append(refs, r(f))
	}
	return Grant{ID: id, Facts: session.Facts{SessionID: sid}, Refs: refs, GrantedAt: t0, ExpiresAt: t0.Add(life)}
}

// T4, T11, T23, D8, D32: a grant covers exactly its credentials, for its
// own session, until the end of its lifetime.
func TestCover(t *testing.T) {
	s := NewStore()
	s.Add(g("g1", "s-a", time.Hour, "username"))
	s.Add(g("g2", "s-a", 2*time.Hour, "password"))
	tests := []struct {
		name  string
		sid   session.ID
		field string
		at    time.Duration
		want  string
	}{
		{"own session", "s-a", "username", 0, "g1"},
		{"other credential", "s-a", "token", 0, ""},
		{"other session", "s-b", "username", 0, ""},
		{"shortly before the end", "s-a", "username", time.Hour - time.Nanosecond, "g1"},
		{"at the end", "s-a", "username", time.Hour, ""},
		{"second grant", "s-a", "password", time.Hour, "g2"},
	}
	for _, tt := range tests {
		got, ok := s.Cover(tt.sid, r(tt.field), t0.Add(tt.at))
		if ok != (tt.want != "") || got.ID != tt.want {
			t.Errorf("%s: Cover = %q, %v; want %q", tt.name, got.ID, ok, tt.want)
		}
	}
}

// T16, T25, D33: a grant ends with its lifetime, its session and the
// revocation.
func TestEnds(t *testing.T) {
	s := NewStore()
	s.Add(g("g1", "s-a", time.Hour, "username"))
	s.Add(g("g2", "s-a", 3*time.Hour, "password"))
	s.Add(g("g3", "s-b", 3*time.Hour, "password"))
	s.Add(g("g4", "s-b", 3*time.Hour, "username"))

	if got := s.Live("s-a", t0); len(got) != 2 || got[0].ID != "g1" {
		t.Fatalf("Live = %+v", got)
	}
	if _, ok := s.Remove("g1", t0.Add(time.Hour)); ok {
		t.Error("Remove found a grant after its end")
	}
	exp := s.Expired(t0.Add(time.Hour))
	if len(exp) != 1 || exp[0].ID != "g1" {
		t.Fatalf("Expired = %+v", exp)
	}
	if len(s.Expired(t0.Add(time.Hour))) != 0 {
		t.Error("Expired returned a grant twice")
	}
	if got, ok := s.Remove("g3", t0); !ok || got.ID != "g3" {
		t.Fatalf("Remove = %+v, %v", got, ok)
	}
	if _, ok := s.Remove("g3", t0); ok {
		t.Error("Remove found a removed grant")
	}
	ended := s.RemoveSession("s-a", t0)
	if len(ended) != 1 || ended[0].ID != "g2" {
		t.Fatalf("RemoveSession = %+v", ended)
	}
	if all := s.All(t0); len(all) != 1 || all[0].ID != "g4" {
		t.Fatalf("All = %+v", all)
	}
}

// Add keeps its own copy of the references.
func TestAddCopies(t *testing.T) {
	s := NewStore()
	gr := g("g1", "s-a", time.Hour, "username")
	s.Add(gr)
	gr.Refs[0] = r("password")
	if _, ok := s.Cover("s-a", r("username"), t0); !ok {
		t.Error("a change of the caller changed the grant")
	}
}

// T36, D63: the binding decides which commands a grant covers. A grant of
// the MCP way and a grant with the binding none cover each command; a
// grant with the binding program covers the same program and variable
// names; a grant with the binding command covers only the same command.
func TestCoverCommand(t *testing.T) {
	psql := Command{Program: "/opt/homebrew/bin/psql", Args: []string{"-c", "select 1"}, Cwd: "/work/beispielprojekt",
		Env: map[string]protocol.Reference{"PGPASSWORD": r("password")}}
	other := func(mut func(*Command)) Command {
		c := psql
		c.Args = append([]string(nil), psql.Args...)
		c.Env = maps.Clone(psql.Env)
		mut(&c)
		return c
	}
	otherArgs := other(func(c *Command) { c.Args = []string{"-c", "select 2"} })
	otherProgram := other(func(c *Command) { c.Program = "/usr/bin/env" })
	otherCwd := other(func(c *Command) { c.Cwd = "/work/beispielprojekt/sub" })
	otherNames := other(func(c *Command) { c.Env = map[string]protocol.Reference{"DB_PASSWORD": r("password")} })
	noArgs := other(func(c *Command) { c.Args = nil })
	tests := []struct {
		name    string
		binding config.Binding
		cmd     Command
		want    bool
	}{
		{"none, same", config.BindingNone, psql, true},
		{"none, other program", config.BindingNone, otherProgram, true},
		{"none, other names", config.BindingNone, otherNames, true},
		{"program, same", config.BindingProgram, psql, true},
		{"program, other arguments", config.BindingProgram, otherArgs, true},
		{"program, other directory", config.BindingProgram, otherCwd, true},
		{"program, other program", config.BindingProgram, otherProgram, false},
		{"program, other names", config.BindingProgram, otherNames, false},
		{"command, same", config.BindingCommand, psql, true},
		{"command, other arguments", config.BindingCommand, otherArgs, false},
		{"command, no arguments", config.BindingCommand, noArgs, false},
		{"command, other directory", config.BindingCommand, otherCwd, false},
		{"command, other names", config.BindingCommand, otherNames, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			gr := g("g1", "s-a", time.Hour, "password")
			gr.Binding, gr.Command = tt.binding, psql
			s.Add(gr)
			if _, ok := s.CoverCommand("s-a", r("password"), tt.cmd, t0); ok != tt.want {
				t.Errorf("CoverCommand = %v, want %v", ok, tt.want)
			}
			// The MCP way uses only a grant with the binding none.
			if _, ok := s.Cover("s-a", r("password"), t0); ok != (tt.binding == config.BindingNone) {
				t.Errorf("Cover = %v for the binding %s", ok, tt.binding)
			}
		})
	}
	// A grant of the MCP way has no binding: it covers each command.
	s := NewStore()
	s.Add(g("g1", "s-a", time.Hour, "password"))
	if _, ok := s.CoverCommand("s-a", r("password"), psql, t0); !ok {
		t.Error("a grant of the MCP way does not cover a command in dev")
	}
}

// T36 (D63, review of pull request 115): the binding program or command
// covers only the approved mapping of each variable name to its
// reference. The binding none covers the references with each command
// and each mapping (D63: "mit jedem Weg und jedem Kommando").
func TestCoverCommandMapping(t *testing.T) {
	approved := map[string]protocol.Reference{"PGUSER": r("username"), "PGPASSWORD": r("password")}
	cmd := func(env map[string]protocol.Reference) Command {
		return Command{Program: "/opt/homebrew/bin/psql", Cwd: "/work/beispielprojekt", Env: env}
	}
	tests := []struct {
		name string
		env  map[string]protocol.Reference
		want map[config.Binding]bool
	}{
		{"same mapping", approved, map[config.Binding]bool{config.BindingNone: true, config.BindingProgram: true, config.BindingCommand: true}},
		{"same mapping, other order", map[string]protocol.Reference{"PGPASSWORD": r("password"), "PGUSER": r("username")},
			map[config.Binding]bool{config.BindingNone: true, config.BindingProgram: true, config.BindingCommand: true}},
		{"password under the user name", map[string]protocol.Reference{"PGUSER": r("password")},
			map[config.Binding]bool{config.BindingNone: true, config.BindingProgram: false, config.BindingCommand: false}},
		{"swapped names", map[string]protocol.Reference{"PGUSER": r("password"), "PGPASSWORD": r("username")},
			map[config.Binding]bool{config.BindingNone: true, config.BindingProgram: false, config.BindingCommand: false}},
		{"subset", map[string]protocol.Reference{"PGPASSWORD": r("password")},
			map[config.Binding]bool{config.BindingNone: true, config.BindingProgram: false, config.BindingCommand: false}},
		{"one reference under two names", map[string]protocol.Reference{"PGUSER": r("username"), "PGPASSWORD": r("password"), "PGPASS2": r("password")},
			map[config.Binding]bool{config.BindingNone: true, config.BindingProgram: false, config.BindingCommand: false}},
	}
	for _, tt := range tests {
		for _, b := range []config.Binding{config.BindingNone, config.BindingProgram, config.BindingCommand} {
			t.Run(tt.name+", "+string(b), func(t *testing.T) {
				s := NewStore()
				gr := g("g1", "s-a", time.Hour, "username", "password")
				gr.Binding, gr.Command = b, cmd(approved)
				s.Add(gr)
				ok := true
				for _, ref := range tt.env {
					if _, c := s.CoverCommand("s-a", ref, cmd(tt.env), t0); !c {
						ok = false
					}
				}
				if ok != tt.want[b] {
					t.Errorf("covered = %v, want %v", ok, tt.want[b])
				}
			})
		}
	}
}
