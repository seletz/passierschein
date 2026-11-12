package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// tokenCmd is the command line of the token commands.
const tokenCmd = "a38 config onepassword-token"

const tokenUsage = `Usage:
  a38 config onepassword-token set    [--replace] [--config FILE]
  a38 config onepassword-token status [--config FILE] [--json]
  a38 config onepassword-token delete [--config FILE]

The token of the 1Password service account lives in the macOS keychain
(D51). Only the macOS build has these commands.

"set" reads the token from the terminal without echo. Standard input must
be a terminal: the command never reads the token from a pipe, a file, an
argument or an environment variable. An existing item is replaced only
with --replace. The command never shows the token or a value derived from
it.

"status" shows the configured source, the metadata of the keychain item
(never its value), the signature of a38 and a token file that is left. It
never reads the token, so it never causes a macOS dialog. It exits with 1
if the configured source has no token.

"delete" removes the keychain item.

The daemon reads the token once at its start. After "set" or "delete",
restart the daemon.
`

// maxTokenInput is the longest line that "set" reads from the terminal.
const maxTokenInput = token.MaxLength

// newTokenStore returns the store of the token. A test replaces it, so
// that no test touches the keychain item of the human.
var newTokenStore = systemTokenStore

// tokenDeps are the parts of the world that the token commands use. A
// test replaces them.
type tokenDeps struct {
	// terminal is true if standard input is a terminal.
	terminal bool
	// readSecret reads one line from the terminal without echo.
	readSecret func(prompt string) ([]byte, error)
	// newStore returns the store of the token.
	newStore func() (token.Store, error)
	// lstat reads the file information of the token file. The commands
	// never read the content of the token file.
	lstat func(path string) (os.FileInfo, error)
}

// runOnePasswordToken stores, shows and deletes the token of the service
// account in the macOS keychain (D51, issue 59).
func runOnePasswordToken(args []string, stdout, stderr io.Writer) int {
	deps := tokenDeps{
		terminal: isTerminal(os.Stdin),
		readSecret: func(prompt string) ([]byte, error) {
			return readSecretLine(os.Stdin, stderr, prompt, maxTokenInput)
		},
		newStore: newTokenStore,
		lstat:    os.Lstat,
	}
	return runTokenWith(args, stdout, stderr, deps)
}

func runTokenWith(args []string, stdout, stderr io.Writer, deps tokenDeps) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, tokenUsage)
		return exitUsage
	}
	var run func([]string, io.Writer, io.Writer, tokenDeps) int
	switch args[0] {
	case "set":
		run = runTokenSet
	case "status":
		run = runTokenStatus
	case "delete":
		run = runTokenDelete
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, tokenUsage)
		return exitOK
	default:
		_, _ = fmt.Fprintf(stderr, "%s: unknown subcommand %q\n\n%s", tokenCmd, args[0], tokenUsage)
		return exitUsage
	}
	return run(args[1:], stdout, stderr, deps)
}

// tokenConfig loads the configuration for the token commands. The
// commands work also without a configuration file: the source is then the
// default.
func tokenConfig(path string) (*config.Config, error) {
	if path == "" {
		var err error
		if path, err = config.DefaultPath(); err != nil {
			return nil, err
		}
	}
	return config.Load(path)
}

// sourceOf returns the configured source of the token, or the default.
func sourceOf(cfg *config.Config) config.TokenSource {
	if cfg == nil || cfg.Backend.OnePassword.TokenSource == "" {
		return config.DefaultTokenSource
	}
	return cfg.Backend.OnePassword.TokenSource
}

// tokenFileOf returns the configured token file, or the default.
func tokenFileOf(cfg *config.Config) string {
	if cfg != nil && cfg.Backend.OnePassword.TokenFile != "" {
		return cfg.Backend.OnePassword.TokenFile
	}
	path, _ := config.DefaultTokenFile()
	return path
}

func runTokenSet(args []string, stdout, stderr io.Writer, deps tokenDeps) int {
	var configPath string
	var replace bool
	fs := newFlagSet("config onepassword-token set", stderr)
	fs.StringVar(&configPath, "config", "", "configuration file (default: the standard location)")
	fs.BoolVar(&replace, "replace", false, "replace an existing keychain item")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		// Never echo the argument: it can be a token.
		_, _ = fmt.Fprintf(stderr, "%s set: unexpected argument; the token is never an argument: run the command in a terminal and paste the token at the prompt\n", tokenCmd)
		return exitUsage
	}
	if !deps.terminal {
		// Read nothing: the token comes only from the terminal (decision
		// of Stefan, 2026-11-12), never from a pipe or a file.
		_, _ = fmt.Fprintf(stderr, "%s set: standard input is not a terminal; run the command in a terminal and paste the token at the prompt. The command does not read the token from a pipe or a file\n", tokenCmd)
		return exitUsage
	}
	store, err := deps.newStore()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s set: %v\n", tokenCmd, err)
		return exitFailure
	}
	// Refuse before the human pastes the token. Put checks again.
	if info, err := store.Describe(); err == nil && info.Exists && !replace {
		_, _ = fmt.Fprintf(stderr, "%s set: the keychain item %s exists already; nothing changed. To replace it, run \"%s set --replace\"\n", tokenCmd, store.Name(), tokenCmd)
		return exitFailure
	}

	raw, err := deps.readSecret("Paste the token of the 1Password service account (no echo), then press Return: ")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s set: %v; nothing stored\n", tokenCmd, err)
		return exitFailure
	}
	tok := token.New(string(raw))
	clear(raw)
	warnings, err := token.CheckShape(tok.Reveal())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s set: %v; nothing stored\n", tokenCmd, err)
		return exitFailure
	}
	for _, w := range warnings {
		_, _ = fmt.Fprintf(stderr, "%s set: warning: %s\n", tokenCmd, w)
	}

	if err := store.Put(tok, replace); err != nil {
		if errors.Is(err, token.ErrExists) {
			_, _ = fmt.Fprintf(stderr, "%s set: the keychain item %s exists already; nothing changed. To replace it, run \"%s set --replace\"\n", tokenCmd, store.Name(), tokenCmd)
			return exitFailure
		}
		_, _ = fmt.Fprintf(stderr, "%s set: %v; nothing stored\n", tokenCmd, err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stdout, "Stored the token in the keychain item %s.\n", store.Name())

	if cfg, err := tokenConfig(configPath); err == nil && sourceOf(cfg) == config.TokenSourceFile {
		_, _ = fmt.Fprintf(stderr, "%s set: note: the configuration uses the token file (backend.onepassword.token_source: file); the daemon does not read the keychain\n", tokenCmd)
	}
	return exitOK
}

// tokenStatus is the output of the status command. It holds metadata
// only, never the token or a value derived from it.
type tokenStatus struct {
	// Source is the configured source of the token.
	Source config.TokenSource `json:"source"`
	// ConfigError says why the configuration did not load; the status
	// then uses the default source.
	ConfigError string `json:"config_error,omitempty"`
	// Keychain describes the keychain item.
	Keychain keychainStatus `json:"keychain"`
	// Signature describes the code signature of this a38.
	Signature *token.Signature `json:"signature,omitempty"`
	// SignatureWarning explains the limit of an unstable signature.
	SignatureWarning string `json:"signature_warning,omitempty"`
	// TokenFile describes the token file.
	TokenFile tokenFileStatus `json:"token_file"`
	// OK is true if the configured source has a token.
	OK bool `json:"ok"`
}

// keychainStatus describes the keychain item without its value.
type keychainStatus struct {
	token.StoreInfo
	// Error says why the item cannot be described.
	Error string `json:"error,omitempty"`
	// DialogNote says that the next read by this build shows a dialog.
	DialogNote string `json:"dialog_note,omitempty"`
	// OtherPrograms lists the trusted programs that are not this a38.
	OtherPrograms []string `json:"other_programs,omitempty"`
}

// tokenFileStatus describes the token file by its metadata only.
type tokenFileStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	// Finding is a problem of the file; empty if none.
	Finding string `json:"finding,omitempty"`
}

// dialogNote explains that this build did not store the item.
const dialogNote = "this build of a38 did not store the item; the next start of the daemon shows a macOS dialog. Store the token again with \"" + tokenCmd + " set --replace\" instead of confirming the dialog"

func runTokenStatus(args []string, stdout, stderr io.Writer, deps tokenDeps) int {
	var af adminFlags
	fs := newFlagSet("config onepassword-token status", stderr)
	af.register(fs)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintf(stderr, "%s status: unexpected argument\n", tokenCmd)
		return exitUsage
	}
	store, err := deps.newStore()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s status: %v\n", tokenCmd, err)
		return exitFailure
	}
	st := collectTokenStatus(store, af.configPath, deps)
	if af.asJSON {
		if err := json.NewEncoder(stdout).Encode(st); err != nil {
			_, _ = fmt.Fprintf(stderr, "%s status: %v\n", tokenCmd, err)
			return exitFailure
		}
	} else {
		renderTokenStatus(stdout, st)
	}
	if !st.OK {
		return exitFailure
	}
	return exitOK
}

// collectTokenStatus gathers the status. It never reads the token: the
// keychain gives the metadata and the access list without a dialog; the
// token file is checked by its metadata (owner, mode) only.
func collectTokenStatus(store token.Store, configPath string, deps tokenDeps) tokenStatus {
	var st tokenStatus
	cfg, err := tokenConfig(configPath)
	if err != nil {
		st.ConfigError = err.Error()
		cfg = nil
	}
	st.Source = sourceOf(cfg)

	info, err := store.Describe()
	st.Keychain.StoreInfo = info
	st.Keychain.Name = store.Name()
	if err != nil {
		st.Keychain.Error = err.Error()
	}
	if sig, err := store.Signature(); err == nil {
		st.Signature = &sig
		if !sig.Stable() {
			st.SignatureWarning = token.UnstableSignatureWarning
		}
		for _, p := range info.Trusted {
			if p != sig.Path {
				st.Keychain.OtherPrograms = append(st.Keychain.OtherPrograms, p)
			}
		}
	}
	if info.Exists && info.AccessListError == "" && len(st.Keychain.OtherPrograms) == 0 && !info.ReadableWithoutDialog {
		st.Keychain.DialogNote = dialogNote
	}

	st.TokenFile.Path = tokenFileOf(cfg)
	fi, err := deps.lstat(st.TokenFile.Path)
	switch {
	case err == nil:
		st.TokenFile.Exists = true
		if err := tokenfile.CheckInfo(fi, os.Getuid()); err != nil {
			st.TokenFile.Finding = err.Error()
		} else if st.Source == config.TokenSourceKeychain {
			st.TokenFile.Finding = "the token file is left although the source is the keychain; any process of the user can read it"
		}
	case !errors.Is(err, os.ErrNotExist):
		st.TokenFile.Exists = true
		st.TokenFile.Finding = err.Error()
	}

	switch st.Source {
	case config.TokenSourceFile:
		st.OK = st.TokenFile.Exists && st.TokenFile.Finding == ""
	default:
		st.OK = info.Exists && st.Keychain.Error == ""
	}
	return st
}

func renderTokenStatus(w io.Writer, st tokenStatus) {
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format+"\n", args...) }
	p("Source:      %s (backend.onepassword.token_source)", st.Source)
	if st.ConfigError != "" {
		p("             the configuration did not load, so this is the default: %s", oneLine(st.ConfigError))
	}
	k := st.Keychain
	switch {
	case k.Error != "":
		p("Keychain:    item %s cannot be read: %s", k.Name, k.Error)
	case !k.Exists:
		p("Keychain:    no item %s", k.Name)
		if st.Source == config.TokenSourceKeychain {
			p("             Next step: store the token with \"%s set\".", tokenCmd)
		}
	default:
		p("Keychain:    item %s exists (value not read)", k.Name)
		p("  Created:   %s", stamp(k.Created))
		p("  Changed:   %s", stamp(k.Modified))
		if k.AccessListError != "" {
			p("  Trusted:   cannot read the access list: %s", k.AccessListError)
		} else {
			p("  Trusted:   %s", orNone(k.Trusted))
			p("  Partition: %s", orNone(k.Partitions))
		}
		if len(k.OtherPrograms) > 0 {
			p("  Warning:   the item trusts a program other than this a38: %s. Run \"%s delete\" and \"%s set\".", strings.Join(k.OtherPrograms, ", "), tokenCmd, tokenCmd)
		}
		if k.DialogNote != "" {
			p("  Note:      %s.", k.DialogNote)
		}
	}
	if s := st.Signature; s != nil {
		line := string(s.Kind)
		if s.TeamID != "" {
			line += ", team " + s.TeamID
		}
		if s.CDHash != "" {
			line += ", cdhash " + s.CDHash
		}
		p("Signature:   %s (%s)", line, s.Path)
	}
	if st.SignatureWarning != "" {
		p("  Warning:   %s.", st.SignatureWarning)
	}
	f := st.TokenFile
	if f.Exists {
		p("Token file:  %s exists (content not read)", f.Path)
	} else {
		p("Token file:  %s does not exist", f.Path)
	}
	if f.Finding != "" {
		p("  Finding:   %s", f.Finding)
	}
	p("The daemon reads the token once at its start; restart it after a change.")
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func orNone(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	return strings.Join(list, ", ")
}

// oneLine puts a text of several lines on one line.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func runTokenDelete(args []string, stdout, stderr io.Writer, deps tokenDeps) int {
	var configPath string
	fs := newFlagSet("config onepassword-token delete", stderr)
	fs.StringVar(&configPath, "config", "", "configuration file (default: the standard location)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintf(stderr, "%s delete: unexpected argument\n", tokenCmd)
		return exitUsage
	}
	store, err := deps.newStore()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s delete: %v\n", tokenCmd, err)
		return exitFailure
	}
	if err := store.Delete(); err != nil {
		if errors.Is(err, token.ErrNotFound) {
			_, _ = fmt.Fprintf(stderr, "%s delete: the keychain has no item %s; nothing changed\n", tokenCmd, store.Name())
			return exitFailure
		}
		_, _ = fmt.Fprintf(stderr, "%s delete: %v\n", tokenCmd, err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stdout, "Deleted the keychain item %s.\n", store.Name())
	cfg, err := tokenConfig(configPath)
	if err != nil {
		cfg = nil
	}
	if sourceOf(cfg) == config.TokenSourceFile {
		_, _ = fmt.Fprintln(stdout, "The daemon uses the token file (backend.onepassword.token_source: file); the keychain item does not change it.")
		return exitOK
	}
	_, _ = fmt.Fprintf(stdout, "A running daemon keeps its token in memory until it stops. Its next start with the 1Password backend fails until you run \"%s set\".\n", tokenCmd)
	return exitOK
}
