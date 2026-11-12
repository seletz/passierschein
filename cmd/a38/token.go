package main

import (
	"bytes"
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

const tokenUsage = `Usage:
  a38 token set    [--replace] [--config FILE]
  a38 token status [--config FILE] [--json]
  a38 token delete [--config FILE]

The token of the 1Password service account lives in the macOS keychain
(D51). Only the macOS build has these commands.

"set" reads the token from the terminal without echo, or from stdin if
stdin is not a terminal (one line, at most 4 KiB). The token is never an
argument and never an environment variable. An existing item is replaced
only with --replace. The command prints the length and a fingerprint (the
first 8 hex digits of SHA-256), never the token.

"status" shows the configured source, the keychain item without its value,
the signature of a38 and a token file that is left. It reads the value
only if this build of a38 stored it, so that macOS shows no dialog. It
exits with 1 if the configured source has no token.

"delete" removes the keychain item.

The daemon reads the token once at its start. After "set" or "delete",
restart the daemon.
`

// maxTokenInput is the largest input of "a38 token set" in bytes.
const maxTokenInput = 4 << 10

// newTokenStore returns the store of the token. A test replaces it, so
// that no test touches the keychain item of the human.
var newTokenStore = systemTokenStore

// tokenDeps are the parts of the world that "a38 token" uses. A test
// replaces them.
type tokenDeps struct {
	// stdin is the input of "set" if it is not a terminal.
	stdin io.Reader
	// terminal is true if stdin is a terminal.
	terminal bool
	// readSecret reads one line from the terminal without echo.
	readSecret func(prompt string) ([]byte, error)
	// newStore returns the store of the token.
	newStore func() (token.Store, error)
	// lstat reads the file information of the token file.
	lstat func(path string) (os.FileInfo, error)
	// readFile reads the token file with its checks.
	readFile func(path string) (token.Token, error)
}

// runToken stores, shows and deletes the token of the service account in
// the macOS keychain (D51, issue 59).
func runToken(args []string, stdout, stderr io.Writer) int {
	deps := tokenDeps{
		stdin:    os.Stdin,
		terminal: isTerminal(os.Stdin),
		readSecret: func(prompt string) ([]byte, error) {
			return readSecretLine(os.Stdin, stderr, prompt, maxTokenInput)
		},
		newStore: newTokenStore,
		lstat:    os.Lstat,
		readFile: tokenfile.Read,
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
		_, _ = fmt.Fprintf(stderr, "a38 token: unknown subcommand %q\n\n%s", args[0], tokenUsage)
		return exitUsage
	}
	return run(args[1:], stdout, stderr, deps)
}

// tokenConfig loads the configuration for "a38 token". The commands work
// also without a configuration file: the source is then the default.
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
	fs := newFlagSet("token set", stderr)
	fs.StringVar(&configPath, "config", "", "configuration file (default: the standard location)")
	fs.BoolVar(&replace, "replace", false, "replace an existing keychain item")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		// Never echo the argument: it can be a token.
		_, _ = fmt.Fprintln(stderr, "a38 token set: unexpected argument; the token is never an argument: paste it at the prompt or give it on stdin")
		return exitUsage
	}
	store, err := deps.newStore()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 token set: %v\n", err)
		return exitFailure
	}
	// Refuse before the human pastes the token. Put checks again.
	if info, err := store.Describe(); err == nil && info.Exists && !replace {
		_, _ = fmt.Fprintf(stderr, "a38 token set: the keychain item %s exists already; nothing changed. To replace it, run \"a38 token set --replace\"\n", store.Name())
		return exitFailure
	}

	var raw []byte
	if deps.terminal {
		raw, err = deps.readSecret("Paste the token of the 1Password service account (no echo), then press Return: ")
	} else {
		raw, err = readTokenInput(deps.stdin)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 token set: %v\n", err)
		return exitFailure
	}
	tok := token.New(string(raw))
	clear(raw)
	warnings, err := token.CheckShape(tok.Reveal())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 token set: %v; nothing stored\n", err)
		return exitFailure
	}
	for _, w := range warnings {
		_, _ = fmt.Fprintf(stderr, "a38 token set: warning: %s\n", w)
	}

	if err := store.Put(tok, replace); err != nil {
		if errors.Is(err, token.ErrExists) {
			_, _ = fmt.Fprintf(stderr, "a38 token set: the keychain item %s exists already; nothing changed. To replace it, run \"a38 token set --replace\"\n", store.Name())
			return exitFailure
		}
		_, _ = fmt.Fprintf(stderr, "a38 token set: %v; nothing stored\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stdout, "Stored the token in the keychain item %s: length %d bytes, fingerprint %s.\n", store.Name(), tok.Length(), tok.Fingerprint())

	if cfg, err := tokenConfig(configPath); err == nil && sourceOf(cfg) == config.TokenSourceFile {
		_, _ = fmt.Fprintln(stderr, "a38 token set: note: the configuration uses the token file (backend.onepassword.token_source: file); the daemon does not read the keychain")
	}
	return exitOK
}

// readTokenInput reads the token from a pipe or a file: one line, at most
// maxTokenInput bytes. It removes one line end only.
func readTokenInput(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxTokenInput+1))
	if err != nil {
		return nil, errors.New("cannot read stdin")
	}
	if len(data) > maxTokenInput {
		clear(data)
		return nil, fmt.Errorf("the input is longer than %d bytes; nothing stored", maxTokenInput)
	}
	data = bytes.TrimSuffix(data, []byte("\n"))
	data = bytes.TrimSuffix(data, []byte("\r"))
	if bytes.ContainsAny(data, "\r\n") {
		clear(data)
		return nil, errors.New("the input has more than one line; give only the token")
	}
	return data, nil
}

// tokenStatus is the output of "a38 token status".
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
	// ValueRead is true if the command read the value for the length and
	// the fingerprint.
	ValueRead bool `json:"value_read"`
	// ValueNote says why the command did not read the value.
	ValueNote string `json:"value_note,omitempty"`
	// Length is the length of the token in bytes.
	Length int `json:"length,omitempty"`
	// Fingerprint is the first 8 hex digits of the SHA-256 of the token.
	Fingerprint string `json:"fingerprint,omitempty"`
	// OtherPrograms lists the trusted programs that are not this a38.
	OtherPrograms []string `json:"other_programs,omitempty"`
}

// tokenFileStatus describes the token file.
type tokenFileStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	// Finding is a problem of the file; empty if none.
	Finding string `json:"finding,omitempty"`
	// Length and Fingerprint are set if the file is the configured
	// source or a file that is left, and the file passes its checks.
	Length      int    `json:"length,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// notReadNote explains why status does not read the value.
const notReadNote = "not read: this build of a38 did not store the item, so reading it could show a macOS dialog; store the token again with \"a38 token set --replace\" if the daemon must start without a dialog"

func runTokenStatus(args []string, stdout, stderr io.Writer, deps tokenDeps) int {
	var af adminFlags
	fs := newFlagSet("token status", stderr)
	af.register(fs)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 token status: unexpected argument")
		return exitUsage
	}
	store, err := deps.newStore()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 token status: %v\n", err)
		return exitFailure
	}
	st := collectTokenStatus(store, af.configPath, deps)
	if af.asJSON {
		if err := json.NewEncoder(stdout).Encode(st); err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 token status: %v\n", err)
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

// collectTokenStatus gathers the status. It reads the value of the
// keychain item only if the item is sure to give it to this build without
// a dialog (token.StoreInfo.ReadableWithoutDialog). Decision: macOS cannot
// be asked to fail instead of showing the dialog of the access list
// (spike, V4: kSecUseAuthenticationUIFail does not stop it), so the
// command decides from the access list, which it reads without a dialog.
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
	if info.Exists {
		if info.ReadableWithoutDialog {
			if tok, err := store.Token(); err == nil {
				st.Keychain.ValueRead = true
				st.Keychain.Length, st.Keychain.Fingerprint = tok.Length(), tok.Fingerprint()
			} else {
				st.Keychain.ValueNote = "not read: " + err.Error()
			}
		} else {
			st.Keychain.ValueNote = notReadNote
		}
	}

	st.TokenFile.Path = tokenFileOf(cfg)
	if _, err := deps.lstat(st.TokenFile.Path); err == nil {
		st.TokenFile.Exists = true
		if tok, err := deps.readFile(st.TokenFile.Path); err != nil {
			st.TokenFile.Finding = err.Error()
		} else {
			st.TokenFile.Length, st.TokenFile.Fingerprint = tok.Length(), tok.Fingerprint()
			if st.Source == config.TokenSourceKeychain {
				st.TokenFile.Finding = "the token file is left although the source is the keychain; any process of the user can read it"
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		st.TokenFile.Exists = true
		st.TokenFile.Finding = err.Error()
	}

	switch st.Source {
	case config.TokenSourceFile:
		st.OK = st.TokenFile.Exists && st.TokenFile.Fingerprint != ""
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
			p("             Next step: store the token with \"a38 token set\".")
		}
	default:
		p("Keychain:    item %s exists", k.Name)
		p("  Created:   %s", stamp(k.Created))
		p("  Changed:   %s", stamp(k.Modified))
		if k.AccessListError != "" {
			p("  Trusted:   cannot read the access list: %s", k.AccessListError)
		} else {
			p("  Trusted:   %s", orNone(k.Trusted))
			p("  Partition: %s", orNone(k.Partitions))
		}
		if len(k.OtherPrograms) > 0 {
			p("  Warning:   the item trusts a program other than this a38: %s. Run \"a38 token delete\" and \"a38 token set\".", strings.Join(k.OtherPrograms, ", "))
		}
		if k.ValueRead {
			p("  Token:     length %d bytes, fingerprint %s", k.Length, k.Fingerprint)
		} else {
			p("  Token:     %s", k.ValueNote)
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
	switch {
	case !f.Exists:
		p("Token file:  %s does not exist", f.Path)
	case f.Fingerprint != "":
		p("Token file:  %s exists: length %d bytes, fingerprint %s", f.Path, f.Length, f.Fingerprint)
	default:
		p("Token file:  %s exists", f.Path)
	}
	if f.Finding != "" {
		p("  Finding:   %s", f.Finding)
		if st.Source == config.TokenSourceKeychain && f.Fingerprint != "" {
			p("  Next step: check that the item above exists, then delete the file: rm %s", f.Path)
		}
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
	fs := newFlagSet("token delete", stderr)
	fs.StringVar(&configPath, "config", "", "configuration file (default: the standard location)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 token delete: unexpected argument")
		return exitUsage
	}
	store, err := deps.newStore()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 token delete: %v\n", err)
		return exitFailure
	}
	if err := store.Delete(); err != nil {
		if errors.Is(err, token.ErrNotFound) {
			_, _ = fmt.Fprintf(stderr, "a38 token delete: the keychain has no item %s; nothing changed\n", store.Name())
			return exitFailure
		}
		_, _ = fmt.Fprintf(stderr, "a38 token delete: %v\n", err)
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
	_, _ = fmt.Fprintln(stdout, "A running daemon keeps its token in memory until it stops. Its next start with the 1Password backend fails until you run \"a38 token set\".")
	return exitOK
}
