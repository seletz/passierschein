package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// commented is a file with comments, blank lines and 2-space lists.
const commented = `# Configuration of a38.
version: 1

# The vaults.
vaults:
  - name: beispiel.app.k8s-dev # the dev vault
    environment: dev
    account: privat

# The projects.
projects:
  - name: beispielprojekt
    paths:
      - /srv/beispielprojekt
    vaults:
      - beispiel.app.k8s-dev

# Grants.
grant:
  lifetime: 4h # long enough

# The accounts.
backend:
  onepassword:
    accounts:
      - name: privat # the private account
`

func TestAddVaultKeepsComments(t *testing.T) {
	// F15 (part): the edit keeps the file byte for byte except the block.
	got, err := AddVault([]byte(commented), "datenbank-dev", EnvTest, "privat")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(commented, "      - beispiel.app.k8s-dev\n\n# Grants", "      - beispiel.app.k8s-dev\n\n# Grants", 1)
	want = strings.Replace(want, "    account: privat\n\n# The projects.", "    account: privat\n  - name: datenbank-dev\n    environment: test\n    account: privat\n\n# The projects.", 1)
	if string(got.Data) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got.Data, want)
	}
}

func TestRemoveProjectAndVaultKeepComments(t *testing.T) {
	// F15 (part).
	got, err := RemoveProject([]byte(commented), "beispielprojekt")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(commented, `  - name: beispielprojekt
    paths:
      - /srv/beispielprojekt
    vaults:
      - beispiel.app.k8s-dev
`, "", 1)
	want = strings.Replace(want, "projects:\n", "projects: []\n", 1)
	if string(got.Data) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got.Data, want)
	}
	got, err = RemoveVault(got.Data, "beispiel.app.k8s-dev")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got.Data), "vaults: []\n") || !strings.Contains(string(got.Data), "lifetime: 4h # long enough") {
		t.Fatalf("unexpected result:\n%s", got.Data)
	}
}

// shortHome sets a temporary HOME with a short path (a socket path is
// limited) and no XDG variable.
func shortHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv(EnvConfig, "")
	return home
}

// withPrivat is a minimal file with the account privat.
const withPrivat = "version: 1\nbackend:\n  onepassword:\n    accounts:\n      - name: privat\n"

func TestEditTable(t *testing.T) {
	shortHome(t)
	tmpl, err := Template()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		src     string
		do      func(string) (Edit, error)
		wantErr string
		wantIn  []string
	}{
		{"duplicate vault", commented, func(s string) (Edit, error) { return AddVault([]byte(s), "beispiel.app.k8s-dev", EnvDev, "privat") }, "in the list already", nil},
		{"unknown environment", commented, func(s string) (Edit, error) { return AddVault([]byte(s), "x", "staging", "privat") }, "not dev, test or prod", nil},
		{"empty environment", commented, func(s string) (Edit, error) { return AddVault([]byte(s), "x", "", "privat") }, "not dev, test or prod", nil},
		{"bad name", commented, func(s string) (Edit, error) { return AddVault([]byte(s), "a/b", EnvDev, "privat") }, "slash", nil},
		{"remove unknown vault", commented, func(s string) (Edit, error) { return RemoveVault([]byte(s), "nope") }, "not in the list", nil},
		{"remove vault in use", commented, func(s string) (Edit, error) { return RemoveVault([]byte(s), "beispiel.app.k8s-dev") }, `"beispielprojekt"`, nil},
		{"project unknown vault", commented, func(s string) (Edit, error) {
			return AddProject([]byte(s), "p2", []string{"/srv/p2"}, []string{"nope"})
		}, "not in the vault list", nil},
		{"project relative path", commented, func(s string) (Edit, error) {
			return AddProject([]byte(s), "p2", []string{"rel/dir"}, []string{"beispiel.app.k8s-dev"})
		}, "must be absolute", nil},
		{"project no path", commented, func(s string) (Edit, error) {
			return AddProject([]byte(s), "p2", nil, []string{"beispiel.app.k8s-dev"})
		}, "at least one path", nil},
		{"project no vault", commented, func(s string) (Edit, error) { return AddProject([]byte(s), "p2", []string{"/srv/p2"}, nil) }, "at least one vault", nil},
		{"project duplicate", commented, func(s string) (Edit, error) {
			return AddProject([]byte(s), "beispielprojekt", []string{"/srv/p2"}, []string{"beispiel.app.k8s-dev"})
		}, "in the list already", nil},
		{"project path of another project", commented, func(s string) (Edit, error) {
			return AddProject([]byte(s), "p2", []string{"/srv/beispielprojekt"}, []string{"beispiel.app.k8s-dev"})
		}, "also belongs", nil},
		{"project remove unknown", commented, func(s string) (Edit, error) { return RemoveProject([]byte(s), "nope") }, "not in the list", nil},
		{"broken file", "version: 2\n", func(s string) (Edit, error) { return AddVault([]byte(s), "x", EnvDev, "privat") }, "fix it first", nil},
		{"crlf", strings.ReplaceAll(commented, "\n", "\r\n"), func(s string) (Edit, error) { return AddVault([]byte(s), "x", EnvDev, "privat") }, "CRLF", nil},
		{"add vault to template (flow list)", string(tmpl), func(s string) (Edit, error) {
			e, err := AddAccount([]byte(s), "privat", TokenSourceKeychain, "")
			if err != nil {
				return e, err
			}
			return AddVault(e.Data, "datenbank-dev", EnvDev, "privat")
		}, "", []string{"vaults:\n  - name: datenbank-dev\n    environment: dev\n    account: privat\n\n# The projects."}},
		{"add project to template", string(tmpl), func(s string) (Edit, error) {
			e, err := AddAccount([]byte(s), "privat", TokenSourceKeychain, "")
			if err != nil {
				return e, err
			}
			e, err = AddVault(e.Data, "v", EnvDev, "privat")
			if err != nil {
				return e, err
			}
			return AddProject(e.Data, "p", []string{"~/develop/p", "/srv/p"}, []string{"v"})
		}, "", []string{"projects:\n  - name: p\n    paths:\n      - ~/develop/p\n      - /srv/p\n    vaults:\n      - v\n"}},
		{"key absent", withPrivat, func(s string) (Edit, error) { return AddVault([]byte(s), "v", EnvProd, "privat") }, "",
			[]string{"\nvaults:\n  - name: v\n    environment: prod\n    account: privat\n"}},
		{"key null", withPrivat + "vaults:\n", func(s string) (Edit, error) { return AddVault([]byte(s), "v", EnvProd, "privat") }, "",
			[]string{"vaults:\n  - name: v\n    environment: prod\n    account: privat\n"}},
		{"flow items are not edited", withPrivat + "vaults: [{name: a, environment: dev, account: privat}]\n", func(s string) (Edit, error) { return AddVault([]byte(s), "v", EnvProd, "privat") }, "a style that a38 does not edit", nil},
		// T43 (part): a vault names an account of the list (D70).
		{"vault without account", commented, func(s string) (Edit, error) { return AddVault([]byte(s), "v", EnvDev, "") }, "each vault names its 1Password account", nil},
		{"vault with unknown account", commented, func(s string) (Edit, error) { return AddVault([]byte(s), "v", EnvDev, "firma") }, `the account "firma" is not in backend.onepassword.accounts; add it first (a38 config account add)`, nil},
		{"add account", commented, func(s string) (Edit, error) {
			return AddAccount([]byte(s), "firma", TokenSourceFile, "~/a38/onepassword-token-firma")
		}, "", []string{"      - name: privat # the private account\n      - name: firma\n        token_source: file\n        token_file: ~/a38/onepassword-token-firma\n"}},
		{"add account keychain", commented, func(s string) (Edit, error) { return AddAccount([]byte(s), "firma", TokenSourceKeychain, "") }, "",
			[]string{"      - name: firma\n        token_source: keychain\n"}},
		{"add account twice", commented, func(s string) (Edit, error) { return AddAccount([]byte(s), "privat", TokenSourceKeychain, "") }, "in the list already", nil},
		{"add account bad name", commented, func(s string) (Edit, error) { return AddAccount([]byte(s), "Firma", TokenSourceKeychain, "") }, "lower-case", nil},
		{"add account reserved name", commented, func(s string) (Edit, error) { return AddAccount([]byte(s), LegacyAccount, TokenSourceKeychain, "") }, "reserved", nil},
		{"add account token file with keychain", commented, func(s string) (Edit, error) { return AddAccount([]byte(s), "firma", TokenSourceKeychain, "/tmp/t") }, "only with the token source file", nil},
		{"add account without section", "version: 1\n", func(s string) (Edit, error) { return AddAccount([]byte(s), "privat", TokenSourceKeychain, "") }, "",
			[]string{"backend:\n  onepassword:\n    accounts:\n      - name: privat\n        token_source: keychain\n"}},
		{"remove account in use", commented, func(s string) (Edit, error) { return RemoveAccount([]byte(s), "privat") }, `"beispiel.app.k8s-dev"`, nil},
		{"remove unknown account", commented, func(s string) (Edit, error) { return RemoveAccount([]byte(s), "firma") }, "not in the list", nil},
		{"missing path is a note", commented, func(s string) (Edit, error) {
			return AddProject([]byte(s), "p2", []string{"/nonexistent-a38/p2"}, []string{"beispiel.app.k8s-dev"})
		}, "", []string{"- /nonexistent-a38/p2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.do(tt.src)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range tt.wantIn {
				if !strings.Contains(string(got.Data), s) {
					t.Errorf("result lacks %q:\n%s", s, got.Data)
				}
			}
			if _, err := Parse(got.Data); err != nil {
				t.Errorf("result does not load: %v", err)
			}
			if tt.name == "missing path is a note" && len(got.Notes) != 1 {
				t.Errorf("notes = %v", got.Notes)
			}
		})
	}
}

func TestTemplateLoadsWithDefaults(t *testing.T) {
	// F15 (part): the file of "a38 config init" loads and states the token source.
	home := shortHome(t)
	data, err := Template()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Grant.Lifetime != DefaultGrantLifetime || cfg.Bundle.Window != DefaultBundleWindow ||
		cfg.Bundle.MaxEntries != DefaultBundleMaxEntries || cfg.SessionName.MaxLength != DefaultSessionNameMaxLength ||
		cfg.SessionName.AllowedCharacters != DefaultSessionNameCharacters {
		t.Errorf("template differs from the defaults: %+v", cfg)
	}
	if cfg.Paths.SocketDir != filepath.Join(home, ".local/state/a38/run") {
		t.Errorf("socket dir = %s", cfg.Paths.SocketDir)
	}
	in, err := Inspect(data)
	if err != nil {
		t.Fatal(err)
	}
	// D71: the template has the list of the accounts, empty; the human
	// adds an account with "a38 config account add".
	if !in.Present["backend.onepassword.accounts"] || in.Present["backend.onepassword.token_source"] {
		t.Errorf("template keys = %v, want backend.onepassword.accounts and no token_source", in.Present)
	}
}

func TestInspect(t *testing.T) {
	in, err := Inspect([]byte(commented))
	if err != nil {
		t.Fatal(err)
	}
	if !in.Present["grant.lifetime"] || in.Present["bundle.window"] || !in.Present["vaults"] {
		t.Errorf("present = %v", in.Present)
	}
	if got := in.Written["beispielprojekt"]; len(got) != 1 || got[0] != "/srv/beispielprojekt" {
		t.Errorf("written = %v", in.Written)
	}
}

func TestLocate(t *testing.T) {
	home := t.TempDir()
	tests := []struct {
		name, flag, env, xdg string
		wantPath             string
		wantSource           PathSource
	}{
		{"flag wins", "/x/flag.yaml", "/x/env.yaml", "/x/xdg", "/x/flag.yaml", SourceFlag},
		{"env", "", "/x/env.yaml", "/x/xdg", "/x/env.yaml", SourceEnv},
		{"env with tilde", "", "~/c.yaml", "", filepath.Join(home, "c.yaml"), SourceEnv},
		{"xdg", "", "", "/x/xdg", "/x/xdg/a38/config.yaml", SourceXDG},
		{"home", "", "", "", filepath.Join(home, ".config/a38/config.yaml"), SourceHome},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			t.Setenv(EnvConfig, tt.env)
			t.Setenv("XDG_CONFIG_HOME", tt.xdg)
			loc, err := Locate(tt.flag)
			if err != nil {
				t.Fatal(err)
			}
			if loc.Path != tt.wantPath || loc.Source != tt.wantSource || loc.Why == "" {
				t.Errorf("got %+v", loc)
			}
		})
	}
}

func TestWriteNew(t *testing.T) {
	// F15 (part): init never overwrites; --force moves the old file.
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "a", "b")
	path := filepath.Join(dir, "config.yaml")
	if _, err := WriteNew(path, []byte("one"), false, now); err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
	if _, err := WriteNew(path, []byte("two"), false, now); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "one" {
		t.Fatalf("file was overwritten: %s", b)
	}
	backup, err := WriteNew(path, []byte("two"), true, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(backup, "config.yaml.bak-20261112T100000Z") {
		t.Errorf("backup = %s", backup)
	}
	if b, _ := os.ReadFile(backup); string(b) != "one" {
		t.Errorf("backup content = %s", b)
	}
	if b, _ := os.ReadFile(path); string(b) != "two" {
		t.Errorf("new content = %s", b)
	}
}

func TestWriteSymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.yaml")
	link := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(target, []byte(commented), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteNew(link, []byte("x"), true, time.Now()); !errors.Is(err, ErrSymlink) {
		t.Errorf("WriteNew err = %v", err)
	}
	if err := Replace(link, []byte(commented), []byte("x")); !errors.Is(err, ErrSymlink) {
		t.Errorf("Replace err = %v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != commented {
		t.Error("the target changed")
	}
}

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	// A concurrent edit: the content is not the content that was read.
	if err := Replace(path, []byte("stale"), []byte("new")); !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}
	// A lock file of another edit.
	if err := os.WriteFile(path+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Replace(path, []byte("old"), []byte("new")); !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v, want ErrLocked", err)
	}
	_ = os.Remove(path + ".lock")
	if err := Replace(path, []byte("old"), []byte("new")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "new" {
		t.Errorf("content = %s", b)
	}
	assertMode(t, path, 0o640)
	left, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(left) != 1 {
		t.Errorf("files left over: %v", left)
	}
}

func TestReplaceReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	err := Replace(path, []byte("old"), []byte("new"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if b, _ := os.ReadFile(path); string(b) != "old" {
		t.Error("the file changed")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != want {
		t.Errorf("%s mode = %o, want %o", path, fi.Mode().Perm(), want)
	}
}

func TestAddProjectPath(t *testing.T) {
	// F15 (part), issue 92: a second path for an existing project, for
	// example the directory of the worktrees next to the repository.
	shortHome(t)
	tmpl, err := Template()
	if err != nil {
		t.Fatal(err)
	}
	// withVault is the template with a vault and a project that has an
	// empty-looking style; it shows that another style also works.
	tests := []struct {
		name    string
		src     string
		name2   string
		paths   []string
		wantErr string
		want    string
	}{
		{
			name: "appends after the last path and keeps the rest", src: commented, name2: "beispielprojekt",
			paths: []string{"/srv/beispielprojekt__worktrees"},
			want:  strings.Replace(commented, "      - /srv/beispielprojekt\n", "      - /srv/beispielprojekt\n      - /srv/beispielprojekt__worktrees\n", 1),
		},
		{
			name: "two paths in the sequence given", src: commented, name2: "beispielprojekt",
			paths: []string{"~/a", "/srv/b"},
			want:  strings.Replace(commented, "      - /srv/beispielprojekt\n", "      - /srv/beispielprojekt\n      - ~/a\n      - /srv/b\n", 1),
		},
		{name: "unknown project", src: commented, name2: "nope", paths: []string{"/srv/x"}, wantErr: "not in the list"},
		{name: "no path", src: commented, name2: "beispielprojekt", wantErr: "at least one path"},
		{name: "relative path", src: commented, name2: "beispielprojekt", paths: []string{"rel/dir"}, wantErr: "must be absolute"},
		{name: "path already in the project", src: commented, name2: "beispielprojekt", paths: []string{"/srv/beispielprojekt"}, wantErr: "already a path of the project"},
		{name: "path twice in the call", src: commented, name2: "beispielprojekt", paths: []string{"/srv/x", "/srv/x"}, wantErr: "twice"},
		{
			name:  "path of another project",
			src:   strings.Replace(commented, "# Grants.", "  - name: zweitprojekt\n    paths:\n      - /srv/zweit\n    vaults:\n      - beispiel.app.k8s-dev\n\n# Grants.", 1),
			name2: "beispielprojekt", paths: []string{"/srv/zweit"}, wantErr: "also belongs",
		},
		{name: "broken file", src: "version: 2\n", name2: "p", paths: []string{"/srv/x"}, wantErr: "fix it first"},
		{name: "template has no project", src: string(tmpl), name2: "p", paths: []string{"/srv/x"}, wantErr: "not in the list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddProjectPath([]byte(tt.src), tt.name2, tt.paths)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got.Data) != tt.want {
				t.Errorf("result:\n%s\nwant:\n%s", got.Data, tt.want)
			}
			if _, err := Parse(got.Data); err != nil {
				t.Errorf("result does not load: %v", err)
			}
		})
	}
}

func TestAddProjectPathMissingIsNote(t *testing.T) {
	// F15 (part): a path that does not exist is a note, as for "add".
	shortHome(t)
	got, err := AddProjectPath([]byte(commented), "beispielprojekt", []string{"/nonexistent-a38/wt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "/nonexistent-a38/wt") {
		t.Errorf("notes = %v", got.Notes)
	}
}
