package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
)

// shortDir returns a short directory under /tmp.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// stateFor returns a state with a configuration for the directory.
func stateFor(dir string) *State {
	return &State{Config: &config.Config{Paths: config.Paths{
		SocketDir: filepath.Join(dir, "run"), AuditFile: filepath.Join(dir, "audit.db"),
	}}}
}

func TestSocketCheckWithRealFiles(t *testing.T) {
	// T27 (part), D43: the real modes of the directory and the sockets.
	tests := []struct {
		name    string
		prepare func(t *testing.T, run string)
		wantOK  bool
		want    string
	}{
		{"missing directory", func(*testing.T, string) {}, true, "does not exist yet"},
		{"right modes", func(t *testing.T, run string) {
			mustMkdir(t, run, 0o700)
		}, true, "0700"},
		{"directory mode 0755", func(t *testing.T, run string) {
			mustMkdir(t, run, 0o755)
		}, false, "mode 0755"},
		{"socket mode 0644", func(t *testing.T, run string) {
			mustMkdir(t, run, 0o700)
			if err := os.WriteFile(filepath.Join(run, "agent.sock"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, false, "mode 0644"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := shortDir(t)
			st := stateFor(dir)
			tt.prepare(t, st.Config.Paths.SocketDir)
			res := socketCheck{SystemEnv{}}.Run(context.Background(), st)
			if len(res) != 1 || res[0].OK != tt.wantOK {
				t.Fatalf("result = %+v", res)
			}
			if got := res[0].Detail + res[0].Cause; !strings.Contains(got, tt.want) {
				t.Errorf("text %q does not contain %q", got, tt.want)
			}
			if !tt.wantOK && res[0].NextStep == "" {
				t.Error("a finding has no next step")
			}
		})
	}
}

func mustMkdir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestAuditChecksWithRealFiles(t *testing.T) {
	// T27 (part): a missing audit file, a file that is no audit log and a
	// read-only file are findings; the free space check reads the disk.
	dir := shortDir(t)
	st := stateFor(dir)
	env := SystemEnv{}
	ctx := context.Background()

	if res := (auditFileCheck{env}).Run(ctx, st); len(res) != 1 || res[0].OK || !strings.Contains(res[0].Finding, "does not exist") {
		t.Errorf("missing file: %+v", res)
	}
	if err := os.WriteFile(st.Config.Paths.AuditFile, []byte("no database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if res := (auditFileCheck{env}).Run(ctx, st); len(res) != 1 || !res[0].OK {
		t.Errorf("existing file: %+v", res)
	}
	if res := (auditChainCheck{env}).Run(ctx, st); len(res) != 1 || res[0].OK {
		t.Errorf("file that is no audit log: %+v", res)
	}
	if os.Getuid() != 0 {
		if err := os.Chmod(st.Config.Paths.AuditFile, 0o400); err != nil {
			t.Fatal(err)
		}
		if res := (auditFileCheck{env}).Run(ctx, st); len(res) != 1 || res[0].OK || !strings.Contains(res[0].Finding, "not writable") {
			t.Errorf("read-only file: %+v", res)
		}
	}
	if res := (auditSpaceCheck{env}).Run(ctx, st); len(res) != 1 || res[0].Check != "audit space" {
		t.Errorf("space: %+v", res)
	}
}
