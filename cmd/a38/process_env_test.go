package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// childKeychainService is the keychain item that a test-built a38 looks
// at instead of the product item a38.onepassword. No test creates it, so
// the child reads at most the metadata of a missing test item.
const childKeychainService = "a38-test-process-env"

// droppedChildEnv are the variables that a child a38 never inherits from
// the test run: they point at the files of the human.
var droppedChildEnv = map[string]bool{
	"HOME": true, "XDG_CONFIG_HOME": true, "XDG_STATE_HOME": true, "XDG_DATA_HOME": true,
	"A38_CONFIG": true, "A38_TEST_KEYCHAIN_SERVICE": true,
}

// a38Cmd returns a command for the test-built a38. Each process test that
// starts the binary uses it (TestOnlyA38CmdStartsTheBinary checks this).
// The child gets a temporary HOME and XDG directories, so that "a38
// doctor" and "a38 service" find no plist of the human and never ask the
// real launchd (issue 70), and the test keychain item, so that the child
// never looks up the product item (D51).
func a38Cmd(ctx context.Context, t *testing.T, bin string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // the test-built binary
	cmd.Env = childEnv(t)
	assertChildEnv(t, cmd.Env)
	return cmd
}

// childEnv returns the environment of the test run with a temporary HOME.
func childEnv(t *testing.T) []string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "a38-home-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	var env []string
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); !droppedChildEnv[k] {
			env = append(env, kv)
		}
	}
	return append(env,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"A38_TEST_KEYCHAIN_SERVICE="+childKeychainService,
	)
}

// assertChildEnv fails the test unless [checkChildEnv] accepts env.
func assertChildEnv(t *testing.T, env []string) {
	t.Helper()
	if err := checkChildEnv(env); err != nil {
		t.Fatalf("child environment: %v", err)
	}
}

// checkChildEnv accepts env only with a temporary HOME without a
// LaunchAgents directory, temporary XDG directories, no A38_CONFIG and the
// test keychain item.
func checkChildEnv(env []string) error {
	vals := map[string][]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		vals[k] = append(vals[k], v)
	}
	temp := func(p string) bool {
		return strings.HasPrefix(p, "/tmp/") || strings.HasPrefix(p, "/private/tmp/") ||
			strings.HasPrefix(p, filepath.Clean(os.TempDir())+"/")
	}
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME"} {
		if v := vals[k]; len(v) != 1 || !temp(v[0]) {
			return fmt.Errorf("%s = %q; it must be one temporary directory", k, v)
		}
	}
	home := vals["HOME"][0]
	if real, _ := os.UserHomeDir(); real != "" && strings.HasPrefix(real+"/", home+"/") {
		return fmt.Errorf("HOME %s holds the home of the user", home)
	}
	if _, err := os.Lstat(filepath.Join(home, "Library", "LaunchAgents")); err == nil {
		return fmt.Errorf("%s has a LaunchAgents directory", home)
	}
	if len(vals["A38_CONFIG"]) != 0 {
		return fmt.Errorf("A38_CONFIG is set")
	}
	if v := vals["A38_TEST_KEYCHAIN_SERVICE"]; len(v) != 1 || !strings.HasPrefix(v[0], "a38-test-") {
		return fmt.Errorf("A38_TEST_KEYCHAIN_SERVICE = %q; the child must not look up the product item", v)
	}
	return nil
}

func TestChildEnv(t *testing.T) {
	// Issue 70: the guard accepts the environment of the helper and
	// refuses each way to reach the files or the keychain item of the
	// human.
	t.Setenv("A38_CONFIG", "/x/config.yaml")
	good := childEnv(t)
	if err := checkChildEnv(good); err != nil {
		t.Fatal(err)
	}
	with := func(drop, add string) []string {
		var e []string
		for _, kv := range good {
			if !strings.HasPrefix(kv, drop+"=") {
				e = append(e, kv)
			}
		}
		if add != "" {
			e = append(e, add)
		}
		return e
	}
	home := ""
	for _, kv := range good {
		if v, ok := strings.CutPrefix(kv, "HOME="); ok {
			home = v
		}
	}
	tests := map[string][]string{
		"real HOME":            with("HOME", "HOME=/Users/beispiel"),
		"no HOME":              with("HOME", ""),
		"real state directory": with("XDG_STATE_HOME", "XDG_STATE_HOME=/Users/beispiel/.local/state"),
		"A38_CONFIG":           with("", "A38_CONFIG=/Users/beispiel/.config/a38/config.yaml"),
		"product keychain":     with("A38_TEST_KEYCHAIN_SERVICE", ""),
		"other keychain item":  with("A38_TEST_KEYCHAIN_SERVICE", "A38_TEST_KEYCHAIN_SERVICE=a38.onepassword"),
	}
	for name, env := range tests {
		if err := checkChildEnv(env); err == nil {
			t.Errorf("%s: the guard accepts the environment", name)
		}
	}
	if err := os.MkdirAll(filepath.Join(home, "Library", "LaunchAgents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkChildEnv(good); err == nil {
		t.Error("the guard accepts a HOME with a LaunchAgents directory")
	}
}

// startPattern finds a start of the test-built binary.
var startPattern = regexp.MustCompile(`exec\.Command(Context)?\((ctx, )?bin[,)]`)

func TestOnlyA38CmdStartsTheBinary(t *testing.T) {
	// Issue 70: each process test starts the built a38 through a38Cmd, so
	// that no test-built a38 sees the HOME of the human.
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f == "process_env_test.go" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if startPattern.MatchString(line) {
				t.Errorf("%s:%d starts the binary without a38Cmd: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}
