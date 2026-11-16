package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
)

// configHome sets a temporary HOME with a short path and clears the
// variables that move the configuration file. No test touches the real
// ~/.config/a38.
func configHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv(config.EnvConfig, "")
	return home
}

func a38(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(args, &o, &e)
	return code, o.String(), e.String()
}

func mustA38(t *testing.T, args ...string) string {
	t.Helper()
	code, out, errOut := a38(t, args...)
	if code != 0 {
		t.Fatalf("a38 %v: exit %d\nstdout: %s\nstderr: %s", args, code, out, errOut)
	}
	return out
}

func TestConfigRoundTrip(t *testing.T) {
	// F15 (part), UC11 (part): init, vault add, project add, show, validate, load.
	home := configHome(t)
	cfgPath := filepath.Join(home, ".config", "a38", "config.yaml")
	proj := filepath.Join(home, "develop", "beispielprojekt")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	out := mustA38(t, "config", "init")
	for _, want := range []string{cfgPath, "a38 config account add", "a38 config vault add", "a38 config onepassword-token set --account", "a38 doctor"} {
		if !strings.Contains(out, want) {
			t.Errorf("init output lacks %q:\n%s", want, out)
		}
	}
	// T43 (part), D70: a vault names its account; the account comes first.
	if code, _, errOut := a38(t, "config", "vault", "add", "beispiel.app.k8s-dev", "--environment", "dev", "--account", "privat"); code != 1 || !strings.Contains(errOut, "a38 config account add") {
		t.Errorf("vault add with an unknown account: code %d, %s", code, errOut)
	}
	out = mustA38(t, "config", "account", "add", "privat")
	if !strings.Contains(out, "a38 config onepassword-token set --account privat") {
		t.Errorf("account add lacks the next step:\n%s", out)
	}
	mustA38(t, "config", "account", "add", "firma", "--token-source", "file")
	out = mustA38(t, "config", "vault", "add", "beispiel.app.k8s-dev", "--environment", "dev", "--account", "privat")
	if !strings.Contains(out, "reads the configuration file only at its start") {
		t.Errorf("no restart note:\n%s", out)
	}
	// Flags after the positional argument and before it.
	mustA38(t, "config", "vault", "add", "--environment", "prod", "--account", "firma", "beispiel.prod")
	if out := mustA38(t, "config", "account", "list"); !strings.Contains(out, "privat") || !strings.Contains(out, "beispiel.prod") || !strings.Contains(out, "file") {
		t.Errorf("account list:\n%s", out)
	}
	if out := mustA38(t, "config", "vault", "list"); !strings.Contains(out, "beispiel.prod") || !strings.Contains(out, "firma") {
		t.Errorf("vault list:\n%s", out)
	}
	mustA38(t, "config", "project", "add", "beispielprojekt", "--path", "~/develop/beispielprojekt", "--path", proj+"__worktrees", "--vault", "beispiel.app.k8s-dev")

	code, out, _ := a38(t, "config", "validate")
	if code != 1 || !strings.Contains(out, "does not exist") {
		t.Errorf("validate: code %d, want 1 for the missing worktree path:\n%s", code, out)
	}
	if err := os.MkdirAll(proj+"__worktrees", 0o755); err != nil {
		t.Fatal(err)
	}
	if out := mustA38(t, "config", "validate"); !strings.Contains(out, "No findings.") {
		t.Errorf("validate:\n%s", out)
	}

	var doc showDoc
	if err := json.Unmarshal([]byte(mustA38(t, "config", "show", "--json")), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vaults) != 2 || len(doc.Projects) != 1 || !doc.Valid || doc.Projects[0].Paths[0].Written != "~/develop/beispielprojekt" {
		t.Errorf("show: %+v", doc)
	}
	if len(doc.Accounts) != 2 || doc.Accounts[1].Name != "firma" || doc.Accounts[1].TokenSource != "file" || doc.Vaults[1].Account != "firma" {
		t.Errorf("show accounts: %+v, vaults %+v", doc.Accounts, doc.Vaults)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Vaults) != 2 || len(cfg.Projects) != 1 || cfg.Projects[0].Invalid != "" || len(cfg.Projects[0].Paths) != 2 {
		t.Errorf("loaded: %+v", cfg)
	}

	// A vault in use is refused and the projects are named.
	code, _, errOut := a38(t, "config", "vault", "remove", "beispiel.app.k8s-dev")
	if code != 1 || !strings.Contains(errOut, `"beispielprojekt"`) {
		t.Errorf("remove vault in use: code %d, %s", code, errOut)
	}
	mustA38(t, "config", "project", "remove", "beispielprojekt")
	mustA38(t, "config", "vault", "remove", "beispiel.app.k8s-dev")
	// An account that a vault names is not removed.
	if code, _, errOut := a38(t, "config", "account", "remove", "firma"); code != 1 || !strings.Contains(errOut, `"beispiel.prod"`) {
		t.Errorf("remove account in use: code %d, %s", code, errOut)
	}
	if out := mustA38(t, "config", "account", "remove", "privat"); !strings.Contains(out, "a38 config onepassword-token delete --account privat") {
		t.Errorf("account remove lacks the hint:\n%s", out)
	}
	if out := mustA38(t, "config", "vault", "list"); strings.Contains(out, "k8s-dev") || !strings.Contains(out, "beispiel.prod") {
		t.Errorf("vault list:\n%s", out)
	}
	if out := mustA38(t, "config", "project", "list"); !strings.Contains(out, "No projects") {
		t.Errorf("project list:\n%s", out)
	}
	if _, err := config.Load(cfgPath); err != nil {
		t.Errorf("file after the edits: %v", err)
	}
}

func TestConfigPath(t *testing.T) {
	// F15 (part): where the configuration comes from.
	home := configHome(t)
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want []string
	}{
		{"default", nil, nil, []string{filepath.Join(home, ".config/a38/config.yaml"), "default location", "no. Create it with: a38 config init"}},
		{"xdg", map[string]string{"XDG_CONFIG_HOME": home + "/x"}, nil, []string{home + "/x/a38/config.yaml", "XDG_CONFIG_HOME"}},
		{"env", map[string]string{config.EnvConfig: home + "/e.yaml"}, nil, []string{home + "/e.yaml", "environment variable A38_CONFIG"}},
		{"flag", map[string]string{config.EnvConfig: home + "/e.yaml"}, []string{"--config", home + "/f.yaml"}, []string{home + "/f.yaml", "the flag --config", "priority"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			out := mustA38(t, append([]string{"config", "path"}, tt.args...)...)
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("lacks %q:\n%s", w, out)
				}
			}
		})
	}
	t.Run("json and exists", func(t *testing.T) {
		p := filepath.Join(home, "j.yaml")
		if err := os.WriteFile(p, []byte("version: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var doc pathDoc
		if err := json.Unmarshal([]byte(mustA38(t, "config", "path", "--json", "--config", p)), &doc); err != nil {
			t.Fatal(err)
		}
		if !doc.Exists || doc.Source != "flag" || doc.Mode != "0600" {
			t.Errorf("%+v", doc)
		}
	})
}

func TestConfigInitRefusals(t *testing.T) {
	// F15 (part): init never overwrites.
	home := configHome(t)
	cfgPath := filepath.Join(home, ".config", "a38", "config.yaml")
	mustA38(t, "config", "init")
	assertFileMode(t, filepath.Dir(cfgPath), 0o700)
	assertFileMode(t, cfgPath, 0o600)
	first, _ := os.ReadFile(cfgPath)

	code, _, errOut := a38(t, "config", "init")
	if code != 1 || !strings.Contains(errOut, cfgPath) || !strings.Contains(errOut, "--force") {
		t.Errorf("second init: code %d, %s", code, errOut)
	}
	mustA38(t, "config", "account", "add", "privat")
	changed, _ := os.ReadFile(cfgPath)
	if bytes.Equal(first, changed) {
		t.Fatal("vault add changed nothing")
	}
	out := mustA38(t, "config", "init", "--force")
	if !strings.Contains(out, "Moved the old file to") {
		t.Errorf("force output: %s", out)
	}
	backups, _ := filepath.Glob(cfgPath + ".bak-*")
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	if b, _ := os.ReadFile(backups[0]); !bytes.Equal(b, changed) {
		t.Error("the backup is not the old file")
	}
	if b, _ := os.ReadFile(cfgPath); !bytes.Equal(b, first) {
		t.Error("the new file is not the template")
	}
}

func TestConfigSymlinkAndReadOnly(t *testing.T) {
	home := configHome(t)
	real := filepath.Join(home, "real.yaml")
	link := filepath.Join(home, "link.yaml")
	if err := os.WriteFile(real, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"config", "init", "--force", "--config", link},
		{"config", "account", "add", "privat", "--config", link},
	} {
		code, _, errOut := a38(t, args...)
		if code != 1 || !strings.Contains(errOut, "symbolic link") {
			t.Errorf("%v: code %d, %s", args, code, errOut)
		}
	}
	if b, _ := os.ReadFile(real); string(b) != "version: 1\n" {
		t.Errorf("target changed: %s", b)
	}

	if os.Geteuid() != 0 {
		ro := filepath.Join(home, "ro")
		if err := os.Mkdir(ro, 0o700); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(ro, "config.yaml")
		if err := os.WriteFile(p, []byte("version: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(ro, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
		code, _, errOut := a38(t, "config", "account", "add", "privat", "--config", p)
		if code != 1 || !strings.Contains(errOut, ro) {
			t.Errorf("read-only dir: code %d, %s", code, errOut)
		}
		if b, _ := os.ReadFile(p); string(b) != "version: 1\n" {
			t.Errorf("file changed: %s", b)
		}
	}
}

func TestConfigShowAndValidateBrokenFile(t *testing.T) {
	// F15 (part): a broken file gives exit 1, the error, no partial dump.
	home := configHome(t)
	p := filepath.Join(home, "broken.yaml")
	if err := os.WriteFile(p, []byte("version: 1\ngrant:\n  lifetime: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := a38(t, "config", "show", "--config", p)
	if code != 1 || out != "" || !strings.Contains(errOut, "line") {
		t.Errorf("show: code %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, out, _ = a38(t, "config", "validate", "--config", p)
	if code != 1 || !strings.Contains(out, "FINDING") || !strings.Contains(out, "Next step") {
		t.Errorf("validate: code %d, %s", code, out)
	}
	code, _, errOut = a38(t, "config", "show", "--config", filepath.Join(home, "missing.yaml"))
	if code != 1 || !strings.Contains(errOut, "a38 config init") {
		t.Errorf("show missing: code %d, %s", code, errOut)
	}
}

func TestConfigShowInvalidProject(t *testing.T) {
	home := configHome(t)
	p := filepath.Join(home, "c.yaml")
	src := "version: 1\nvaults: []\nprojects:\n  - name: beispielprojekt\n    paths:\n      - /nonexistent-a38/p\n    vaults:\n      - datenbank-dev\n"
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	out := mustA38(t, "config", "show", "--config", p)
	for _, want := range []string{"INVALID", `"datenbank-dev"`, "(missing)", "[default]", "[file]"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
	if code, _, _ := a38(t, "config", "validate", "--config", p); code != 1 {
		t.Errorf("validate code = %d, want 1", code)
	}
}

func TestConfigHelpAndUsage(t *testing.T) {
	configHome(t)
	_, out, _ := a38(t, "config", "--help")
	for _, sub := range []string{"path", "show", "init", "validate", "account", "vault", "project", "onepassword-token"} {
		if !strings.Contains(out, "  "+sub+" ") {
			t.Errorf("config --help lacks %q:\n%s", sub, out)
		}
	}
	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"path help", []string{"config", "path", "--help"}, 0, "a38 config path"},
		{"show help", []string{"config", "show", "-h"}, 0, "a38 config show"},
		{"init help", []string{"config", "init", "--help"}, 0, "--force"},
		{"validate help", []string{"config", "validate", "--help"}, 0, "a38 config validate"},
		{"vault help", []string{"config", "vault", "--help"}, 0, "a38 config vault add"},
		{"vault add help", []string{"config", "vault", "add", "--help"}, 0, "--environment"},
		{"project help", []string{"config", "project", "--help"}, 0, "--path DIR"},
		{"project remove help", []string{"config", "project", "remove", "--help"}, 0, "a38 config project remove"},
		{"vault without sub", []string{"config", "vault"}, 2, ""},
		{"vault unknown", []string{"config", "vault", "x"}, 2, ""},
		{"vault add without environment", []string{"config", "vault", "add", "x"}, 2, ""},
		{"vault add without account", []string{"config", "vault", "add", "x", "--environment", "dev"}, 2, "--account is necessary"},
		{"account help", []string{"config", "account", "--help"}, 0, "a38 config account add"},
		{"account add two names", []string{"config", "account", "add", "x", "y"}, 2, ""},
		{"vault add two names", []string{"config", "vault", "add", "x", "y", "--environment", "dev"}, 2, ""},
		{"unknown flag", []string{"config", "show", "--nope"}, 2, ""},
		{"path argument", []string{"config", "path", "x"}, 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, o, e := a38(t, tt.args...)
			if code != tt.code || !strings.Contains(o+e, tt.want) {
				t.Errorf("code %d\nstdout: %s\nstderr: %s", code, o, e)
			}
		})
	}
}

func TestConfigVaultAddErrors(t *testing.T) {
	// F15 (part): the same rules as loading, before the write.
	home := configHome(t)
	p := filepath.Join(home, "c.yaml")
	src := "version: 1\nvaults:\n  - name: datenbank-dev\n    environment: dev\n    account: privat\nbackend:\n  onepassword:\n    accounts:\n      - name: privat\n"
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"duplicate", []string{"vault", "add", "datenbank-dev", "--environment", "dev", "--account", "privat"}, "in the list already"},
		{"environment", []string{"vault", "add", "x", "--environment", "staging", "--account", "privat"}, "not dev, test or prod"},
		{"unknown account", []string{"vault", "add", "x", "--environment", "dev", "--account", "firma"}, "a38 config account add"},
		{"account twice", []string{"account", "add", "privat"}, "in the list already"},
		{"account in use", []string{"account", "remove", "privat"}, `"datenbank-dev"`},
		{"project unknown vault", []string{"project", "add", "p", "--path", "/srv/p", "--vault", "nope"}, "not in the vault list"},
		{"project relative path", []string{"project", "add", "p", "--path", "rel", "--vault", "datenbank-dev"}, "must be absolute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, errOut := a38(t, append(append([]string{"config"}, tt.args...), "--config", p)...)
			if code != 1 || !strings.Contains(errOut, tt.want) {
				t.Errorf("code %d: %s", code, errOut)
			}
			if b, _ := os.ReadFile(p); string(b) != src {
				t.Errorf("the file changed: %s", b)
			}
		})
	}
}

func TestConfigEditKeepsComments(t *testing.T) {
	// F15 (part): a commented file survives an edit except the changed block.
	home := configHome(t)
	p := filepath.Join(home, "c.yaml")
	src := "# my file\nversion: 1\n\n# vaults\nvaults:\n  - name: datenbank-dev # dev\n    environment: dev\n    account: privat\n\n# end\ngrant:\n  lifetime: 2h\nbackend:\n  onepassword:\n    accounts: [{name: privat}]\n"
	if err := os.WriteFile(p, []byte(src), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o640); err != nil {
		t.Fatal(err)
	}
	mustA38(t, "config", "vault", "add", "beispiel.test", "--environment", "test", "--account", "privat", "--config", p)
	got, _ := os.ReadFile(p)
	want := strings.Replace(src, "    account: privat\n\n", "    account: privat\n  - name: beispiel.test\n    environment: test\n    account: privat\n\n", 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	assertFileMode(t, p, 0o640)
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != want {
		t.Errorf("%s mode = %o, want %o", path, fi.Mode().Perm(), want)
	}
}

func TestConfigProjectAddPath(t *testing.T) {
	// F15 (part), issue 92: a second path for an existing project, for
	// the directory of the worktrees.
	home := configHome(t)
	proj := filepath.Join(home, "develop", "beispielprojekt")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	mustA38(t, "config", "init")
	mustA38(t, "config", "account", "add", "privat")
	mustA38(t, "config", "vault", "add", "datenbank-dev", "--environment", "dev", "--account", "privat")
	mustA38(t, "config", "project", "add", "beispielprojekt", "--path", proj, "--vault", "datenbank-dev")

	wt := proj + "__worktrees"
	out := mustA38(t, "config", "project", "add-path", "beispielprojekt", "--path", wt)
	for _, want := range []string{"beispielprojekt", wt, "do not exist yet", "reads the configuration file only at its start"} {
		if !strings.Contains(out, want) {
			t.Errorf("add-path output lacks %q:\n%s", want, out)
		}
	}
	list := mustA38(t, "config", "project", "list")
	if !strings.Contains(list, wt) {
		t.Errorf("project list lacks the new path:\n%s", list)
	}

	// Errors: unknown project, no path, path already there.
	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"unknown project", []string{"config", "project", "add-path", "nope", "--path", wt + "2"}, 1, "not in the list"},
		{"no path", []string{"config", "project", "add-path", "beispielprojekt"}, 2, "--path"},
		{"no name", []string{"config", "project", "add-path", "--path", wt + "2"}, 2, "exactly one project name"},
		{"path already there", []string{"config", "project", "add-path", "beispielprojekt", "--path", wt}, 1, "already a path"},
		{"help", []string{"config", "project", "add-path", "--help"}, 0, "a38 config project add-path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, o, e := a38(t, tt.args...)
			if code != tt.code || !strings.Contains(o+e, tt.want) {
				t.Errorf("code %d\nstdout: %s\nstderr: %s", code, o, e)
			}
		})
	}
}

func TestConfigValidateMissingUIApp(t *testing.T) {
	// T27 (part), issue 92: a paths.ui_app that does not exist is a
	// finding of "a38 config validate", with the cause and the next step.
	home := configHome(t)
	p := filepath.Join(home, "c.yaml")
	missing := filepath.Join(home, "Passierschein.app", "Contents", "MacOS", "Passierschein")
	src := "version: 1\npaths:\n  ui_app: " + missing + "\n"
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := a38(t, "config", "validate", "--config", p)
	for _, want := range []string{"paths.ui_app", missing, "Next step"} {
		if code != 1 || !strings.Contains(out, want) {
			t.Fatalf("code %d, output lacks %q:\n%s", code, want, out)
		}
	}
	// The file exists: no finding.
	if err := os.MkdirAll(filepath.Dir(missing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(missing, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out := mustA38(t, "config", "validate", "--config", p); !strings.Contains(out, "No findings.") {
		t.Errorf("validate with the app present:\n%s", out)
	}
}
