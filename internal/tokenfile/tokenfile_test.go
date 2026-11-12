package tokenfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/token"
)

// marker is a fake token. No output of the package may contain it.
const marker = "ops_beispiel-marker-7f3a"

func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestReadAndCheck(t *testing.T) {
	// G3 (assumption of stage 3): the token file is a regular file of the
	// user with the mode 0600 exactly.
	tests := []struct {
		name    string
		prepare func(t *testing.T, path string)
		want    error
		text    string
	}{
		{"good file", func(t *testing.T, p string) { write(t, p, marker+"\n", 0o600) }, nil, ""},
		{"missing file", func(*testing.T, string) {}, ErrMissing, "does not exist"},
		{"mode 0644", func(t *testing.T, p string) { write(t, p, marker, 0o644) }, ErrMode, "0644"},
		{"mode 0400", func(t *testing.T, p string) { write(t, p, marker, 0o400) }, ErrMode, "0400"},
		{"mode 0640", func(t *testing.T, p string) { write(t, p, marker, 0o640) }, ErrMode, "0640"},
		{"directory", func(t *testing.T, p string) {
			if err := os.Mkdir(p, 0o700); err != nil {
				t.Fatal(err)
			}
		}, ErrNotRegular, "not a regular file"},
		{"symbolic link", func(t *testing.T, p string) {
			target := p + ".real"
			write(t, target, marker, 0o600)
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
		}, ErrSymlink, "symbolic link"},
		{"empty file", func(t *testing.T, p string) { write(t, p, " \n", 0o600) }, ErrEmpty, "empty"},
		{"too large", func(t *testing.T, p string) { write(t, p, strings.Repeat("x", MaxSize+1), 0o600) }, ErrTooLarge, "larger"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(shortDir(t), "onepassword-token")
			tt.prepare(t, path)
			tok, err := Read(path)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Read error = %v, want %v", err, tt.want)
			}
			cerr := Check(path)
			// Check reads no content: it cannot see an empty or a too
			// large file in the same way, so compare only the file states.
			if tt.want != ErrEmpty && tt.want != ErrTooLarge && !errors.Is(cerr, tt.want) {
				t.Errorf("Check error = %v, want %v", cerr, tt.want)
			}
			if err != nil {
				if !strings.Contains(err.Error(), tt.text) {
					t.Errorf("error %q does not contain %q", err, tt.text)
				}
				if strings.Contains(err.Error(), marker) || strings.Contains(fmt.Sprint(cerr), marker) {
					t.Error("an error contains the token")
				}
				if !tok.Empty() {
					t.Error("a failed read returned a token")
				}
				return
			}
			if tok.Reveal() != marker {
				t.Error("the token is not the trimmed content of the file")
			}
		})
	}
}

// fakeInfo is a file info with an owner.
type fakeInfo struct {
	mode fs.FileMode
	sys  any
}

func (f fakeInfo) Name() string       { return "onepassword-token" }
func (f fakeInfo) Size() int64        { return 10 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return f.sys }

func TestCheckInfoOwner(t *testing.T) {
	// G3: a file of another user, also of root, is a finding.
	path := filepath.Join(shortDir(t), "t")
	write(t, path, marker, 0o600)
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckInfo(fi, os.Getuid()); err != nil {
		t.Fatalf("own file: %v", err)
	}
	if err := CheckInfo(fi, os.Getuid()+1); !errors.Is(err, ErrOwner) {
		t.Errorf("other user: %v, want ErrOwner", err)
	}
	if err := CheckInfo(fakeInfo{mode: 0o600}, os.Getuid()); !errors.Is(err, ErrOwner) {
		t.Errorf("unknown owner: %v, want ErrOwner", err)
	}
	if err := CheckInfo(fakeInfo{mode: 0o600 | fs.ModeSetuid}, os.Getuid()); !errors.Is(err, ErrMode) {
		t.Errorf("setuid: %v, want ErrMode", err)
	}
}

func TestFileSource(t *testing.T) {
	// D51: the token file is a token.Source.
	path := filepath.Join(shortDir(t), "onepassword-token")
	write(t, path, marker+"\n", 0o600)
	var src token.Source = File{Path: path}
	tok, err := src.Token()
	if err != nil || tok.Reveal() != marker {
		t.Fatalf("Token() = %v, %v", tok, err)
	}
	if _, err := (File{Path: path + ".fehlt"}).Token(); !errors.Is(err, ErrMissing) {
		t.Errorf("missing file: %v", err)
	}
}
