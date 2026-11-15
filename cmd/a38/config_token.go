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
  a38 config onepassword-token set    --account NAME [--replace] [--config FILE]
  a38 config onepassword-token status [--account NAME] [--config FILE] [--json]
  a38 config onepassword-token delete --account NAME | --legacy [--config FILE]

Each 1Password account of the configuration (backend.onepassword.accounts)
has the token of its own service account in its own item of the macOS
keychain: service a38.onepassword, account NAME (D72). Only the macOS build
has these commands.

"set" stores the token of one account. The account must be in the
configuration. The command reads the token from the terminal without echo.
Standard input must be a terminal: the command never reads the token from a
pipe, a file, an argument or an environment variable. An existing item is
replaced only with --replace. The command never shows the token or a value
derived from it.

"status" shows for each account (or only for --account) the configured
source, the metadata of the keychain item (never its value) and the token
file; then the item of the configuration with one token if it is left, and
the signature of a38. It never reads a token, so it never causes a macOS
dialog. It exits with 1 if an account that a vault names has no token.

"delete --account NAME" removes the keychain item of the account. The
account need not be in the configuration. "delete --legacy" removes the
item a38.onepassword/service-account of the configuration with one token.

The daemon reads the tokens once at its start. After "set" or "delete",
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
	// newStore returns the store of the token of an account. The account
	// config.LegacyAccount gives the item of the configuration with one
	// token.
	newStore func(account string) (token.Store, error)
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

// tokenConfig loads the configuration for the token commands. "set"
// needs it; "status" and "delete" work also without it.
func tokenConfig(path string) (*config.Config, error) {
	if path == "" {
		var err error
		if path, err = config.DefaultPath(); err != nil {
			return nil, err
		}
	}
	return config.Load(path)
}

// errNoAccount is the text for a missing --account.
const errNoAccount = "give the account with --account NAME; the names are in backend.onepassword.accounts (a38 config account list)"

func runTokenSet(args []string, stdout, stderr io.Writer, deps tokenDeps) int {
	var configPath, account string
	var replace bool
	fs := newFlagSet("config onepassword-token set", stderr)
	fs.StringVar(&configPath, "config", "", "configuration file (default: the standard location)")
	fs.StringVar(&account, "account", "", "name of the 1Password account (backend.onepassword.accounts)")
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
	if account == "" {
		_, _ = fmt.Fprintf(stderr, "%s set: %s\n", tokenCmd, errNoAccount)
		return exitUsage
	}
	if !deps.terminal {
		// Read nothing: the token comes only from the terminal (decision
		// of Stefan, 2026-11-12), never from a pipe or a file.
		_, _ = fmt.Fprintf(stderr, "%s set: standard input is not a terminal; run the command in a terminal and paste the token at the prompt. The command does not read the token from a pipe or a file\n", tokenCmd)
		return exitUsage
	}
	// D72: the account must be in the configuration, so that a typing
	// error does not create an item that the daemon never reads.
	cfg, err := tokenConfig(configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s set: the configuration does not load, so the account cannot be checked; nothing read: %v\n", tokenCmd, err)
		return exitFailure
	}
	acc, ok := cfg.Account(account)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "%s set: the account %q is not in backend.onepassword.accounts; add it first with \"a38 config account add %s\"; nothing read\n", tokenCmd, account, account)
		return exitFailure
	}
	store, err := deps.newStore(account)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s set: %v\n", tokenCmd, err)
		return exitFailure
	}
	again := fmt.Sprintf("%s set --account %s --replace", tokenCmd, account)
	// Refuse before the human pastes the token. Put checks again.
	if info, err := store.Describe(); err == nil && info.Exists && !replace {
		_, _ = fmt.Fprintf(stderr, "%s set: the keychain item %s exists already; nothing changed. To replace it, run \"%s\"\n", tokenCmd, store.Name(), again)
		return exitFailure
	}

	raw, err := deps.readSecret(fmt.Sprintf("Paste the token of the 1Password service account of the account %q (no echo), then press Return: ", account))
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
			_, _ = fmt.Fprintf(stderr, "%s set: the keychain item %s exists already; nothing changed. To replace it, run \"%s\"\n", tokenCmd, store.Name(), again)
			return exitFailure
		}
		_, _ = fmt.Fprintf(stderr, "%s set: %v; nothing stored\n", tokenCmd, err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stdout, "Stored the token of the account %q in the keychain item %s.\n", account, store.Name())
	if acc.TokenSource == config.TokenSourceFile {
		_, _ = fmt.Fprintf(stderr, "%s set: note: the account %q uses its token file (token_source: file); the daemon does not read the keychain for it\n", tokenCmd, account)
	}
	return exitOK
}

// tokenStatus is the output of the status command. It holds metadata
// only, never a token or a value derived from it.
type tokenStatus struct {
	// ConfigError says why the configuration did not load.
	ConfigError string `json:"config_error,omitempty"`
	// Accounts describes each account (or the one of --account).
	Accounts []accountStatus `json:"accounts"`
	// Legacy describes the item and the token file of the configuration
	// with one token; nil if neither is left.
	Legacy *legacyStatus `json:"legacy,omitempty"`
	// Signature describes the code signature of this a38.
	Signature *token.Signature `json:"signature,omitempty"`
	// SignatureWarning explains the limit of an unstable signature.
	SignatureWarning string `json:"signature_warning,omitempty"`
	// OK is true if each account that a vault names has a token.
	OK bool `json:"ok"`
}

// accountStatus describes the token of one account.
type accountStatus struct {
	// Account is the name of the account.
	Account string `json:"account"`
	// InConfig is false for --account with a name that the configuration
	// does not have.
	InConfig bool `json:"in_config"`
	// Vaults lists the vaults of the account.
	Vaults []string `json:"vaults"`
	// Source is the configured source of the token.
	Source config.TokenSource `json:"source"`
	// Keychain describes the keychain item of the account.
	Keychain keychainStatus `json:"keychain"`
	// TokenFile describes the token file of the account.
	TokenFile tokenFileStatus `json:"token_file"`
	// OK is true if the source has a token, or if no vault names the
	// account.
	OK bool `json:"ok"`
}

// legacyStatus describes what is left of the configuration with one token.
type legacyStatus struct {
	Keychain  *keychainStatus  `json:"keychain,omitempty"`
	TokenFile *tokenFileStatus `json:"token_file,omitempty"`
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

func runTokenStatus(args []string, stdout, stderr io.Writer, deps tokenDeps) int {
	var af adminFlags
	var account string
	fs := newFlagSet("config onepassword-token status", stderr)
	af.register(fs)
	fs.StringVar(&account, "account", "", "show only this account")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintf(stderr, "%s status: unexpected argument\n", tokenCmd)
		return exitUsage
	}
	if account != "" {
		if err := config.ValidAccountName(account); err != nil {
			_, _ = fmt.Fprintf(stderr, "%s status: the account name %v\n", tokenCmd, err)
			return exitUsage
		}
	}
	st, err := collectTokenStatus(af.configPath, account, deps)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s status: %v\n", tokenCmd, err)
		return exitFailure
	}
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

// collectTokenStatus gathers the status. It never reads a token: the
// keychain gives the metadata and the access list without a dialog; a
// token file is checked by its metadata (owner, mode) only.
func collectTokenStatus(configPath, only string, deps tokenDeps) (tokenStatus, error) {
	var st tokenStatus
	cfg, err := tokenConfig(configPath)
	if err != nil {
		st.ConfigError = err.Error()
		cfg = nil
	}
	var accounts []config.Account
	if cfg != nil {
		accounts = cfg.Backend.OnePassword.Accounts
	}
	if only != "" {
		var found []config.Account
		for _, a := range accounts {
			if a.Name == only {
				found = append(found, a)
			}
		}
		if len(found) == 0 {
			// Not in the configuration: show the item with the default
			// source, so that the human finds an item that is left.
			path, _ := config.DefaultAccountTokenFile(only)
			found = []config.Account{{Name: only, TokenSource: config.DefaultTokenSource, TokenFile: path}}
		}
		accounts = found
	}
	st.OK = st.ConfigError == ""
	var sigStore token.Store
	for _, a := range accounts {
		store, err := deps.newStore(a.Name)
		if err != nil {
			return st, err
		}
		if sigStore == nil {
			sigStore = store
		}
		as := accountStatus{Account: a.Name, Source: a.TokenSource}
		if cfg != nil {
			_, as.InConfig = cfg.Account(a.Name)
			as.Vaults = cfg.VaultsOf(a.Name)
		}
		as.Keychain = describeItem(store)
		as.TokenFile = describeFile(a.TokenFile, a.TokenSource == config.TokenSourceKeychain, deps)
		switch {
		case len(as.Vaults) == 0:
			as.OK = true
		case a.TokenSource == config.TokenSourceFile:
			as.OK = as.TokenFile.Exists && as.TokenFile.Finding == ""
		default:
			as.OK = as.Keychain.Exists && as.Keychain.Error == ""
		}
		if as.InConfig && !as.OK {
			st.OK = false
		}
		st.Accounts = append(st.Accounts, as)
	}
	if only == "" {
		legacy, err := deps.newStore(config.LegacyAccount)
		if err != nil {
			return st, err
		}
		if sigStore == nil {
			sigStore = legacy
		}
		var l legacyStatus
		if k := describeItem(legacy); k.Exists || k.Error != "" {
			l.Keychain = &k
		}
		// The old token file is left only if no account names it.
		path, err := config.DefaultTokenFile()
		for _, a := range accounts {
			if a.TokenFile == path {
				err = errors.New("in use")
			}
		}
		if err == nil {
			if f := describeFile(path, false, deps); f.Exists {
				f.Finding = "the token file of the configuration with one token is left; any process of the user can read it"
				l.TokenFile = &f
			}
		}
		if l.Keychain != nil || l.TokenFile != nil {
			st.Legacy = &l
		}
	}
	if sigStore != nil {
		if sig, err := sigStore.Signature(); err == nil {
			st.Signature = &sig
			if !sig.Stable() {
				st.SignatureWarning = token.UnstableSignatureWarning
			}
			for i := range st.Accounts {
				st.Accounts[i].Keychain.trust(sig)
			}
		}
	}
	return st, nil
}

// describeItem reads the metadata of a keychain item, never its value.
func describeItem(store token.Store) keychainStatus {
	info, err := store.Describe()
	k := keychainStatus{StoreInfo: info}
	k.Name = store.Name()
	if err != nil {
		k.Error = err.Error()
	}
	return k
}

// trust compares the access list with the signature of this a38.
func (k *keychainStatus) trust(sig token.Signature) {
	for _, p := range k.Trusted {
		if p != sig.Path {
			k.OtherPrograms = append(k.OtherPrograms, p)
		}
	}
	if k.Exists && k.AccessListError == "" && len(k.OtherPrograms) == 0 && !k.ReadableWithoutDialog {
		k.DialogNote = "this build of a38 did not store the item; the next start of the daemon shows a macOS dialog. Store the token again with --replace instead of confirming the dialog"
	}
}

// describeFile checks a token file by its metadata. With left true, an
// existing file is a finding: the source is the keychain.
func describeFile(path string, left bool, deps tokenDeps) tokenFileStatus {
	f := tokenFileStatus{Path: path}
	fi, err := deps.lstat(path)
	switch {
	case err == nil:
		f.Exists = true
		if err := tokenfile.CheckInfo(fi, os.Getuid()); err != nil {
			f.Finding = err.Error()
		} else if left {
			f.Finding = "the token file is left although the source is the keychain; any process of the user can read it"
		}
	case !errors.Is(err, os.ErrNotExist):
		f.Exists = true
		f.Finding = err.Error()
	}
	return f
}

func renderTokenStatus(w io.Writer, st tokenStatus) {
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format+"\n", args...) }
	if st.ConfigError != "" {
		p("Configuration: does not load: %s", oneLine(st.ConfigError))
	}
	if len(st.Accounts) == 0 && st.ConfigError == "" {
		p("Accounts:    none in backend.onepassword.accounts; add one with \"a38 config account add NAME\"")
	}
	for _, a := range st.Accounts {
		switch {
		case !a.InConfig:
			p("Account %s: not in the configuration", a.Account)
		case len(a.Vaults) == 0:
			p("Account %s: no vault names it; the daemon does not read its token", a.Account)
		default:
			p("Account %s: vaults %s", a.Account, strings.Join(a.Vaults, ", "))
		}
		p("  Source:      %s (token_source)", a.Source)
		renderItem(p, "  Keychain:    ", a.Keychain, a.Source == config.TokenSourceKeychain && a.InConfig, a.Account)
		f := a.TokenFile
		if f.Exists {
			p("  Token file:  %s exists (content not read)", f.Path)
		} else {
			p("  Token file:  %s does not exist", f.Path)
		}
		if f.Finding != "" {
			p("    Finding:   %s", f.Finding)
		}
	}
	if l := st.Legacy; l != nil {
		if k := l.Keychain; k != nil {
			if k.Error != "" {
				p("Old item:    %s cannot be read: %s", k.Name, k.Error)
			} else {
				p("Old item:    %s exists (value not read); the daemon does not read it any more.", k.Name)
				p("             Next step: after \"%s set --account NAME\" for each account, run \"%s delete --legacy\".", tokenCmd, tokenCmd)
			}
		}
		if f := l.TokenFile; f != nil {
			p("Old file:    %s exists (content not read); %s.", f.Path, strings.TrimPrefix(f.Finding, "the token file of the configuration with one token is left; "))
			p("             Next step: move it to the token file of an account or delete it: rm %s", f.Path)
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
	p("The daemon reads the tokens once at its start; restart it after a change.")
}

// renderItem writes the lines of a keychain item.
func renderItem(p func(string, ...any), head string, k keychainStatus, used bool, account string) {
	switch {
	case k.Error != "":
		p("%sitem %s cannot be read: %s", head, k.Name, k.Error)
	case !k.Exists:
		p("%sno item %s", head, k.Name)
		if used {
			p("               Next step: store the token with \"%s set --account %s\".", tokenCmd, account)
		}
	default:
		p("%sitem %s exists (value not read)", head, k.Name)
		p("    Created:   %s", stamp(k.Created))
		p("    Changed:   %s", stamp(k.Modified))
		if k.AccessListError != "" {
			p("    Trusted:   cannot read the access list: %s", k.AccessListError)
		} else {
			p("    Trusted:   %s", orNone(k.Trusted))
			p("    Partition: %s", orNone(k.Partitions))
		}
		if len(k.OtherPrograms) > 0 {
			p("    Warning:   the item trusts a program other than this a38: %s. Run \"%s delete --account %s\" and \"%s set --account %s\".", strings.Join(k.OtherPrograms, ", "), tokenCmd, account, tokenCmd, account)
		}
		if k.DialogNote != "" {
			p("    Note:      %s (\"%s set --account %s --replace\").", k.DialogNote, tokenCmd, account)
		}
	}
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
	var configPath, account string
	var legacy bool
	fs := newFlagSet("config onepassword-token delete", stderr)
	fs.StringVar(&configPath, "config", "", "configuration file (default: the standard location)")
	fs.StringVar(&account, "account", "", "name of the 1Password account")
	fs.BoolVar(&legacy, "legacy", false, "delete the item of the configuration with one token")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintf(stderr, "%s delete: unexpected argument\n", tokenCmd)
		return exitUsage
	}
	switch {
	case account == "" && !legacy:
		_, _ = fmt.Fprintf(stderr, "%s delete: %s, or give --legacy for the item of the configuration with one token\n", tokenCmd, errNoAccount)
		return exitUsage
	case account != "" && legacy:
		_, _ = fmt.Fprintf(stderr, "%s delete: give either --account or --legacy, not both\n", tokenCmd)
		return exitUsage
	case legacy:
		account = config.LegacyAccount
	default:
		if err := config.ValidAccountName(account); err != nil {
			_, _ = fmt.Fprintf(stderr, "%s delete: the account name %v\n", tokenCmd, err)
			return exitUsage
		}
	}
	store, err := deps.newStore(account)
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
	if legacy {
		_, _ = fmt.Fprintln(stdout, "The daemon did not read this item; nothing changes for it.")
		return exitOK
	}
	cfg, err := tokenConfig(configPath)
	if err != nil {
		cfg = nil
	}
	var acc config.Account
	inConfig := false
	if cfg != nil {
		acc, inConfig = cfg.Account(account)
	}
	switch {
	case !inConfig:
		_, _ = fmt.Fprintf(stdout, "The account %q is not in the configuration; the daemon did not read this item.\n", account)
	case acc.TokenSource == config.TokenSourceFile:
		_, _ = fmt.Fprintf(stdout, "The account %q uses its token file (token_source: file); the keychain item does not change it.\n", account)
	default:
		_, _ = fmt.Fprintf(stdout, "A running daemon keeps the token in memory until it stops. At its next start the vaults of the account %q are locked until you run \"%s set --account %s\".\n", account, tokenCmd, account)
	}
	return exitOK
}
