package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/protocol"
)

// minimal is the smallest valid configuration.
const minimal = "version: 1\n"

// example is a complete configuration with neutral test data.
const example = `version: 1
vaults:
  - name: beispiel.app.k8s-dev
    environment: dev
  - name: beispiel.app.k8s-test
    environment: test
  - name: beispiel.app.k8s-prod
    environment: prod
projects:
  - name: beispielprojekt
    paths:
      - /home/beispiel/develop/beispielprojekt
      - ~/develop/beispielprojekt__worktrees/
    vaults:
      - beispiel.app.k8s-dev
  - name: zweitprojekt
    paths:
      - /home/beispiel/develop/zweitprojekt
    vaults:
      - beispiel.app.k8s-dev
      - beispiel.app.k8s-test
grant:
  lifetime: 30m
bundle:
  window: 1500ms
  max_entries: 3
session_name:
  max_length: 20
  allowed_characters: "abcdefghijklmnopqrstuvwxyz-."
paths:
  socket_dir: /home/beispiel/a38/run
  audit_file: ~/a38/audit.db
backend:
  onepassword:
    token_source: file
    token_file: ~/a38/onepassword-token
`

// setHome gives the test a fixed home directory and no XDG variables.
func setHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", "/home/beispiel")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("A38_CONFIG", "")
}

func TestParseDefaults(t *testing.T) {
	// T4 (part): without a value the grant lifetime is four hours.
	// T19 (part): the defaults of the bundle are three seconds and ten entries.
	// T21 (part): the default rules for the session name.
	setHome(t)
	cfg, err := Parse([]byte(minimal))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := &Config{
		Grant:  Grant{Lifetime: 4 * time.Hour},
		Bundle: Bundle{Window: 3 * time.Second, MaxEntries: 10},
		SessionName: SessionNameRules{
			MaxLength:         80,
			AllowedCharacters: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 -_():",
		},
		Paths: Paths{
			SocketDir: "/home/beispiel/.local/state/a38/run",
			AuditFile: "/home/beispiel/.local/state/a38/audit.db",
		},
		// D51, issue 59: the keychain is the default source; the token
		// file keeps its default path for the source file.
		Backend: Backend{OnePassword: OnePassword{TokenSource: TokenSourceKeychain, TokenFile: "/home/beispiel/.config/a38/onepassword-token"}},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Parse(minimal) =\n%+v\nwant\n%+v", cfg, want)
	}
	if got, want := cfg.AgentSocket(), "/home/beispiel/.local/state/a38/run/agent.sock"; got != want {
		t.Errorf("AgentSocket() = %q, want %q", got, want)
	}
	if got, want := cfg.AdminSocket(), "/home/beispiel/.local/state/a38/run/admin.sock"; got != want {
		t.Errorf("AdminSocket() = %q, want %q", got, want)
	}
}

func TestParseExample(t *testing.T) {
	setHome(t)
	cfg, err := Parse([]byte(example))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := &Config{
		Vaults: []Vault{
			{Name: "beispiel.app.k8s-dev", Environment: EnvDev},
			{Name: "beispiel.app.k8s-test", Environment: EnvTest},
			{Name: "beispiel.app.k8s-prod", Environment: EnvProd},
		},
		Projects: []Project{
			{Name: "beispielprojekt",
				Paths:        []string{"/home/beispiel/develop/beispielprojekt", "/home/beispiel/develop/beispielprojekt__worktrees"},
				RealPaths:    []string{"/home/beispiel/develop/beispielprojekt", "/home/beispiel/develop/beispielprojekt__worktrees"},
				MissingPaths: []string{"/home/beispiel/develop/beispielprojekt", "/home/beispiel/develop/beispielprojekt__worktrees"},
				Vaults:       []string{"beispiel.app.k8s-dev"}},
			{Name: "zweitprojekt",
				Paths:        []string{"/home/beispiel/develop/zweitprojekt"},
				RealPaths:    []string{"/home/beispiel/develop/zweitprojekt"},
				MissingPaths: []string{"/home/beispiel/develop/zweitprojekt"},
				Vaults:       []string{"beispiel.app.k8s-dev", "beispiel.app.k8s-test"}},
		},
		Grant:       Grant{Lifetime: 30 * time.Minute},
		Bundle:      Bundle{Window: 1500 * time.Millisecond, MaxEntries: 3},
		SessionName: SessionNameRules{MaxLength: 20, AllowedCharacters: "abcdefghijklmnopqrstuvwxyz-."},
		Paths:       Paths{SocketDir: "/home/beispiel/a38/run", AuditFile: "/home/beispiel/a38/audit.db"},
		Backend:     Backend{OnePassword: OnePassword{TokenSource: TokenSourceFile, TokenFile: "/home/beispiel/a38/onepassword-token"}},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Parse(example) =\n%+v\nwant\n%+v", cfg, want)
	}
	if len(cfg.InvalidProjects()) != 0 {
		t.Errorf("InvalidProjects() = %v, want none", cfg.InvalidProjects())
	}
}

func TestParseAccepts(t *testing.T) {
	setHome(t)
	tests := []struct {
		name  string
		yaml  string
		check func(*testing.T, *Config)
	}{
		// T4 (part): a lifetime of 30 minutes and of twelve hours is valid.
		{"lifetime 30 minutes", minimal + "grant:\n  lifetime: 30m\n", func(t *testing.T, c *Config) {
			if c.Grant.Lifetime != 30*time.Minute {
				t.Errorf("lifetime = %v", c.Grant.Lifetime)
			}
		}},
		{"lifetime twelve hours", minimal + "grant:\n  lifetime: 12h\n", func(t *testing.T, c *Config) {
			if c.Grant.Lifetime != 12*time.Hour {
				t.Errorf("lifetime = %v", c.Grant.Lifetime)
			}
		}},
		// T14 (part): 50 is the largest valid bundle maximum.
		{"bundle maximum 50", minimal + "bundle:\n  max_entries: 50\n", func(t *testing.T, c *Config) {
			if c.Bundle.MaxEntries != MaxBundleEntries {
				t.Errorf("bundle = %+v", c.Bundle)
			}
		}},
		// T19 (part): the test runs with a bundle maximum of three.
		{"bundle maximum three", minimal + "bundle:\n  max_entries: 3\n", func(t *testing.T, c *Config) {
			if c.Bundle.MaxEntries != 3 || c.Bundle.Window != 3*time.Second {
				t.Errorf("bundle = %+v", c.Bundle)
			}
		}},
		// T22 (part): the name of a vault says nothing about its environment.
		{"vault name against environment", minimal + "vaults:\n  - {name: beispiel-prod, environment: dev}\n  - {name: beispiel-dev, environment: prod}\n", func(t *testing.T, c *Config) {
			if v, ok := c.Vault("beispiel-prod"); !ok || v.Environment != EnvDev {
				t.Errorf("Vault(beispiel-prod) = %+v, %v", v, ok)
			}
			if v, ok := c.Vault("beispiel-dev"); !ok || v.Environment != EnvProd {
				t.Errorf("Vault(beispiel-dev) = %+v, %v", v, ok)
			}
			if _, ok := c.Vault("k8s-beispiel"); ok {
				t.Error("Vault(k8s-beispiel) found a vault that is not in the configuration")
			}
			if _, ok := c.Vault("Beispiel-Prod"); ok {
				t.Error("Vault matches without regard to case")
			}
		}},
		{"project without vaults", minimal + "projects:\n  - name: beispielprojekt\n    paths: [/home/beispiel/develop/beispielprojekt]\n", func(t *testing.T, c *Config) {
			if p, ok := c.Project("beispielprojekt"); !ok || p.Invalid != "" || len(p.Vaults) != 0 {
				t.Errorf("Project = %+v, %v", p, ok)
			}
		}},
		// T21 (part): the configuration can list a control character; the
		// rules reject it in a name in each case.
		{"control character in the allowed set", minimal + "session_name:\n  allowed_characters: \"abc\\n\"\n", func(t *testing.T, c *Config) {
			if err := c.SessionName.Check("ab\nc"); err == nil {
				t.Error("Check accepted a name with a line break")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestParseRejects(t *testing.T) {
	setHome(t)
	vault := func(body string) string { return minimal + "vaults:\n" + body }
	project := func(body string) string {
		return vault("  - {name: beispiel.app.k8s-dev, environment: dev}\n") + "projects:\n" + body
	}
	tests := []struct {
		name string
		yaml string
		// wantText must be part of the error message.
		wantText string
	}{
		{"empty file", "", "version"},
		{"version missing", "vaults: []\n", "version"},
		{"version 2", "version: 2\n", "version"},
		{"not YAML", "version: [1\n", ""},
		{"not a mapping", "- version\n", ""},
		{"unknown key", minimal + "vault: []\n", "vault"},
		{"unknown key in a vault", vault("  - {name: beispiel.app.k8s-dev, environment: dev, pattern: k8s}\n"), "pattern"},
		{"two documents", minimal + "---\nversion: 1\n", "one YAML document"},
		{"relative token file", minimal + "backend:\n  onepassword:\n    token_file: token\n", "backend.onepassword.token_file"},
		{"empty token file", minimal + "backend:\n  onepassword:\n    token_file: \"\"\n", "backend.onepassword.token_file"},
		{"unknown backend key", minimal + "backend:\n  onepassword:\n    token: x\n", "token"},
		// D51, issue 59: only keychain and file are token sources.
		{"unknown token source", minimal + "backend:\n  onepassword:\n    token_source: env\n", "backend.onepassword.token_source"},
		{"token source in capitals", minimal + "backend:\n  onepassword:\n    token_source: Keychain\n", "backend.onepassword.token_source"},
		{"empty token source", minimal + "backend:\n  onepassword:\n    token_source: \"\"\n", "backend.onepassword.token_source"},

		// T22 (part): cases in which the daemon does not start. The message
		// names the vault.
		{"vault without environment", vault("  - name: beispiel.app.k8s-dev\n"), `"beispiel.app.k8s-dev"`},
		{"environment staging", vault("  - {name: beispiel.app.k8s-dev, environment: staging}\n"), `"beispiel.app.k8s-dev"`},
		{"environment empty", vault("  - {name: beispiel.app.k8s-dev, environment: \"\"}\n"), `"beispiel.app.k8s-dev"`},
		{"environment Dev", vault("  - {name: beispiel.app.k8s-dev, environment: Dev}\n"), `"beispiel.app.k8s-dev"`},
		{"environment null", vault("  - {name: beispiel.app.k8s-dev, environment: null}\n"), `"beispiel.app.k8s-dev"`},
		{"vault twice with different environments", vault("  - {name: beispiel.app.k8s-dev, environment: dev}\n  - {name: beispiel.app.k8s-dev, environment: prod}\n"), `"beispiel.app.k8s-dev"`},
		{"vault twice with the same environment", vault("  - {name: beispiel.app.k8s-dev, environment: dev}\n  - {name: beispiel.app.k8s-dev, environment: dev}\n"), `"beispiel.app.k8s-dev"`},
		{"vault without name", vault("  - {environment: dev}\n"), "name"},
		{"vault name with a slash", vault("  - {name: beispiel/dev, environment: dev}\n"), "slash"},
		{"vault name with a control character", vault("  - {name: \"beispiel\\tdev\", environment: dev}\n"), "control character"},

		{"project without name", project("  - paths: [/home/beispiel/develop/beispielprojekt]\n"), "name"},
		{"project twice", project("  - {name: beispielprojekt, paths: [/home/beispiel/a]}\n  - {name: beispielprojekt, paths: [/home/beispiel/b]}\n"), `"beispielprojekt"`},
		{"project without paths", project("  - {name: beispielprojekt, vaults: [beispiel.app.k8s-dev]}\n"), `"beispielprojekt"`},
		{"project with a relative path", project("  - {name: beispielprojekt, paths: [develop/beispielprojekt]}\n"), "absolute"},
		{"project with the root directory", project("  - {name: beispielprojekt, paths: [/]}\n"), "root"},
		{"path in two projects", project("  - {name: beispielprojekt, paths: [/home/beispiel/a]}\n  - {name: zweitprojekt, paths: [/home/beispiel/a/]}\n"), `"/home/beispiel/a"`},
		{"project with an empty vault name", project("  - {name: beispielprojekt, paths: [/home/beispiel/a], vaults: [\"\"]}\n"), `"beispielprojekt"`},

		// T4 (part): a lifetime of 0, a negative value or text: no start.
		{"lifetime 0", minimal + "grant:\n  lifetime: 0\n", "grant.lifetime"},
		{"lifetime 0s", minimal + "grant:\n  lifetime: 0s\n", "grant.lifetime"},
		{"lifetime negative", minimal + "grant:\n  lifetime: -4h\n", "grant.lifetime"},
		{"lifetime text", minimal + "grant:\n  lifetime: lange\n", "grant.lifetime"},
		{"lifetime without unit", minimal + "grant:\n  lifetime: 4\n", "grant.lifetime"},
		{"lifetime empty", minimal + "grant:\n  lifetime:\n", "grant.lifetime"},
		{"lifetime is a list", minimal + "grant:\n  lifetime: [4h]\n", "grant.lifetime"},

		// T19 (part): a bundle window of 0 or a maximum of 0: no start.
		{"window 0", minimal + "bundle:\n  window: 0s\n", "bundle.window"},
		{"window negative", minimal + "bundle:\n  window: -3s\n", "bundle.window"},
		{"window text", minimal + "bundle:\n  window: kurz\n", "bundle.window"},
		// T14 (part): the approval app rejects more than 50 entries.
		{"maximum 51", minimal + "bundle:\n  max_entries: 51\n", "bundle.max_entries: the value must be 50 or less"},
		{"maximum huge", minimal + "bundle:\n  max_entries: 100000\n", "bundle.max_entries"},
		{"maximum 0", minimal + "bundle:\n  max_entries: 0\n", "bundle.max_entries"},
		{"maximum negative", minimal + "bundle:\n  max_entries: -1\n", "bundle.max_entries"},
		{"maximum text", minimal + "bundle:\n  max_entries: viele\n", "max_entries"},
		{"maximum empty", minimal + "bundle:\n  max_entries:\n", "bundle.max_entries"},

		// T21 (part): a maximum length of 0 or an empty character set: no start.
		{"name length 0", minimal + "session_name:\n  max_length: 0\n", "session_name.max_length"},
		{"name length negative", minimal + "session_name:\n  max_length: -1\n", "session_name.max_length"},
		{"name length above the protocol limit", minimal + "session_name:\n  max_length: 257\n", "session_name.max_length"},
		{"name characters empty", minimal + "session_name:\n  allowed_characters: \"\"\n", "session_name.allowed_characters"},
		{"name characters null", minimal + "session_name:\n  allowed_characters:\n", "session_name.allowed_characters"},

		{"relative socket directory", minimal + "paths:\n  socket_dir: run\n", "paths.socket_dir"},
		{"relative audit file", minimal + "paths:\n  audit_file: audit.db\n", "paths.audit_file"},
		{"socket path too long", minimal + "paths:\n  socket_dir: /home/beispiel/" + strings.Repeat("a", 100) + "\n", "paths.socket_dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatalf("Parse accepted the configuration: %+v", cfg)
			}
			if cfg != nil {
				t.Error("Parse returned a configuration together with an error")
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantText)
			}
		})
	}
}

func TestParseReportsAllErrors(t *testing.T) {
	setHome(t)
	_, err := Parse([]byte(minimal + "vaults:\n  - name: beispiel.app.k8s-dev\n  - {name: beispiel.app.k8s-test, environment: staging}\ngrant:\n  lifetime: 0s\n"))
	if err == nil {
		t.Fatal("Parse accepted the configuration")
	}
	for _, want := range []string{`"beispiel.app.k8s-dev"`, `"beispiel.app.k8s-test"`, "grant.lifetime"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

func TestInvalidProject(t *testing.T) {
	// T22 (part): a project that names a vault that is not in the vault
	// list does not stop the start. Only this project is invalid.
	setHome(t)
	cfg, err := Parse([]byte(minimal + `vaults:
  - {name: beispiel.app.k8s-dev, environment: dev}
projects:
  - name: projekt-a
    paths: [/home/beispiel/develop/projekt-a]
    vaults: [beispiel.app.k8s-dev, beispiel.infra.k8s-dev]
  - name: projekt-b
    paths: [/home/beispiel/develop/projekt-b]
    vaults: [beispiel.app.k8s-dev]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a, ok := cfg.Project("projekt-a")
	if !ok || a.Invalid == "" {
		t.Fatalf("Project(projekt-a) = %+v, %v; want an invalid project", a, ok)
	}
	if !strings.Contains(a.Invalid, `"beispiel.infra.k8s-dev"`) {
		t.Errorf("Invalid = %q, want it to name the missing vault", a.Invalid)
	}
	if a.Allows("beispiel.app.k8s-dev") {
		t.Error("an invalid project allows a vault")
	}
	b, ok := cfg.Project("projekt-b")
	if !ok || b.Invalid != "" {
		t.Errorf("Project(projekt-b) = %+v, %v; want a valid project", b, ok)
	}
	if !b.Allows("beispiel.app.k8s-dev") || b.Allows("beispiel.infra.k8s-dev") || b.Allows("") {
		t.Error("Allows of the valid project is wrong")
	}
	invalid := cfg.InvalidProjects()
	if len(invalid) != 1 || invalid[0].Name != "projekt-a" {
		t.Errorf("InvalidProjects() = %+v, want only projekt-a", invalid)
	}
	if got, ok := cfg.ProjectFor("/home/beispiel/develop/projekt-a/cmd"); !ok || got.Name != "projekt-a" || got.Invalid == "" {
		t.Errorf("ProjectFor finds %+v, %v; want the invalid project", got, ok)
	}
}

func TestProjectFor(t *testing.T) {
	// T11 (part): the project follows from the working directory.
	setHome(t)
	cfg, err := Parse([]byte(minimal + `projects:
  - name: beispielprojekt
    paths:
      - /home/beispiel/develop/beispielprojekt
      - /home/beispiel/develop/beispielprojekt__worktrees
  - name: teilprojekt
    paths: [/home/beispiel/develop/beispielprojekt/teil]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tests := []struct {
		dir  string
		want string
	}{
		{"/home/beispiel/develop/beispielprojekt", "beispielprojekt"},
		{"/home/beispiel/develop/beispielprojekt/", "beispielprojekt"},
		{"/home/beispiel/develop/beispielprojekt/internal/protocol", "beispielprojekt"},
		{"/home/beispiel/develop/beispielprojekt__worktrees/feat-16-architektur", "beispielprojekt"},
		{"/home/beispiel/develop/beispielprojekt/teil", "teilprojekt"},
		{"/home/beispiel/develop/beispielprojekt/teil/a/b", "teilprojekt"},
		{"/home/beispiel/develop/beispielprojekt/teilweise", "beispielprojekt"},
		{"/home/beispiel/develop/beispielprojekt-kopie", ""},
		{"/home/beispiel/develop", ""},
		{"/home/beispiel/develop/beispielprojekt/../zweitprojekt", ""},
		{"/", ""},
		{"", ""},
		{"develop/beispielprojekt", ""},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			got, ok := cfg.ProjectFor(tt.dir)
			if ok != (tt.want != "") || got.Name != tt.want {
				t.Errorf("ProjectFor(%q) = %q, %v; want %q", tt.dir, got.Name, ok, tt.want)
			}
		})
	}
}

func TestSessionNameCheck(t *testing.T) {
	// T21 (part): length and characters of the session name.
	setHome(t)
	defaults, err := Parse([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	custom, err := Parse([]byte(minimal + "session_name:\n  max_length: 20\n  allowed_characters: \"abcdefghijklmnopqrstuvwxyz-.\\n\\u202e\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	name80 := "beispiel: koordinator (2) " + strings.Repeat("A_9-", 13) + "ab"
	if len(name80) != 80 {
		t.Fatalf("test data: name80 has %d characters", len(name80))
	}
	tests := []struct {
		name    string
		rules   SessionNameRules
		in      string
		wantErr bool
	}{
		{"usual name", defaults.SessionName, "beispiel-koordinator", false},
		{"name with suffix", defaults.SessionName, "beispiel-koordinator-7f3a", false},
		{"all character classes", defaults.SessionName, "beispiel: koordinator (2)", false},
		{"no name", defaults.SessionName, "", false},
		{"80 characters", defaults.SessionName, name80, false},
		{"81 characters", defaults.SessionName, name80 + "a", true},
		{"dot", defaults.SessionName, "beispiel.koordinator", true},
		{"umlaut", defaults.SessionName, "beispiel-prüfer", true},
		{"slash", defaults.SessionName, "beispiel/koordinator", true},
		{"line break", defaults.SessionName, "beispiel\nkoordinator", true},
		{"escape sequence", defaults.SessionName, "beispiel\x1b[31mkoordinator", true},
		{"NUL", defaults.SessionName, "beispiel\x00koordinator", true},
		{"direction mark", defaults.SessionName, "beispiel\u202ekoordinator", true},
		{"invalid UTF-8", defaults.SessionName, "beispiel\xffkoordinator", true},
		{"custom: dot is allowed", custom.SessionName, "beispiel.koordinator", false},
		{"custom: 20 characters", custom.SessionName, "beispiel-koordinator", false},
		{"custom: 21 characters", custom.SessionName, "beispiel-koordinatorr", true},
		{"custom: upper case is not in the set", custom.SessionName, "Beispiel", true},
		{"custom: line break stays forbidden", custom.SessionName, "beispiel\nkoord", true},
		{"custom: direction mark stays forbidden", custom.SessionName, "beispiel\u202ekoord", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rules.Check(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Check(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err != nil {
				if got := protocol.CodeOf(err); got != protocol.CodeInvalidRequest {
					t.Errorf("code = %q, want %q", got, protocol.CodeInvalidRequest)
				}
				if tt.in != "" && strings.Contains(err.Error(), tt.in) {
					t.Errorf("the error repeats the name: %v", err)
				}
			}
		})
	}
}

func TestLoad(t *testing.T) {
	setHome(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(example), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Vaults) != 3 {
		t.Errorf("Load read %d vaults, want 3", len(cfg.Vaults))
	}
	missing := filepath.Join(dir, "fehlt.yaml")
	if _, err := Load(missing); err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("Load of a missing file: error = %v, want it to name the file", err)
	}
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("version: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), bad) {
		t.Errorf("Load of an invalid file: error = %v, want it to name the file", err)
	}
}

func TestDefaultPaths(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		wantConfig string
		wantSocket string
		wantAudit  string
	}{
		{"home only", map[string]string{"HOME": "/home/beispiel"},
			"/home/beispiel/.config/a38/config.yaml", "/home/beispiel/.local/state/a38/run", "/home/beispiel/.local/state/a38/audit.db"},
		{"XDG directories", map[string]string{"HOME": "/home/beispiel", "XDG_CONFIG_HOME": "/etc/beispiel", "XDG_STATE_HOME": "/var/beispiel"},
			"/etc/beispiel/a38/config.yaml", "/var/beispiel/a38/run", "/var/beispiel/a38/audit.db"},
		{"relative XDG directories are ignored", map[string]string{"HOME": "/home/beispiel", "XDG_CONFIG_HOME": "etc", "XDG_STATE_HOME": "var"},
			"/home/beispiel/.config/a38/config.yaml", "/home/beispiel/.local/state/a38/run", "/home/beispiel/.local/state/a38/audit.db"},
		{"A38_CONFIG", map[string]string{"HOME": "/home/beispiel", "A38_CONFIG": "/home/beispiel/test/config.yaml"},
			"/home/beispiel/test/config.yaml", "/home/beispiel/.local/state/a38/run", "/home/beispiel/.local/state/a38/audit.db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "A38_CONFIG"} {
				t.Setenv(key, tt.env[key])
			}
			if got, err := DefaultPath(); err != nil || got != tt.wantConfig {
				t.Errorf("DefaultPath() = %q, %v; want %q", got, err, tt.wantConfig)
			}
			if got, err := DefaultSocketDir(); err != nil || got != tt.wantSocket {
				t.Errorf("DefaultSocketDir() = %q, %v; want %q", got, err, tt.wantSocket)
			}
			if got, err := DefaultAuditFile(); err != nil || got != tt.wantAudit {
				t.Errorf("DefaultAuditFile() = %q, %v; want %q", got, err, tt.wantAudit)
			}
			// G3: the token file lies next to the default configuration
			// file, also if A38_CONFIG names another file.
			wantToken := filepath.Join(filepath.Dir(tt.wantConfig), "onepassword-token")
			if tt.env["A38_CONFIG"] != "" {
				wantToken = "/home/beispiel/.config/a38/onepassword-token"
			}
			if got, err := DefaultTokenFile(); err != nil || got != wantToken {
				t.Errorf("DefaultTokenFile() = %q, %v; want %q", got, err, wantToken)
			}
		})
	}
}

func TestEnvironmentValid(t *testing.T) {
	for _, env := range []Environment{EnvDev, EnvTest, EnvProd} {
		if !env.Valid() {
			t.Errorf("%q.Valid() = false", env)
		}
	}
	for _, env := range []Environment{"", "Dev", "staging", "DEV", " dev"} {
		if env.Valid() {
			t.Errorf("%q.Valid() = true", env)
		}
	}
}

// T11 (part): a project path under a symbolic link matches the resolved
// working directory of a session (issue 44).
func TestProjectPathsUnderSymlink(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"projekt/teil", "tief/projekt"} {
		if err := os.MkdirAll(filepath.Join(real, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse([]byte(minimal + `projects:
  - name: beispielprojekt
    paths: [` + link + `/projekt]
  - name: teilprojekt
    paths: [` + real + `/projekt/teil]
  - name: fehlt
    paths: [` + link + `/gibt-es-nicht]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tests := []struct {
		dir, want string
	}{
		{real + "/projekt", "beispielprojekt"},
		{real + "/projekt/src", "beispielprojekt"},
		{real + "/projekt/teil/x", "teilprojekt"}, // longest match wins
		{real + "/tief/projekt", ""},
		{link + "/gibt-es-nicht/x", "fehlt"}, // not resolvable: kept as written
	}
	for _, tt := range tests {
		got, ok := cfg.ProjectFor(tt.dir)
		if ok != (tt.want != "") || got.Name != tt.want {
			t.Errorf("ProjectFor(%q) = %q, %v; want %q", tt.dir, got.Name, ok, tt.want)
		}
	}
	p, _ := cfg.Project("beispielprojekt")
	if p.Paths[0] != link+"/projekt" {
		t.Errorf("Paths = %v; want the path as written", p.Paths)
	}
	if len(p.MissingPaths) != 0 {
		t.Errorf("MissingPaths = %v; want none", p.MissingPaths)
	}
	m, _ := cfg.Project("fehlt")
	if !reflect.DeepEqual(m.MissingPaths, []string{link + "/gibt-es-nicht"}) || m.Invalid != "" {
		t.Errorf("MissingPaths = %v, Invalid = %q; want the path and a valid project", m.MissingPaths, m.Invalid)
	}
}

// T11 (part): the configured path is the real path and the session
// directory comes from a symbolic link that the daemon resolved.
func TestProjectPathRealSessionResolved(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse([]byte(minimal + "projects:\n  - name: beispielprojekt\n    paths: [" + real + "]\n"))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := cfg.ProjectFor(resolved); !ok || got.Name != "beispielprojekt" {
		t.Errorf("ProjectFor(%q) = %q, %v", resolved, got.Name, ok)
	}
}

// T11 (part): on macOS /tmp is a link to /private/tmp.
func TestProjectPathTmp(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse([]byte(minimal + "projects:\n  - name: beispielprojekt\n    paths: [" + dir + "]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := cfg.ProjectFor(resolved + "/src"); !ok || got.Name != "beispielprojekt" {
		t.Errorf("ProjectFor(%q) = %q, %v", resolved, got.Name, ok)
	}
}
