package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// auditFixture writes n entries and returns the path of the file.
func auditFixture(t *testing.T, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ref := protocol.Reference{Vault: "beispielprojekt", Item: "datenbank-dev", Field: "passwort"}
	for i := 1; i <= n; i++ {
		typ, code := audit.EventGrant, protocol.Code("")
		if i%2 == 0 {
			typ, code = audit.EventDenial, protocol.CodeDenied
		}
		_, err := s.Append(context.Background(), audit.Event{
			Type:       typ,
			Verified:   session.Facts{SessionID: "s-1", Origin: session.OriginLocal, Project: "beispielprojekt"},
			Claims:     session.Claims{SessionName: "beispiel-koordinator"},
			References: []protocol.Reference{ref},
			Result:     audit.Result{Code: code, Detail: fmt.Sprintf("entry %d", i)},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func runCLI(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = run(args, &o, &e)
	return code, o.String(), e.String()
}

// UC8: verify exits 0 for an intact chain and non-zero with the number of
// the first wrong entry.
func TestAuditVerify(t *testing.T) {
	path := auditFixture(t, 5)
	code, out, errOut := runCLI("audit", "verify", "--file", path)
	if code != 0 || !strings.Contains(out, "5 entries") || errOut != "" {
		t.Errorf("intact: code %d, stdout %q, stderr %q", code, out, errOut)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE entries SET detail = 'changed' WHERE seq = 3`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	code, out, errOut = runCLI("audit", "verify", "--file", path)
	if code == 0 || !strings.Contains(errOut, "entry 3") || out != "" {
		t.Errorf("broken: code %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestAuditList(t *testing.T) {
	path := auditFixture(t, 4)
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantRows int // lines of stdout
		contains string
	}{
		{"table", []string{"audit", "list", "--file", path}, 0, 5, "beispiel-koordinator"},
		{"type filter", []string{"audit", "list", "--file", path, "--type", "denial"}, 0, 3, "denied"},
		{"code filter", []string{"audit", "list", "--file", path, "--code", "denied", "--limit", "1"}, 0, 2, "entry 4"},
		{"ref filter", []string{"audit", "list", "--file", path, "--ref", "op://beispielprojekt/datenbank-dev/passwort"}, 0, 5, "op://beispielprojekt/datenbank-dev/passwort"},
		{"ref filter no match", []string{"audit", "list", "--file", path, "--ref", "op://a/b/c"}, 0, 1, "SEQ"},
		{"since duration", []string{"audit", "list", "--file", path, "--since", "1h"}, 0, 5, ""},
		{"until in the past", []string{"audit", "list", "--file", path, "--until", "2000-01-01T00:00:00Z"}, 0, 1, ""},
		{"bad type", []string{"audit", "list", "--file", path, "--type", "nonsense"}, 2, 0, ""},
		{"bad ref", []string{"audit", "list", "--file", path, "--ref", "nonsense"}, 2, 0, ""},
		{"bad time", []string{"audit", "list", "--file", path, "--since", "gestern"}, 2, 0, ""},
		{"unknown flag", []string{"audit", "list", "--nope"}, 2, 0, ""},
		{"missing file", []string{"audit", "list", "--file", filepath.Join(t.TempDir(), "none.db")}, 1, 0, ""},
		{"unknown subcommand", []string{"audit", "show"}, 2, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := runCLI(tt.args...)
			if code != tt.wantCode {
				t.Fatalf("code = %d, want %d (stderr %q)", code, tt.wantCode, errOut)
			}
			if tt.wantCode != 0 {
				if errOut == "" {
					t.Error("want a message on stderr")
				}
				return
			}
			if got := strings.Count(out, "\n"); got != tt.wantRows {
				t.Errorf("lines = %d, want %d:\n%s", got, tt.wantRows, out)
			}
			if !strings.Contains(out, tt.contains) {
				t.Errorf("stdout lacks %q:\n%s", tt.contains, out)
			}
		})
	}
}

func TestAuditListJSON(t *testing.T) {
	path := auditFixture(t, 3)
	code, out, _ := runCLI("audit", "list", "--file", path, "--json", "--type", "grant")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	var e audit.Entry
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatal(err)
	}
	if e.Seq != 1 || e.Type != audit.EventGrant || len(e.References) != 1 || len(e.Hash) != 64 {
		t.Errorf("entry = %+v", e)
	}
}

// The file comes from paths.audit_file of the configuration.
func TestAuditConfig(t *testing.T) {
	path := auditFixture(t, 2)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	text := "version: 1\npaths:\n  audit_file: " + path + "\n"
	if err := os.WriteFile(cfg, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI("audit", "verify", "--config", cfg)
	if code != 0 || !strings.Contains(out, "2 entries") {
		t.Errorf("code %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = runCLI("audit", "verify", "--config", filepath.Join(t.TempDir(), "none.yaml"))
	if code != 1 || errOut == "" {
		t.Errorf("missing config: code %d, stderr %q", code, errOut)
	}
}

func TestParseTime(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	got, err := parseTime(now, "2h")
	if err != nil || !got.Equal(now.Add(-2*time.Hour)) {
		t.Errorf("2h = %v, %v", got, err)
	}
	if _, err := parseTime(now, "-2h"); err == nil {
		t.Error("negative duration: want error")
	}
}

func TestDashStripsControl(t *testing.T) {
	if got := dash("a\x1b[2Jb"); strings.ContainsRune(got, 0x1b) {
		t.Errorf("dash = %q", got)
	}
}
