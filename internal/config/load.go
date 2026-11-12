package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/seletz/passierschein/internal/protocol"
)

// EnvConfig is the environment variable that names the configuration file.
// The flag "--config" of a command has priority.
const EnvConfig = "A38_CONFIG"

// rawFile is the YAML form of the file. A yaml.Node field keeps the
// difference between "key is absent" (use the default) and "key has an
// empty or wrong value" (fail).
type rawFile struct {
	Version  yaml.Node    `yaml:"version"`
	Vaults   []rawVault   `yaml:"vaults"`
	Projects []rawProject `yaml:"projects"`
	Grant    struct {
		Lifetime yaml.Node `yaml:"lifetime"`
	} `yaml:"grant"`
	Bundle struct {
		Window     yaml.Node `yaml:"window"`
		MaxEntries yaml.Node `yaml:"max_entries"`
	} `yaml:"bundle"`
	SessionName struct {
		MaxLength         yaml.Node `yaml:"max_length"`
		AllowedCharacters yaml.Node `yaml:"allowed_characters"`
	} `yaml:"session_name"`
	Paths struct {
		SocketDir yaml.Node `yaml:"socket_dir"`
		AuditFile yaml.Node `yaml:"audit_file"`
		UIApp     yaml.Node `yaml:"ui_app"`
	} `yaml:"paths"`
}

type rawVault struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
}

type rawProject struct {
	Name   string   `yaml:"name"`
	Paths  []string `yaml:"paths"`
	Vaults []string `yaml:"vaults"`
}

// Load reads and checks the configuration file at path. An error names the
// file and each finding.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read the configuration file: %w", err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: the configuration file %s is not valid:\n%w", path, err)
	}
	return cfg, nil
}

// Parse checks the content of a configuration file and returns the
// configuration with all defaults set. The error lists each finding on its
// own line.
func Parse(data []byte) (*Config, error) {
	var raw rawFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("the file is not valid YAML for this format: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("the file must contain exactly one YAML document")
	}

	var findings []error
	fail := func(format string, args ...any) {
		findings = append(findings, fmt.Errorf(format, args...))
	}

	if version, err := intValue(raw.Version, 0); err != nil || version != FileVersion {
		fail("version: the value must be %d", FileVersion)
	}

	cfg := &Config{}
	cfg.Vaults = parseVaults(raw.Vaults, fail)
	cfg.Projects = parseProjects(raw.Projects, cfg, fail)

	var err error
	if cfg.Grant.Lifetime, err = durationValue(raw.Grant.Lifetime, DefaultGrantLifetime); err != nil {
		fail("grant.lifetime: %v", err)
	}
	if cfg.Bundle.Window, err = durationValue(raw.Bundle.Window, DefaultBundleWindow); err != nil {
		fail("bundle.window: %v", err)
	}
	if cfg.Bundle.MaxEntries, err = intValue(raw.Bundle.MaxEntries, DefaultBundleMaxEntries); err != nil {
		fail("bundle.max_entries: %v", err)
	} else if cfg.Bundle.MaxEntries < 1 {
		fail("bundle.max_entries: the value must be 1 or more")
	}
	if cfg.SessionName.MaxLength, err = intValue(raw.SessionName.MaxLength, DefaultSessionNameMaxLength); err != nil {
		fail("session_name.max_length: %v", err)
	} else if cfg.SessionName.MaxLength < 1 || cfg.SessionName.MaxLength > protocol.MaxSessionNameLength {
		fail("session_name.max_length: the value must be 1 to %d", protocol.MaxSessionNameLength)
	}
	if cfg.SessionName.AllowedCharacters, err = stringValue(raw.SessionName.AllowedCharacters, DefaultSessionNameCharacters); err != nil {
		fail("session_name.allowed_characters: %v", err)
	}

	cfg.Paths.SocketDir = pathValue(raw.Paths.SocketDir, DefaultSocketDir, "paths.socket_dir", fail)
	cfg.Paths.AuditFile = pathValue(raw.Paths.AuditFile, DefaultAuditFile, "paths.audit_file", fail)
	// An empty UIApp means: search next to the a38 binary.
	cfg.Paths.UIApp = pathValue(raw.Paths.UIApp, func() (string, error) { return "", nil }, "paths.ui_app", fail)
	if cfg.Paths.SocketDir != "" && len(cfg.AgentSocket()) > maxSocketPath {
		fail("paths.socket_dir: the path of a socket is longer than %d bytes", maxSocketPath)
	}

	if len(findings) > 0 {
		return nil, errors.Join(findings...)
	}
	return cfg, nil
}

// parseVaults checks the vault list.
func parseVaults(raw []rawVault, fail func(string, ...any)) []Vault {
	var vaults []Vault
	seen := map[string]bool{}
	for i, rv := range raw {
		if err := checkName(rv.Name); err != nil {
			fail("vaults[%d]: the name %v", i, err)
			continue
		}
		env := Environment(rv.Environment)
		switch {
		case rv.Environment == "":
			fail("vault %q: the environment is missing; use dev, test or prod", rv.Name)
		case !env.Valid():
			fail("vault %q: the environment is not dev, test or prod", rv.Name)
		}
		if seen[rv.Name] {
			fail("vault %q: the vault is in the list more than once", rv.Name)
		}
		seen[rv.Name] = true
		vaults = append(vaults, Vault{Name: rv.Name, Environment: env})
	}
	return vaults
}

// parseProjects checks the projects. A vault that is not in the vault list
// marks the project as invalid; it is not a finding that stops loading.
func parseProjects(raw []rawProject, cfg *Config, fail func(string, ...any)) []Project {
	var projects []Project
	names := map[string]bool{}
	owner := map[string]string{}
	for i, rp := range raw {
		if err := checkName(rp.Name); err != nil {
			fail("projects[%d]: the name %v", i, err)
			continue
		}
		if names[rp.Name] {
			fail("project %q: the project is in the list more than once", rp.Name)
		}
		names[rp.Name] = true
		p := Project{Name: rp.Name}
		if len(rp.Paths) == 0 {
			fail("project %q: the project has no paths", rp.Name)
		}
		for _, raw := range rp.Paths {
			path, err := absPath(raw)
			switch {
			case err != nil:
				fail("project %q: a path %v", rp.Name, err)
				continue
			case path == string(filepath.Separator):
				fail("project %q: the root directory cannot be a project path", rp.Name)
				continue
			}
			if other, taken := owner[path]; taken && other != rp.Name {
				fail("project %q: the path %q also belongs to the project %q", rp.Name, path, other)
			}
			owner[path] = rp.Name
			p.Paths = append(p.Paths, path)
			// The daemon resolves the working directory of a session, so
			// resolve the project path too. A path that does not exist
			// stays as written and does not fail loading (issue 44).
			real, rerr := filepath.EvalSymlinks(path)
			if rerr != nil {
				real = path
				p.MissingPaths = append(p.MissingPaths, path)
			}
			p.RealPaths = append(p.RealPaths, real)
		}
		var missing []string
		for _, vault := range rp.Vaults {
			if err := checkName(vault); err != nil {
				fail("project %q: the name of a vault %v", rp.Name, err)
				continue
			}
			if _, ok := cfg.Vault(vault); !ok {
				missing = append(missing, strconv.Quote(vault))
			}
			p.Vaults = append(p.Vaults, vault)
		}
		if len(missing) > 0 {
			p.Invalid = fmt.Sprintf("The project %q names vaults that are not in the vault list: %s.",
				rp.Name, strings.Join(missing, ", "))
		}
		projects = append(projects, p)
	}
	return projects
}

// checkName checks the name of a vault or a project. The text completes
// the sentence "the name ...".
func checkName(name string) error {
	switch {
	case name == "":
		return errors.New("is missing or empty")
	case strings.Contains(name, "/"):
		return errors.New("contains a slash")
	case strings.ContainsFunc(name, protocol.IsControl):
		return errors.New("contains a control character")
	case strings.TrimSpace(name) != name:
		return errors.New("starts or ends with white space")
	}
	return nil
}

// scalar returns the text of a scalar node. present is false if the key is
// absent from the file.
func scalar(n yaml.Node) (text string, present bool, err error) {
	if n.Kind == 0 {
		return "", false, nil
	}
	if n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return "", true, errors.New("the value is empty or is not a single value")
	}
	return n.Value, true, nil
}

// durationValue reads a positive duration such as "4h", "30m" or "3s".
func durationValue(n yaml.Node, def time.Duration) (time.Duration, error) {
	text, present, err := scalar(n)
	if err != nil || !present {
		return def, err
	}
	d, err := time.ParseDuration(text)
	if err != nil {
		return 0, errors.New(`the value must be a duration with a unit, for example "4h", "30m" or "3s"`)
	}
	if d <= 0 {
		return 0, errors.New("the duration must be more than zero")
	}
	return d, nil
}

// intValue reads a whole number.
func intValue(n yaml.Node, def int) (int, error) {
	text, present, err := scalar(n)
	if err != nil || !present {
		return def, err
	}
	v, err := strconv.Atoi(text)
	if err != nil || n.Tag != "!!int" {
		return 0, errors.New("the value must be a whole number")
	}
	return v, nil
}

// stringValue reads a text that is not empty.
func stringValue(n yaml.Node, def string) (string, error) {
	text, present, err := scalar(n)
	if err != nil || !present {
		return def, err
	}
	if text == "" {
		return "", errors.New("the value is empty")
	}
	return text, nil
}

// pathValue reads an absolute path or takes the default.
func pathValue(n yaml.Node, def func() (string, error), key string, fail func(string, ...any)) string {
	text, present, err := scalar(n)
	if err != nil {
		fail("%s: %v", key, err)
		return ""
	}
	if !present {
		path, err := def()
		if err != nil {
			fail("%s: no default: %v", key, err)
		}
		return path
	}
	path, err := absPath(text)
	if err != nil {
		fail("%s: the path %v", key, err)
	}
	return path
}

// absPath returns the clean form of an absolute path. A leading "~/" is
// the home directory. The text completes the sentence "the path ...".
func absPath(path string) (string, error) {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, rest)
	}
	if strings.ContainsFunc(path, protocol.IsControl) {
		return "", errors.New("contains a control character")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New(`must be absolute or start with "~/"`)
	}
	return filepath.Clean(path), nil
}

// homeDir returns the home directory of the user.
func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", errors.New("cannot find the home directory of the user")
	}
	return home, nil
}

// xdgDir returns the directory of an XDG variable, or the fallback below
// the home directory if the variable is empty or not absolute.
func xdgDir(variable string, fallback ...string) (string, error) {
	if dir := os.Getenv(variable); filepath.IsAbs(dir) {
		return filepath.Clean(dir), nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, fallback...)...), nil
}

// DefaultPath returns the location of the configuration file: the value of
// A38_CONFIG if it is set, else $XDG_CONFIG_HOME/a38/config.yaml, else
// ~/.config/a38/config.yaml.
func DefaultPath() (string, error) {
	if path := os.Getenv(EnvConfig); path != "" {
		return absPath(path)
	}
	dir, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "a38", "config.yaml"), nil
}

// stateDir returns $XDG_STATE_HOME/a38, or ~/.local/state/a38.
func stateDir() (string, error) {
	dir, err := xdgDir("XDG_STATE_HOME", ".local", "state")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "a38"), nil
}

// DefaultSocketDir returns the default directory of the sockets:
// $XDG_STATE_HOME/a38/run, or ~/.local/state/a38/run. "a38 mcp" uses it to
// find the agent socket without the configuration file.
func DefaultSocketDir() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "run"), nil
}

// DefaultAuditFile returns the default location of the audit file:
// $XDG_STATE_HOME/a38/audit.db, or ~/.local/state/a38/audit.db.
func DefaultAuditFile() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "audit.db"), nil
}
