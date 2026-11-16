package config

import (
	"reflect"
	"strings"
	"testing"
)

// twoAccounts is a valid configuration with two accounts (D71).
const twoAccounts = `version: 1
vaults:
  - name: beispiel.app.k8s-dev
    environment: dev
    account: privat
  - name: datenbank-dev
    environment: dev
    account: firma
backend:
  onepassword:
    accounts:
      - name: privat
      - name: firma
        token_source: file
        token_file: ~/a38/onepassword-token-firma
`

func TestAccounts(t *testing.T) {
	// T43: the configuration loads only with an account at each vault, and
	// for the format with one token it names the next step (D70, D71, D75).
	setHome(t)
	cfg, err := Parse([]byte(twoAccounts))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []Account{
		{Name: "privat", TokenSource: TokenSourceKeychain, TokenFile: "/home/beispiel/.config/a38/onepassword-token-privat"},
		{Name: "firma", TokenSource: TokenSourceFile, TokenFile: "/home/beispiel/a38/onepassword-token-firma"},
	}
	if !reflect.DeepEqual(cfg.Backend.OnePassword.Accounts, want) {
		t.Errorf("accounts = %+v, want %+v", cfg.Backend.OnePassword.Accounts, want)
	}
	if v, _ := cfg.Vault("datenbank-dev"); v.Account != "firma" {
		t.Errorf("account of datenbank-dev = %q, want firma", v.Account)
	}
	if got := cfg.VaultsOf("privat"); !reflect.DeepEqual(got, []string{"beispiel.app.k8s-dev"}) {
		t.Errorf("VaultsOf(privat) = %v", got)
	}
	if a, ok := cfg.Account("firma"); !ok || a.TokenSource != TokenSourceFile {
		t.Errorf("Account(firma) = %+v, %v", a, ok)
	}
	if got := cfg.UnusedAccounts(); len(got) != 0 {
		t.Errorf("UnusedAccounts = %v, want none", got)
	}

	unused, err := Parse([]byte(twoAccounts + "      - name: reserve\n"))
	if err != nil {
		t.Fatalf("Parse with an account without vault: %v", err)
	}
	if got := unused.UnusedAccounts(); !reflect.DeepEqual(got, []string{"reserve"}) {
		t.Errorf("UnusedAccounts = %v, want [reserve]", got)
	}

	next := `a38 config onepassword-token set --account`
	// T43: steps 1 to 5 of D75 in the message, so that the human knows
	// that the token must be stored again for the account.
	steps := []string{
		"backend.onepassword.accounts",
		`add "account: privat" to each vault`,
		`check the file with "a38 config validate"`,
		`store the token again with "a38 config onepassword-token set --account privat" (the old keychain item is not taken over)`,
		`restart the daemon and check it with "a38 doctor"`,
	}
	rejects := []struct {
		name string
		yaml string
		want []string
	}{
		{"vault without account", "version: 1\nvaults:\n  - name: beispiel.app.k8s-dev\n    environment: dev\n",
			[]string{`vault "beispiel.app.k8s-dev": the account is missing`, "backend.onepassword.accounts"}},
		{"unknown account", "version: 1\nvaults:\n  - name: beispiel.app.k8s-dev\n    environment: dev\n    account: privat\n",
			[]string{`the account "privat" is not in backend.onepassword.accounts`}},
		{"old token_source", "version: 1\nbackend:\n  onepassword:\n    token_source: keychain\n",
			append([]string{"backend.onepassword.token_source: this key is from the configuration with one token", next}, steps...)},
		// A typical old file has no token_source key: the keychain was the
		// default. The vaults have no account, and the list is empty.
		{"old file without token_source", "version: 1\nvaults:\n  - name: beispiel.app.k8s-dev\n    environment: dev\n",
			append([]string{`vault "beispiel.app.k8s-dev": the account is missing`, "the configuration names no 1Password account; it is in the format with one token"}, steps...)},
		{"old token_file", "version: 1\nbackend:\n  onepassword:\n    token_file: ~/a38/token\n",
			[]string{"backend.onepassword.token_file: this key is from the configuration with one token", next}},
		{"name twice", accountsYAML("- name: privat", "- name: privat"), []string{`account "privat": the name is in the list more than once`}},
		{"upper case", accountsYAML("- name: Privat"), []string{"backend.onepassword.accounts[0]: the name"}},
		{"leading hyphen", accountsYAML("- name: -privat"), []string{"backend.onepassword.accounts[0]: the name"}},
		{"too long", accountsYAML("- name: " + strings.Repeat("a", 33)), []string{"backend.onepassword.accounts[0]: the name"}},
		{"empty name", accountsYAML("- token_source: keychain"), []string{"backend.onepassword.accounts[0]: the name"}},
		{"reserved name", accountsYAML("- name: service-account"), []string{`"service-account" is reserved`}},
		{"same token file", accountsYAML("- name: privat\n        token_source: file\n        token_file: /tmp/t",
			"- name: firma\n        token_source: file\n        token_file: /tmp/t"), []string{`account "firma": the token file is the token file of the account "privat"`}},
		{"unknown source", accountsYAML("- name: privat\n        token_source: env"), []string{`account "privat": token_source: the value must be "keychain" or "file"`}},
		{"unknown key", accountsYAML("- name: privat\n        token: x"), []string{"token"}},
	}
	for _, tc := range rejects {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatal("Parse succeeded, want an error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// accountsYAML returns a file with the list items under
// backend.onepassword.accounts and no vault.
func accountsYAML(items ...string) string {
	var b strings.Builder
	b.WriteString("version: 1\nbackend:\n  onepassword:\n    accounts:\n")
	for _, it := range items {
		b.WriteString("      " + it + "\n")
	}
	return b.String()
}

func TestValidAccountName(t *testing.T) {
	// T43 (part): the rule for the name of an account (D71).
	for name, want := range map[string]bool{
		"privat": true, "firma": true, "a": true, "konto-2": true, "2konto": true,
		strings.Repeat("a", 32): true, strings.Repeat("a", 33): false,
		"": false, "Privat": false, "-a": false, "a_b": false, "a b": false, "ä": false,
		LegacyAccount: false,
	} {
		if got := ValidAccountName(name) == nil; got != want {
			t.Errorf("ValidAccountName(%q) ok = %v, want %v", name, got, want)
		}
	}
}
