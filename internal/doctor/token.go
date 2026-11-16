package doctor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// tokenCmd is the command that stores, shows and deletes a token.
const tokenCmd = "a38 config onepassword-token"

// tokenSourceCheck checks the token source of each 1Password account (D72,
// D74, issue 88): the keychain item of the account (default) or its token
// file. It also reports the keychain item and the token file of the
// configuration with one token (D75). It never reads a token.
//
// Decision: the unstable signature of a38 (ad-hoc or unsigned, G5) is a
// note, not a finding. It is the known state of stage 1; a finding would
// make the doctor fail on each machine until the signed release.
type tokenSourceCheck struct {
	env  Env
	goos string
}

func (tokenSourceCheck) Name() string { return "1password token" }

func (c tokenSourceCheck) Run(_ context.Context, st *State) []Result {
	if st.Config == nil {
		return nil
	}
	if c.goos != "darwin" {
		return []Result{ok(c.Name(), "not used: the 1Password backend is part of the macOS build only")}
	}
	kind := ""
	if st.Status != nil && st.Status.Backend != nil {
		kind = st.Status.Backend.Kind
	}
	if kind == string(backend.KindFake) {
		return []Result{ok(c.Name(), "not used: the daemon runs with the backend fake")}
	}
	needed := kind == string(backend.KindOnePassword)
	cfg := st.Config
	var out []Result
	if len(cfg.Backend.OnePassword.Accounts) == 0 {
		out = append(out, ok(c.Name(), "the configuration names no 1Password account; add one with a38 config account add NAME"))
	}
	var keychainAccount string
	for _, a := range cfg.Backend.OnePassword.Accounts {
		vaults := cfg.VaultsOf(a.Name)
		if len(vaults) == 0 {
			out = append(out, note(c.Name(), "account %q names no vault; the daemon does not read its token. Add a vault with a38 config vault add VAULT --account %s, or remove the account with a38 config account remove %s", a.Name, a.Name, a.Name))
			continue
		}
		acc := accountRef{name: a.Name, label: fmt.Sprintf("account %q (vaults: %s)", a.Name, strings.Join(vaults, ", "))}
		if a.TokenSource == config.TokenSourceFile {
			out = append(out, c.file(acc, a.TokenFile, needed)...)
			continue
		}
		if keychainAccount == "" {
			keychainAccount = a.Name
		}
		out = append(out, c.keychain(acc, a.TokenFile, needed)...)
	}
	out = append(out, c.legacy(cfg)...)
	if keychainAccount != "" {
		out = append(out, c.signature(keychainAccount)...)
	}
	return out
}

// accountRef names an account in a result.
type accountRef struct {
	name  string
	label string
}

// file checks the token file of an account: it exists, it is a regular
// file of the user, and its mode is 0600. It never reads the content.
func (c tokenSourceCheck) file(a accountRef, path string, needed bool) []Result {
	fi, err := c.env.Lstat(path)
	switch {
	case isMissing(err) && needed:
		return []Result{finding(c.Name(), a.label+": the token file does not exist",
			path+" does not exist (token_source: file); the vaults of the account are locked, each request ends with backend_locked",
			"write the token of the service account to "+path+" with the mode 0600, or use the keychain: remove token_source: file of the account and run "+tokenCmd+" set --account "+a.name+"; then restart the daemon (how-to: Mehrere 1Password-Konten einrichten)")}
	case isMissing(err):
		return []Result{ok(c.Name(), "%s: source file: %s does not exist; only a38 serve --backend 1password needs it", a.label, path)}
	case err != nil:
		return []Result{finding(c.Name(), a.label+": the token file cannot be read", errText(err), "check the path "+path)}
	}
	if err := tokenfile.CheckInfo(fi, os.Getuid()); err != nil {
		return []Result{finding(c.Name(), a.label+": the token file is not safe", path+": "+errText(err),
			"make it a regular file of your user and run: chmod 600 "+path+"; then restart the daemon")}
	}
	return []Result{ok(c.Name(), "%s: source file: %s, regular file of the user, mode 0600 (content not read; fallback for development and CI: any process of the user can read it)", a.label, path)}
}

// keychain checks the keychain item of an account without its value, and
// a token file of the account that is left.
func (c tokenSourceCheck) keychain(a accountRef, path string, needed bool) []Result {
	store := c.env.TokenStore(a.name)
	if store == nil {
		return []Result{finding(c.Name(), a.label+": the keychain is not available", "this build of a38 has no keychain",
			"use the macOS build of a38, or set token_source: file for the account")}
	}
	info, err := store.Describe()
	if err != nil {
		return []Result{finding(c.Name(), a.label+": the keychain item cannot be read", errText(err),
			"run "+tokenCmd+" status --account "+a.name+"; if the login keychain is locked, unlock it")}
	}
	set := tokenCmd + " set --account " + a.name
	var out []Result
	switch {
	case !info.Exists && needed:
		out = append(out, finding(c.Name(), a.label+": the keychain has no token of the account",
			"no item "+store.Name()+" (token_source: keychain); the vaults of the account are locked, each request ends with backend_locked",
			"run "+set+" and paste the token of the service account, then restart the daemon (how-to: Mehrere 1Password-Konten einrichten)"))
	case !info.Exists:
		out = append(out, ok(c.Name(), "%s: source keychain: no item %s; only a38 serve --backend 1password needs it; store the token with %s", a.label, store.Name(), set))
	default:
		out = append(out, ok(c.Name(), "%s: source keychain: item %s exists, changed %s, trusted: %s (value not read)",
			a.label, store.Name(), stampOf(info.Modified), strings.Join(info.Trusted, ", ")))
		if info.AccessListError != "" {
			out = append(out, note(c.Name(), "%s: the access list of the item cannot be read: %s", a.label, info.AccessListError))
		}
		if sig, err := store.Signature(); err == nil {
			out = append(out, c.trust(a, info, sig)...)
		}
	}
	if _, err := c.env.Lstat(path); err == nil {
		out = append(out, finding(c.Name(), a.label+": a token file is left although the source is the keychain",
			path+" exists; any process of the user can read it",
			"check with "+tokenCmd+" status --account "+a.name+" that the keychain item exists and that the daemon starts with it, then delete the file: rm "+path))
	}
	return out
}

// trust compares the access list of an item with the signature of a38.
func (c tokenSourceCheck) trust(a accountRef, info token.StoreInfo, sig token.Signature) []Result {
	var others []string
	for _, p := range info.Trusted {
		if p != sig.Path {
			others = append(others, p)
		}
	}
	switch {
	case len(others) > 0:
		return []Result{finding(c.Name(), a.label+": the keychain item trusts a program other than this a38",
			"trusted: "+strings.Join(others, ", ")+"; that program reads the token without a dialog",
			"run "+tokenCmd+" delete --account "+a.name+", then "+tokenCmd+" set --account "+a.name+"; never confirm a macOS dialog for an a38 item that you did not cause yourself")}
	case info.AccessListError == "" && !info.ReadableWithoutDialog:
		return []Result{note(c.Name(), "%s: this build of a38 did not store the item; the next start of the daemon shows a macOS dialog. Store the token again with %s set --account %s --replace instead of confirming the dialog", a.label, tokenCmd, a.name)}
	}
	return nil
}

// legacy reports the keychain item and the token file of the
// configuration with one token (D75). The daemon reads neither. It reads
// only metadata.
func (c tokenSourceCheck) legacy(cfg *config.Config) []Result {
	var out []Result
	if store := c.env.TokenStore(config.LegacyAccount); store != nil {
		if info, err := store.Describe(); err == nil && info.Exists {
			out = append(out, finding(c.Name(), "the keychain item of the configuration with one token is left",
				"item "+store.Name()+" exists; the daemon does not read it any more",
				"store the token of each account with "+tokenCmd+" set --account NAME, check with a38 doctor that the daemon starts, then run "+tokenCmd+" delete --legacy"))
		}
	}
	if path, err := config.DefaultTokenFile(); err == nil && !usedByAccount(cfg, path) {
		if _, err := c.env.Lstat(path); err == nil {
			out = append(out, finding(c.Name(), "the token file of the configuration with one token is left",
				path+" exists; the daemon does not read it any more, but any process of the user can read it",
				"move it to the token file of an account (token_file) or delete it: rm "+path))
		}
	}
	return out
}

// signature shows the signature of a38 once, as a note (G5). It asks the
// store of the first account with the source keychain.
func (c tokenSourceCheck) signature(account string) []Result {
	store := c.env.TokenStore(account)
	if store == nil {
		return nil
	}
	sig, err := store.Signature()
	switch {
	case err != nil:
		return []Result{note(c.Name(), "the signature of a38 cannot be read: %v", err)}
	case !sig.Stable():
		return []Result{note(c.Name(), "signature of a38: %s; %s", sig.Kind, token.UnstableSignatureWarning)}
	}
	return []Result{ok(c.Name(), "signature of a38: Developer ID, team %s", sig.TeamID)}
}

// stampOf formats a time of the keychain item.
func stampOf(t time.Time) string {
	if t.IsZero() {
		return "at an unknown time"
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// usedByAccount reports whether an account names the path as its token
// file.
func usedByAccount(cfg *config.Config, path string) bool {
	for _, a := range cfg.Backend.OnePassword.Accounts {
		if a.TokenFile == path {
			return true
		}
	}
	return false
}
