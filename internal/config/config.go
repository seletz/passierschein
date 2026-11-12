// Package config loads and checks the configuration file of the daemon.
//
// The file is YAML. Its default location is
// $XDG_CONFIG_HOME/a38/config.yaml, or ~/.config/a38/config.yaml if the
// variable is not set. The daemon reads the file once at its start. A
// change needs a restart of the daemon.
//
// A complete file with neutral data:
//
//	version: 1
//	vaults:
//	  - name: beispiel.app.k8s-dev
//	    environment: dev
//	projects:
//	  - name: beispielprojekt
//	    paths:
//	      - ~/develop/beispielprojekt
//	      - ~/develop/beispielprojekt__worktrees
//	    vaults:
//	      - beispiel.app.k8s-dev
//	grant:
//	  lifetime: 4h
//	bundle:
//	  window: 3s
//	  max_entries: 10
//	session_name:
//	  max_length: 80
//	  allowed_characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 -_():"
//	paths:
//	  socket_dir: ~/.local/state/a38/run
//	  audit_file: ~/.local/state/a38/audit.db
//
// Only "version" is necessary. Each other value has a default.
//
// Loading is strict: an unknown key, a vault without a valid environment,
// or a value of zero where a positive value is necessary makes loading
// fail, and the daemon does not start. There is one exception: a project
// that names a vault that is not in the vault list does not make loading
// fail. The project is marked as invalid, and the daemon rejects only the
// requests of this project.
package config

import (
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/seletz/passierschein/internal/protocol"
)

// FileVersion is the version of the file format.
const FileVersion = 1

// Defaults of the configuration.
const (
	// DefaultGrantLifetime is the lifetime of a grant.
	DefaultGrantLifetime = 4 * time.Hour
	// DefaultBundleWindow is the duration of the bundle window.
	DefaultBundleWindow = 3 * time.Second
	// DefaultBundleMaxEntries is the maximum number of entries in a bundle.
	DefaultBundleMaxEntries = 10
	// MaxBundleEntries is the largest valid value of bundle.max_entries.
	// The approval app rejects a request with more entries. The app has the
	// same limit in Swift: app/Sources/A38ApprovalCore/Decoding.swift
	// (maxCredentials). Change both together.
	MaxBundleEntries = 50
	// DefaultSessionNameMaxLength is the maximum length of a session name
	// in characters.
	DefaultSessionNameMaxLength = 80
	// DefaultSessionNameCharacters lists the characters that a session
	// name can contain: letters A to Z and a to z, digits, space, hyphen,
	// underscore, round brackets and colon.
	DefaultSessionNameCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 -_():"
)

// File names in the socket directory.
const (
	// AgentSocketName is the file name of the agent socket.
	AgentSocketName = "agent.sock"
	// AdminSocketName is the file name of the admin socket.
	AdminSocketName = "admin.sock"
)

// maxSocketPath is the maximum length of a socket path in bytes. macOS
// permits 103 bytes and the end mark.
const maxSocketPath = 103

// Environment is the environment of a vault. The configuration states it
// for each vault. The daemon never derives it from the name of the vault.
type Environment string

// The environments of a vault.
const (
	// EnvDev is the development environment. Only a dev vault delivers
	// values in the MVP.
	EnvDev Environment = "dev"
	// EnvTest is the test environment.
	EnvTest Environment = "test"
	// EnvProd is the production environment.
	EnvProd Environment = "prod"
)

// Valid reports whether e is dev, test or prod. The comparison is exact.
func (e Environment) Valid() bool {
	return e == EnvDev || e == EnvTest || e == EnvProd
}

// Config is the checked configuration. Do not change a Config after
// loading; the daemon shares it between goroutines.
type Config struct {
	// Vaults lists each vault that a38 can see. A vault that is not in the
	// list does not exist for a38.
	Vaults []Vault
	// Projects lists the projects and the vaults that each project can use.
	Projects []Project
	// Grant holds the settings for grants.
	Grant Grant
	// Bundle holds the settings for the bundling of requests.
	Bundle Bundle
	// SessionName holds the rules for the session name.
	SessionName SessionNameRules
	// Paths holds the locations of the sockets and of the audit file.
	Paths Paths
}

// Vault is one vault of the vault list.
type Vault struct {
	// Name is the name of the vault in the backend. The comparison with
	// the vault of a reference is exact.
	Name string
	// Environment is the environment of the vault.
	Environment Environment
}

// Project is one project with its policy.
type Project struct {
	// Name is the name of the project.
	Name string
	// Paths lists the directories of the project, absolute and clean. A
	// session belongs to the project if its working directory is one of
	// the directories or is below one.
	Paths []string
	// RealPaths lists Paths with symbolic links resolved at loading. A
	// path that does not exist or cannot be resolved stays as written.
	// ProjectFor compares the working directory with both lists.
	RealPaths []string
	// MissingPaths lists the paths that did not exist at loading, as
	// written. They do not make the project invalid; a38 doctor reports
	// them.
	MissingPaths []string
	// Vaults lists the vaults that the project can use.
	Vaults []string
	// Invalid is empty for a valid project. Otherwise it is the finding in
	// one sentence, for example the name of a vault that is not in the
	// vault list. The daemon rejects each request of an invalid project
	// with config_invalid.
	Invalid string
}

// Allows reports whether the project can use the vault. An invalid project
// allows no vault. The environment of the vault is a separate check.
func (p Project) Allows(vault string) bool {
	if p.Invalid != "" {
		return false
	}
	for _, v := range p.Vaults {
		if v == vault {
			return true
		}
	}
	return false
}

// Grant holds the settings for grants.
type Grant struct {
	// Lifetime is the lifetime of a grant. It applies to all projects. The
	// human can select a shorter lifetime in the approval request, never a
	// longer one.
	Lifetime time.Duration
}

// Bundle holds the settings for the bundling of requests.
type Bundle struct {
	// Window is the duration of the bundle window.
	Window time.Duration
	// MaxEntries is the maximum number of entries in one bundle.
	MaxEntries int
}

// Paths holds the locations of the files of the daemon.
type Paths struct {
	// SocketDir is the directory of the two sockets. The daemon creates it
	// with the mode 0700.
	SocketDir string
	// AuditFile is the SQLite file of the audit log.
	AuditFile string
	// UIApp is the path of the approval app. It is empty if the
	// configuration does not name one; the daemon then searches next to
	// its own binary. The flag --ui-app of "a38 serve" has priority.
	UIApp string
}

// SessionNameRules holds the rules for the session name. They apply to the
// whole daemon.
type SessionNameRules struct {
	// MaxLength is the maximum length in characters.
	MaxLength int
	// AllowedCharacters lists each character that a name can contain.
	AllowedCharacters string
}

// Check checks a session name against the rules. The empty name is valid:
// it means that no name is set. A control character is never valid, also
// if AllowedCharacters lists it. A failure is a *protocol.Error with the
// code invalid_request. The error text does not repeat the name.
func (r SessionNameRules) Check(name string) error {
	if !utf8.ValidString(name) {
		return protocol.Errorf(protocol.CodeInvalidRequest, "the session name is not valid UTF-8")
	}
	if utf8.RuneCountInString(name) > r.MaxLength {
		return protocol.Errorf(protocol.CodeInvalidRequest, "the session name is longer than %d characters", r.MaxLength)
	}
	for _, c := range name {
		if protocol.IsControl(c) {
			return protocol.Errorf(protocol.CodeInvalidRequest, "the session name contains a control character")
		}
		if !strings.ContainsRune(r.AllowedCharacters, c) {
			return protocol.Errorf(protocol.CodeInvalidRequest, "the session name contains a character that the configuration does not allow")
		}
	}
	return nil
}

// Vault returns the vault with the name. The comparison is exact.
func (c *Config) Vault(name string) (Vault, bool) {
	for _, v := range c.Vaults {
		if v.Name == name {
			return v, true
		}
	}
	return Vault{}, false
}

// Project returns the project with the name.
func (c *Config) Project(name string) (Project, bool) {
	for _, p := range c.Projects {
		if p.Name == name {
			return p, true
		}
	}
	return Project{}, false
}

// ProjectFor returns the project of a working directory. dir must be
// absolute. The caller resolves symbolic links before the call; ProjectFor
// does not read the file system. If directories of more than one project
// contain dir, the longest directory wins. ProjectFor also returns an
// invalid project, so that the caller can reject its requests with
// config_invalid.
func (c *Config) ProjectFor(dir string) (Project, bool) {
	if !filepath.IsAbs(dir) {
		return Project{}, false
	}
	dir = filepath.Clean(dir)
	var best Project
	bestLen := -1
	for _, p := range c.Projects {
		for _, root := range slices.Concat(p.Paths, p.RealPaths) {
			if dir != root && !strings.HasPrefix(dir, root+string(filepath.Separator)) {
				continue
			}
			if len(root) > bestLen {
				best, bestLen = p, len(root)
			}
		}
	}
	return best, bestLen >= 0
}

// InvalidProjects returns the projects with an invalid configuration.
func (c *Config) InvalidProjects() []Project {
	var out []Project
	for _, p := range c.Projects {
		if p.Invalid != "" {
			out = append(out, p)
		}
	}
	return out
}

// AgentSocket returns the path of the agent socket.
func (c *Config) AgentSocket() string {
	return filepath.Join(c.Paths.SocketDir, AgentSocketName)
}

// AdminSocket returns the path of the admin socket.
func (c *Config) AdminSocket() string {
	return filepath.Join(c.Paths.SocketDir, AdminSocketName)
}
