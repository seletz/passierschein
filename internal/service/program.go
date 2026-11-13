package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ResolveProgram returns the absolute real path of exe: links resolved.
// If a38 runs through a link in the PATH to the bundle
// (~/.local/bin/a38 -> /Applications/Passierschein.app/Contents/MacOS/a38,
// D50), the result is the path in the bundle.
func ResolveProgram(exe string) (string, error) {
	if exe == "" {
		return "", errors.New("the path of the running a38 is unknown")
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("cannot resolve the path of a38: %w", err)
	}
	return real, nil
}

// InBundle reports whether path lies in an app bundle as
// <X>.app/Contents/MacOS/<name>.
func InBundle(path string) bool {
	dir := filepath.Dir(path)
	return filepath.Base(dir) == "MacOS" && filepath.Base(filepath.Dir(dir)) == "Contents" &&
		strings.HasSuffix(filepath.Dir(filepath.Dir(dir)), ".app")
}

// CheckProgram checks the file that launchd starts at each login, and its
// directory. A file that another user can change would run with the rights
// of the user and would read the token (D51). The rules: a regular file
// that the owner can execute; the file and its directory belong to the
// user uid or to root; neither group nor others can write them.
func CheckProgram(path string, uid int) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("the program %s cannot be read: %w", path, err)
	}
	if err := checkOwnerMode(fi, uid, "the program "+path); err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("the program %s is not a regular file", path)
	}
	if fi.Mode().Perm()&0o100 == 0 {
		return fmt.Errorf("the owner cannot execute the program %s", path)
	}
	dir := filepath.Dir(path)
	di, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("the directory %s cannot be read: %w", dir, err)
	}
	return checkOwnerMode(di, uid, "the directory "+dir)
}

func checkOwnerMode(fi fs.FileInfo, uid int, what string) error {
	if p := fi.Mode().Perm(); p&0o022 != 0 {
		return fmt.Errorf("%s is writable by group or others (mode %04o)", what, p)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("the owner of %s is unknown", what) // fail closed
	}
	if int(st.Uid) != uid && st.Uid != 0 {
		return fmt.Errorf("%s belongs to user %d, not to the user or root", what, st.Uid)
	}
	return nil
}

// ProgramWarnings returns the warnings for a program path that may not
// stay: a temporary directory, the build cache of Go or a source checkout
// (a directory with go.mod above the program). The service then breaks
// with the next clean-up or build.
func ProgramWarnings(path string) []string {
	var w []string
	tmp := []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders", "/private/var/folders"}
	for _, t := range tmp {
		t = filepath.Clean(t)
		if t != "" && t != "/" && (path == t || strings.HasPrefix(path, t+"/")) {
			w = append(w, fmt.Sprintf("the program %s lies in a temporary directory; the service breaks when the system removes it. Install a38 from the release (how-to: Daemon als Dienst einrichten)", path))
			break
		}
	}
	if strings.Contains(path, "/go-build") {
		w = append(w, fmt.Sprintf("the program %s lies in the build cache of Go; install a built a38 instead", path))
	}
	for dir := filepath.Dir(path); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			w = append(w, fmt.Sprintf("the program %s lies in the source directory %s; the next build changes it, and the keychain item of the token then needs a confirmation (D51). Use the release build for the service", path, dir))
			break
		}
	}
	return w
}
