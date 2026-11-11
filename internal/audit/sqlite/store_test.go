package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

var t0 = time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

func refDB() protocol.Reference {
	return protocol.Reference{Vault: "beispielprojekt", Item: "datenbank-dev", Field: "passwort"}
}

func refApp() protocol.Reference {
	return protocol.Reference{Vault: "beispielprojekt", Item: "beispiel.app.k8s-dev", Field: "token"}
}

func event(typ audit.EventType, project string, refs ...protocol.Reference) audit.Event {
	return audit.Event{
		Type: typ,
		Verified: session.Facts{
			SessionID: "s-1", Origin: session.OriginLocal, Project: project,
			PeerPID: 4242, PeerUID: 501, Process: "agent", Cwd: "/work/" + project,
		},
		Claims:     session.Claims{SessionName: "beispiel-koordinator", Agent: "claude", Task: "deploy", Reason: "test"},
		References: refs,
		Result:     audit.Result{RequestID: "r-1", GrantID: "g-1", LifetimeSeconds: 600, Detail: "ok"},
	}
}

func openTemp(t *testing.T) (*Store, string, *clock.Fake) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.db")
	clk := clock.NewFake(t0)
	s, err := Open(path, WithClock(clk))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path, clk
}

func fill(t *testing.T, s *Store, clk *clock.Fake, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		clk.Advance(time.Second)
		ev := event(audit.EventRequest, "beispielprojekt", refDB())
		ev.Result.Detail = fmt.Sprintf("entry %d", i)
		if _, err := s.Append(context.Background(), ev); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
}

// rawExec changes the file behind the back of the store, like an attacker.
func rawExec(t *testing.T, path string, stmts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, st := range stmts {
		if _, err := db.Exec(st); err != nil {
			t.Fatalf("%s: %v", st, err)
		}
	}
}

// T7: verify finds a changed, deleted and swapped entry and names the first
// wrong entry. Removing the last entry is the documented limit of D37.
func TestVerifyTamper(t *testing.T) {
	tests := []struct {
		name        string
		tamper      []string
		wantOK      bool
		wantBad     uint64
		wantEntries uint64
	}{
		{"unchanged", nil, true, 0, 10},
		{"one field changed", []string{`UPDATE entries SET detail = 'other' WHERE seq = 4`}, false, 4, 3},
		{"time changed", []string{`UPDATE entries SET time_ns = time_ns + 1 WHERE seq = 7`}, false, 7, 6},
		{"reference changed", []string{`UPDATE entries SET refs = '[]' WHERE seq = 2`}, false, 2, 1},
		{"hash changed", []string{`UPDATE entries SET hash = '` + strings.Repeat("a", 64) + `' WHERE seq = 5`}, false, 5, 4},
		{"entry changed and hash repaired is found at the next entry", nil, false, 0, 0}, // set below
		{"first entry deleted", []string{`DELETE FROM entries WHERE seq = 1`}, false, 1, 0},
		{"one entry deleted", []string{`DELETE FROM entries WHERE seq = 4`}, false, 4, 3},
		{"two entries swapped", []string{
			`UPDATE entries SET seq = 100 WHERE seq = 4`,
			`UPDATE entries SET seq = 4 WHERE seq = 5`,
			`UPDATE entries SET seq = 5 WHERE seq = 100`,
		}, false, 4, 3},
		// Known limit (D37): nothing in the chain shows that the end is gone.
		{"last entry removed", []string{`DELETE FROM entries WHERE seq = 10`}, true, 0, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, path, clk := openTemp(t)
			fill(t, s, clk, 10)
			if strings.HasPrefix(tt.name, "entry changed and hash repaired") {
				// The attacker recalculates the hash of the changed entry only.
				entries, err := s.List(context.Background(), audit.Filter{})
				if err != nil {
					t.Fatal(err)
				}
				e := entries[5] // seq 6
				e.Result.Detail = "other"
				e.Hash = entryHash(e)
				tt.tamper = []string{fmt.Sprintf(`UPDATE entries SET detail = 'other', hash = '%s' WHERE seq = 6`, e.Hash)}
				tt.wantBad, tt.wantEntries = 7, 6
			}
			rawExec(t, path, tt.tamper...)
			got, err := s.Verify(context.Background())
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if got.OK != tt.wantOK || got.FirstBadSeq != tt.wantBad || got.Entries != tt.wantEntries {
				t.Errorf("Verify = %+v, want OK=%v FirstBadSeq=%d Entries=%d", got, tt.wantOK, tt.wantBad, tt.wantEntries)
			}
			if !tt.wantOK && got.Finding == "" {
				t.Error("a broken chain needs a finding")
			}
		})
	}
}

func TestVerifyEmpty(t *testing.T) {
	s, _, _ := openTemp(t)
	got, err := s.Verify(context.Background())
	if err != nil || !got.OK || got.Entries != 0 {
		t.Errorf("Verify = %+v, %v; want OK with no entries", got, err)
	}
}

func TestAppendChain(t *testing.T) {
	s, _, clk := openTemp(t)
	ctx := context.Background()
	var prev audit.Entry
	for i := 1; i <= 3; i++ {
		clk.Advance(time.Minute)
		e, err := s.Append(ctx, event(audit.EventGrant, "beispielprojekt", refDB(), refApp()))
		if err != nil {
			t.Fatal(err)
		}
		if e.Seq != uint64(i) {
			t.Errorf("Seq = %d, want %d", e.Seq, i)
		}
		wantPrev := genesisHash
		if i > 1 {
			wantPrev = prev.Hash
		}
		if e.PrevHash != wantPrev {
			t.Errorf("entry %d PrevHash = %s, want %s", i, e.PrevHash, wantPrev)
		}
		if e.Hash != entryHash(e) || len(e.Hash) != 64 {
			t.Errorf("entry %d has a wrong hash", i)
		}
		if !e.Time.Equal(clk.Now()) || e.Time.Location() != time.UTC {
			t.Errorf("entry %d Time = %v, want %v in UTC", i, e.Time, clk.Now())
		}
		prev = e
	}
	// The listed entry equals the returned entry.
	got, err := s.List(ctx, audit.Filter{})
	if err != nil || len(got) != 3 {
		t.Fatalf("List: %d entries, %v", len(got), err)
	}
	if !reflect.DeepEqual(got[2], prev) {
		t.Errorf("listed entry differs from returned entry:\n got %+v\nwant %+v", got[2], prev)
	}
}

// The canonical form is part of the file format. This test pins it.
func TestCanonicalGolden(t *testing.T) {
	e := audit.Entry{
		Seq: 1, Time: t0, Event: event(audit.EventDelivery, "beispielprojekt", refDB()),
		PrevHash: genesisHash,
	}
	const want = "6bd79feb35a8758ebc8f534d26f537671695ed0212f9b6668dbb5f0a7824198e"
	if got := entryHash(e); got != want {
		t.Errorf("hash = %s, want %s", got, want)
	}
}

// The canonical form separates the fields: moving text between two fields
// changes the hash.
func TestCanonicalUnambiguous(t *testing.T) {
	a := audit.Entry{Seq: 1, Time: t0, Event: audit.Event{Type: audit.EventRequest}, PrevHash: genesisHash}
	b := a
	a.Claims.Agent, a.Claims.Task = "ab", "c"
	b.Claims.Agent, b.Claims.Task = "a", "bc"
	if entryHash(a) == entryHash(b) {
		t.Error("different fields give the same hash")
	}
	c, d := a, a
	c.References = []protocol.Reference{{Vault: "a", Item: "b", Field: "c"}}
	d.References = []protocol.Reference{{Vault: "a", Item: "b", Field: "c"}, {Vault: "d", Item: "e", Field: "f"}}
	if entryHash(c) == entryHash(d) {
		t.Error("different references give the same hash")
	}
}

func TestListFilter(t *testing.T) {
	s, _, clk := openTemp(t)
	ctx := context.Background()
	add := func(ev audit.Event) {
		clk.Advance(time.Hour)
		if _, err := s.Append(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	add(event(audit.EventRequest, "beispielprojekt", refDB())) // 1
	add(event(audit.EventGrant, "beispielprojekt", refDB()))   // 2
	denial := event(audit.EventDenial, "anderes-projekt", refApp())
	denial.Result.Code = protocol.CodeDenied
	denial.Claims.SessionName = "beispiel-zweite"
	add(denial)                                                           // 3
	add(event(audit.EventError, ""))                                      // 4
	add(event(audit.EventDelivery, "beispielprojekt", refApp(), refDB())) // 5

	dbRef := refDB()
	tests := []struct {
		name string
		f    audit.Filter
		want []uint64
	}{
		{"all", audit.Filter{}, []uint64{1, 2, 3, 4, 5}},
		{"project", audit.Filter{Project: "beispielprojekt"}, []uint64{1, 2, 5}},
		{"session name", audit.Filter{SessionName: "beispiel-zweite"}, []uint64{3}},
		{"reference", audit.Filter{Reference: &dbRef}, []uint64{1, 2, 5}},
		{"types", audit.Filter{Types: []audit.EventType{audit.EventGrant, audit.EventDenial}}, []uint64{2, 3}},
		{"code", audit.Filter{Code: protocol.CodeDenied}, []uint64{3}},
		{"since", audit.Filter{Since: t0.Add(4 * time.Hour)}, []uint64{4, 5}},
		{"until is exclusive", audit.Filter{Until: t0.Add(2 * time.Hour)}, []uint64{1}},
		{"limit gives the newest", audit.Filter{Limit: 2}, []uint64{4, 5}},
		{"combined", audit.Filter{Project: "beispielprojekt", Types: []audit.EventType{audit.EventRequest, audit.EventDelivery}, Limit: 1}, []uint64{5}},
		{"no match", audit.Filter{Project: "gibt-es-nicht"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.List(ctx, tt.f)
			if err != nil {
				t.Fatal(err)
			}
			var seqs []uint64
			for _, e := range got {
				seqs = append(seqs, e.Seq)
			}
			if !reflect.DeepEqual(seqs, tt.want) {
				t.Errorf("seqs = %v, want %v", seqs, tt.want)
			}
		})
	}
}

func TestConcurrentAppend(t *testing.T) {
	s, _, _ := openTemp(t)
	ctx := context.Background()
	const workers, each = 8, 10
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if _, err := s.Append(ctx, event(audit.EventRequest, "beispielprojekt", refDB())); err != nil {
					t.Errorf("Append: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	got, err := s.Verify(ctx)
	if err != nil || !got.OK || got.Entries != workers*each {
		t.Errorf("Verify = %+v, %v; want OK with %d entries", got, err, workers*each)
	}
}

func TestReopen(t *testing.T) {
	s, path, clk := openTemp(t)
	fill(t, s, clk, 3)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path, WithClock(clk))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	e, err := s2.Append(context.Background(), event(audit.EventGrant, "beispielprojekt", refDB()))
	if err != nil {
		t.Fatal(err)
	}
	if e.Seq != 4 {
		t.Errorf("Seq after reopen = %d, want 4", e.Seq)
	}
	got, err := s2.Verify(context.Background())
	if err != nil || !got.OK || got.Entries != 4 {
		t.Errorf("Verify = %+v, %v", got, err)
	}
}

func TestOpenReadOnly(t *testing.T) {
	s, path, clk := openTemp(t)
	fill(t, s, clk, 2)

	// The daemon keeps its store open; a second, read-only store reads.
	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ro.Close() }()
	if got, err := ro.List(context.Background(), audit.Filter{}); err != nil || len(got) != 2 {
		t.Errorf("List = %d entries, %v", len(got), err)
	}
	if got, err := ro.Verify(context.Background()); err != nil || !got.OK {
		t.Errorf("Verify = %+v, %v", got, err)
	}
	if _, err := ro.Append(context.Background(), event(audit.EventRequest, "beispielprojekt")); err == nil {
		t.Error("Append on a read-only store: want error")
	}
	if _, err := OpenReadOnly(filepath.Join(t.TempDir(), "missing.db")); err == nil {
		t.Error("OpenReadOnly of a missing file: want error")
	}
	other := filepath.Join(t.TempDir(), "other.db")
	rawExec(t, other, `CREATE TABLE x (a)`)
	if _, err := OpenReadOnly(other); err == nil {
		t.Error("OpenReadOnly of a foreign file: want error")
	}
}

func TestOpenCreatesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "a38", "audit.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != want {
			t.Errorf("%s mode = %o, want %o", p, st.Mode().Perm(), want)
		}
	}
}

// T26 (part): a write error is an error of Append, and nothing is written.
func TestAppendErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("closed store", func(t *testing.T) {
		s, _, _ := openTemp(t)
		_ = s.Close()
		if _, err := s.Append(ctx, event(audit.EventRequest, "beispielprojekt")); err == nil {
			t.Error("want error")
		}
	})
	t.Run("database refuses the write", func(t *testing.T) {
		s, path, clk := openTemp(t)
		fill(t, s, clk, 1)
		rawExec(t, path, `CREATE TRIGGER refuse BEFORE INSERT ON entries BEGIN SELECT RAISE(ABORT, 'disk full'); END`)
		if _, err := s.Append(ctx, event(audit.EventRequest, "beispielprojekt")); err == nil {
			t.Fatal("want error")
		}
		rawExec(t, path, `DROP TRIGGER refuse`)
		// The failed write did not use a number: the chain stays whole.
		e, err := s.Append(ctx, event(audit.EventRequest, "beispielprojekt"))
		if err != nil || e.Seq != 2 {
			t.Fatalf("Append after the fault: %+v, %v", e, err)
		}
		if got, _ := s.Verify(ctx); !got.OK {
			t.Errorf("Verify = %+v", got)
		}
	})
	t.Run("invalid event", func(t *testing.T) {
		s, _, _ := openTemp(t)
		bad := []audit.Event{
			{Type: "nonsense"},
			event(audit.EventRequest, "beispielprojekt", protocol.Reference{Vault: "a", Item: "", Field: "c"}),
		}
		for _, ev := range bad {
			if _, err := s.Append(ctx, ev); err == nil {
				t.Errorf("Append(%+v): want error", ev)
			}
		}
		if got, _ := s.List(ctx, audit.Filter{}); len(got) != 0 {
			t.Error("an invalid event was written")
		}
	})
}

// An entry never holds a secret value. The types give no place for it; this
// test fails if somebody adds one.
func TestNoSecretInEntry(t *testing.T) {
	secretType := reflect.TypeOf(protocol.Secret{})
	var walk func(path string, typ reflect.Type)
	walk = func(path string, typ reflect.Type) {
		if typ == secretType {
			t.Errorf("%s holds a protocol.Secret", path)
			return
		}
		switch typ.Kind() {
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				if strings.Contains(strings.ToLower(f.Name), "secret") || strings.Contains(strings.ToLower(f.Name), "value") {
					t.Errorf("%s.%s: the name suggests a secret value", path, f.Name)
				}
				if typ == reflect.TypeOf(time.Time{}) {
					continue
				}
				walk(path+"."+f.Name, f.Type)
			}
		case reflect.Slice, reflect.Array, reflect.Pointer:
			walk(path+"[]", typ.Elem())
		case reflect.Map, reflect.Interface, reflect.Func, reflect.Chan:
			t.Errorf("%s has the kind %s, which can hold anything", path, typ.Kind())
		}
	}
	walk("audit.Entry", reflect.TypeOf(audit.Entry{}))

	// A secret that lives next to the event does not reach the file.
	const marker = "geheimer-wert-nur-fuer-den-test"
	secret := protocol.NewSecret(marker)
	s, path, _ := openTemp(t)
	ev := event(audit.EventDelivery, "beispielprojekt", refDB())
	ev.Result.Detail = fmt.Sprint("delivered ", secret) // fmt redacts
	if _, err := s.Append(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(marker)) {
		t.Error("the file contains the secret value")
	}
}
