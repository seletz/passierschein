package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkProgram creates an executable file with the mode and returns its path.
func mkProgram(t *testing.T, path string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveProgramThroughLinkIntoBundle(t *testing.T) {
	// D50: a38 in the PATH is a link into Passierschein.app. The plist
	// names the real path in the bundle.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inBundle := mkProgram(t, filepath.Join(root, "Applications", "Passierschein.app", "Contents", "MacOS", "a38"), 0o755)
	bin := filepath.Join(root, "home", ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bin, "a38")
	if err := os.Symlink(inBundle, link); err != nil {
		t.Fatal(err)
	}
	// a link to the link
	link2 := filepath.Join(root, "a38-link")
	if err := os.Symlink(link, link2); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{inBundle, link, link2} {
		got, err := ResolveProgram(from)
		if err != nil {
			t.Fatal(err)
		}
		if got != inBundle {
			t.Errorf("ResolveProgram(%s) = %s, want %s", from, got, inBundle)
		}
		if !InBundle(got) {
			t.Errorf("InBundle(%s) = false", got)
		}
	}
	if InBundle(link) {
		t.Error("the link in the PATH counts as a bundle")
	}
	if _, err := ResolveProgram(filepath.Join(root, "missing")); err == nil {
		t.Error("a missing program resolves")
	}
	if _, err := ResolveProgram(""); err == nil {
		t.Error("an empty path resolves")
	}
}

func TestCheckProgram(t *testing.T) {
	// D51: launchd starts the program at each login with the rights of
	// the user; another user must not be able to change it.
	uid := os.Getuid()
	tests := []struct {
		name    string
		prepare func(t *testing.T, dir string) string
		want    string
	}{
		{"good", func(t *testing.T, dir string) string { return mkProgram(t, filepath.Join(dir, "ok", "a38"), 0o755) }, ""},
		{"group writable", func(t *testing.T, dir string) string { return mkProgram(t, filepath.Join(dir, "g", "a38"), 0o775) }, "writable by group or others"},
		{"world writable", func(t *testing.T, dir string) string { return mkProgram(t, filepath.Join(dir, "w", "a38"), 0o757) }, "writable by group or others"},
		{"not executable", func(t *testing.T, dir string) string { return mkProgram(t, filepath.Join(dir, "x", "a38"), 0o644) }, "cannot execute"},
		{"directory world writable", func(t *testing.T, dir string) string {
			p := mkProgram(t, filepath.Join(dir, "d", "a38"), 0o755)
			if err := os.Chmod(filepath.Dir(p), 0o777); err != nil {
				t.Fatal(err)
			}
			return p
		}, "the directory"},
		{"directory", func(t *testing.T, dir string) string {
			p := filepath.Join(dir, "dir", "a38")
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			return p
		}, "not a regular file"},
		{"missing", func(_ *testing.T, dir string) string { return filepath.Join(dir, "none") }, "cannot be read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckProgram(tt.prepare(t, t.TempDir()), uid)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
	t.Run("other owner", func(t *testing.T) {
		if uid == 0 {
			t.Skip("root owns each file")
		}
		p := mkProgram(t, filepath.Join(t.TempDir(), "a38"), 0o755)
		if err := CheckProgram(p, uid+1); err == nil || !strings.Contains(err.Error(), "belongs to user") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProgramWarnings(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(tmp, "src")
	mkProgram(t, filepath.Join(checkout, "bin", "a38"), 0o755)
	if err := os.WriteFile(filepath.Join(checkout, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		path string
		want []string
	}{
		{"/Applications/Passierschein.app/Contents/MacOS/a38", nil},
		{"/tmp/a38", []string{"temporary directory"}},
		{"/private/var/folders/x/y/T/go-build123/b001/exe/a38", []string{"temporary directory", "build cache"}},
		{filepath.Join(checkout, "bin", "a38"), []string{"source directory"}},
	}
	for _, tt := range tests {
		got := ProgramWarnings(tt.path)
		joined := strings.Join(got, "\n")
		for _, w := range tt.want {
			if !strings.Contains(joined, w) {
				t.Errorf("warnings for %s = %q, want %q", tt.path, got, w)
			}
		}
		if len(tt.want) == 0 && len(got) != 0 {
			t.Errorf("warnings for %s = %q, want none", tt.path, got)
		}
	}
}
