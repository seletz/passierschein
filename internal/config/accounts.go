package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// LegacyAccount is the account name of the keychain item of the
// configuration with one token (D51). No account can have this name, so
// that the old item and the item of an account never collide (D72).
const LegacyAccount = "service-account"

// MaxAccountName is the maximum length of the name of an account.
const MaxAccountName = 32

// accountName is the rule for the name of an account (D71): lower-case
// letters, digits and hyphen, 1 to 32 characters, the first a letter or a
// digit.
var accountName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Account is one 1Password account with the token source of its service
// account (D71).
type Account struct {
	// Name is the name of the account in a38, freely selected. It is the
	// account of the keychain item of the token (D72).
	Name string
	// TokenSource is the source of the token: the macOS keychain (the
	// default) or the token file.
	TokenSource TokenSource
	// TokenFile is the token file of the account. Only the source file
	// reads it; a38 doctor checks it also for the source keychain.
	TokenFile string
}

// ValidAccountName checks the name of an account. The text completes the
// sentence "the name ...".
func ValidAccountName(name string) error {
	switch {
	case name == "":
		return errors.New("is missing or empty")
	case name == LegacyAccount:
		return fmt.Errorf("%q is reserved for the keychain item of the configuration with one token", LegacyAccount)
	case !accountName.MatchString(name):
		return fmt.Errorf("must have 1 to %d characters: lower-case letters a to z, digits and hyphens, and start with a letter or a digit", MaxAccountName)
	}
	return nil
}

type rawAccount struct {
	Name        string    `yaml:"name"`
	TokenSource yaml.Node `yaml:"token_source"`
	TokenFile   yaml.Node `yaml:"token_file"`
}

// migrateHint is the next step for a file in the format with one token
// (D75).
const migrateHint = `add an account under backend.onepassword.accounts (for example "accounts: [{name: privat, token_source: keychain}]") and move a token_source: file and its token_file into it; add "account: privat" to each vault; check the file with "a38 config validate"; store the token again with "a38 config onepassword-token set --account privat" (the old keychain item is not taken over); restart the daemon and check it with "a38 doctor" (how-to: Mehrere 1Password-Konten einrichten)`

// parseAccounts checks backend.onepassword. The keys of the format with
// one token are an error with the next step (D75).
func parseAccounts(raw rawOnePassword, fail func(string, ...any)) []Account {
	for _, old := range []struct {
		key  string
		node yaml.Node
	}{{"token_source", raw.TokenSource}, {"token_file", raw.TokenFile}} {
		if old.node.Kind != 0 {
			fail("backend.onepassword.%s: this key is from the configuration with one token and is no longer valid; each account names its own token source. Next step: %s", old.key, migrateHint)
		}
	}
	var accounts []Account
	seen := map[string]bool{}
	files := map[string]string{}
	for i, ra := range raw.Accounts {
		if err := ValidAccountName(ra.Name); err != nil {
			fail("backend.onepassword.accounts[%d]: the name %v", i, err)
			continue
		}
		if seen[ra.Name] {
			fail("account %q: the name is in the list more than once", ra.Name)
			continue
		}
		seen[ra.Name] = true
		a := Account{Name: ra.Name}
		if src, err := stringValue(ra.TokenSource, string(DefaultTokenSource)); err != nil {
			fail("account %q: token_source: %v", ra.Name, err)
		} else if a.TokenSource = TokenSource(src); !a.TokenSource.Valid() {
			fail("account %q: token_source: the value must be %q or %q", ra.Name, TokenSourceKeychain, TokenSourceFile)
		}
		a.TokenFile = pathValue(ra.TokenFile, func() (string, error) { return DefaultAccountTokenFile(ra.Name) },
			fmt.Sprintf("account %q: token_file", ra.Name), fail)
		if other, taken := files[a.TokenFile]; taken && a.TokenFile != "" {
			fail("account %q: the token file is the token file of the account %q; each account needs its own file", ra.Name, other)
		}
		files[a.TokenFile] = ra.Name
		accounts = append(accounts, a)
	}
	return accounts
}

// checkVaultAccounts checks that each vault names an account of the list
// (D70). There is no default account. A file without accounts and with a
// vault without account is in the format with one token, also without the
// key token_source (the keychain was the default); the error then names
// the next step once (D75). oldKeys says that parseAccounts named it
// already.
func checkVaultAccounts(cfg *Config, oldKeys bool, fail func(string, ...any)) {
	if !oldKeys && len(cfg.Backend.OnePassword.Accounts) == 0 {
		for _, v := range cfg.Vaults {
			if v.Account == "" {
				fail("backend.onepassword.accounts: the configuration names no 1Password account; it is in the format with one token. Next step: %s", migrateHint)
				break
			}
		}
	}
	for _, v := range cfg.Vaults {
		switch _, ok := cfg.Account(v.Account); {
		case v.Account == "":
			fail("vault %q: the account is missing; each vault names its 1Password account (account: NAME), and NAME must be in backend.onepassword.accounts", v.Name)
		case !ok:
			fail("vault %q: the account %q is not in backend.onepassword.accounts; add it there or correct the name", v.Name, v.Account)
		}
	}
}

// Account returns the account with the name.
func (c *Config) Account(name string) (Account, bool) {
	for _, a := range c.Backend.OnePassword.Accounts {
		if a.Name == name {
			return a, true
		}
	}
	return Account{}, false
}

// VaultsOf returns the names of the vaults of the account, in the order of
// the vault list.
func (c *Config) VaultsOf(account string) []string {
	var out []string
	for _, v := range c.Vaults {
		if v.Account == account {
			out = append(out, v.Name)
		}
	}
	return out
}

// UnusedAccounts returns the names of the accounts that no vault names.
// The daemon does not read their tokens; a38 doctor names them.
func (c *Config) UnusedAccounts() []string {
	var out []string
	for _, a := range c.Backend.OnePassword.Accounts {
		if len(c.VaultsOf(a.Name)) == 0 {
			out = append(out, a.Name)
		}
	}
	return out
}

// DefaultAccountTokenFile returns the default token file of an account:
// $XDG_CONFIG_HOME/a38/onepassword-token-NAME, or
// ~/.config/a38/onepassword-token-NAME. A38_CONFIG does not move it.
func DefaultAccountTokenFile(account string) (string, error) {
	dir, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "a38", "onepassword-token-"+account), nil
}
