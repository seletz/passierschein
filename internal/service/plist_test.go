package service

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write the golden files")

// goldenSpec is the plist of the golden file: the release layout of D50
// with the 1Password backend and the app.
func goldenSpec() Spec {
	return Spec{
		Program: "/Applications/Passierschein.app/Contents/MacOS/a38",
		Args:    []string{"serve", "--backend", "1password", "--ui", "app"},
		LogPath: "/Users/beispiel/Library/Logs/a38/serve.log",
	}
}

func TestRenderGolden(t *testing.T) {
	// F15: the plist of the LaunchAgent, byte for byte.
	got := goldenSpec().Render()
	path := filepath.Join("testdata", "daemon.plist")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("plist differs from %s (go test -run TestRenderGolden -update):\n%s", path, got)
	}
}

func TestRenderPassesPlutil(t *testing.T) {
	// F15: plutil accepts each generated plist as valid.
	plutil, err := exec.LookPath("plutil")
	if err != nil {
		t.Skip("plutil exists on macOS only")
	}
	specs := map[string]Spec{
		"golden": goldenSpec(),
		"fake with config and env": {
			Program: "/Users/beispiel/.local/bin/a38 & <co>",
			Args:    []string{"serve", "--backend", "fake", "--ui", "auto-approve", "--fake-file", "/tmp/beispiel fake.yaml", "--config", "/tmp/a&b/config.yaml"},
			LogPath: "/tmp/log",
			Env:     map[string]string{"XDG_STATE_HOME": "/tmp/state", "XDG_CONFIG_HOME": "/tmp/cfg"},
		},
	}
	for name, s := range specs {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "x.plist")
			if err := os.WriteFile(file, s.Render(), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(plutil, "-lint", file).CombinedOutput()
			if err != nil || !strings.Contains(string(out), "OK") {
				t.Fatalf("plutil -lint: %v: %s", err, out)
			}
		})
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	// F15: the package reads back what it writes, also with characters
	// that XML escapes.
	s := Spec{
		Program: "/Users/beispiel/a&b <c>/a38",
		Args:    []string{"serve", "--backend", "1password", "--ui", "app", "--config", "/Users/beispiel/\"x\"/config.yaml"},
		LogPath: "/Users/beispiel/Library/Logs/a38/serve.log",
		Env:     map[string]string{"XDG_STATE_HOME": "/Users/beispiel/state", "A38_TOKEN": "beispiel-wert-marker", "PATH": "/bin"},
	}
	data := s.Render()
	if !HasMarker(data) {
		t.Fatal("the plist has no marker")
	}
	p, err := ParsePlist(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Label != Label {
		t.Errorf("Label = %q", p.Label)
	}
	want := append([]string{s.Program}, s.Args...)
	if !slices.Equal(p.ProgramArguments, want) {
		t.Errorf("ProgramArguments = %q, want %q", p.ProgramArguments, want)
	}
	if got := p.Arg("--config"); got != "/Users/beispiel/\"x\"/config.yaml" {
		t.Errorf("Arg(--config) = %q", got)
	}
	env, _ := p.Values["EnvironmentVariables"].(map[string]any)
	if len(env) != 1 || env["XDG_STATE_HOME"] != "/Users/beispiel/state" {
		t.Errorf("EnvironmentVariables = %v; only the XDG variables may pass", env)
	}
	if bytes.Contains(data, []byte("beispiel-wert-marker")) {
		t.Error("the plist contains a variable that is not allowed")
	}
	checks := map[string]any{
		"RunAtLoad": true, "KeepAlive": false, "LimitLoadToSessionType": "Aqua",
		"ProcessType": "Interactive", "StandardErrorPath": s.LogPath, "StandardOutPath": s.LogPath,
		"Umask": int64(63), "ExitTimeOut": int64(20),
	}
	for k, v := range checks {
		if p.Values[k] != v {
			t.Errorf("%s = %#v, want %#v", k, p.Values[k], v)
		}
	}
	if ids, _ := p.Values["AssociatedBundleIdentifiers"].([]any); len(ids) != 1 || ids[0] != BundleID {
		t.Errorf("AssociatedBundleIdentifiers = %v", p.Values["AssociatedBundleIdentifiers"])
	}
}

func TestRenderWithoutEnv(t *testing.T) {
	// No variable: no key EnvironmentVariables.
	if bytes.Contains(goldenSpec().Render(), []byte("EnvironmentVariables")) {
		t.Error("the plist has EnvironmentVariables without a variable")
	}
}

func TestParsePlistRejects(t *testing.T) {
	tests := map[string]string{
		"empty":        "",
		"binary":       "bplist00\x00\x01",
		"no plist":     `<?xml version="1.0"?><dict></dict>`,
		"array on top": `<plist version="1.0"><array></array></plist>`,
		"bad integer":  `<plist version="1.0"><dict><key>Umask</key><integer>x</integer></dict></plist>`,
		"truncated":    `<plist version="1.0"><dict><key>Label</key>`,
		"unknown":      `<plist version="1.0"><dict><key>Label</key><data>AA==</data></dict></plist>`,
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePlist([]byte(in)); err == nil {
				t.Error("ParsePlist accepted the input")
			}
		})
	}
}

func TestHasMarker(t *testing.T) {
	if HasMarker([]byte(`<plist version="1.0"><dict><key>Label</key><string>` + Label + `</string></dict></plist>`)) {
		t.Error("a plist without the comment counts as ours")
	}
}
