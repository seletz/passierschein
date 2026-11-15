package onepassword

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
)

// Account is one 1Password account of [AccountsOptions] (D71, D73).
type Account struct {
	// Name is the name of the account in the configuration.
	Name string
	// Vaults lists the vaults of the configuration that name this
	// account.
	Vaults []string
	// Token is the token of the service account of the account. It is
	// empty if the daemon could not read it at its start; Locked then
	// says why.
	Token token.Token
	// Locked says, without a value, why the account has no token, for
	// example "the keychain has no item a38.onepassword/firma". Each
	// request for a vault of the account ends with backend_locked.
	Locked string
}

// AccountsOptions configures an [Accounts] backend. The fields other than
// Accounts apply to each account, as in [Options].
type AccountsOptions struct {
	// Accounts lists the accounts in the order of the configuration.
	Accounts []Account
	// Version is the version of a38.
	Version string
	// Logger receives warnings without values. nil discards them.
	Logger *slog.Logger
	// CallTimeout is the time limit of one SDK call.
	CallTimeout time.Duration
	// RetryAfter is the time between two attempts to connect.
	RetryAfter time.Duration
	// Now returns the time; nil means time.Now.
	Now func() time.Time
}

// Accounts is the 1Password backend for more than one account (F25). It
// holds one [Backend], and so one client of the SDK, for each account with
// a token. It sends each request through the vault of the reference to the
// account of the vault (D73). An account without token locks only its
// vaults. All methods are safe for concurrent use.
type Accounts struct {
	order   []string
	byName  map[string]*accountState
	byVault map[string]*accountState
	log     *slog.Logger
}

type accountState struct {
	name    string
	vaults  []string
	locked  string
	backend *Backend
}

var _ backend.Backend = (*Accounts)(nil)

// NewAccounts returns the backend for the accounts. It does not connect to
// 1Password.
func NewAccounts(opts AccountsOptions) (*Accounts, error) {
	return newAccounts(opts, func(string) connectFunc { return connectSDK })
}

func newAccounts(opts AccountsOptions, connect func(account string) connectFunc) (*Accounts, error) {
	if len(opts.Accounts) == 0 {
		return nil, errors.New("1password backend: the configuration names no account with a vault")
	}
	a := &Accounts{byName: map[string]*accountState{}, byVault: map[string]*accountState{}, log: opts.Logger}
	if a.log == nil {
		a.log = slog.New(slog.DiscardHandler)
	}
	for _, acc := range opts.Accounts {
		if _, dup := a.byName[acc.Name]; dup || acc.Name == "" {
			return nil, fmt.Errorf("1password backend: the account %q is empty or in the list more than once", acc.Name)
		}
		if len(acc.Vaults) == 0 {
			return nil, fmt.Errorf("1password backend: the account %q has no vault", acc.Name)
		}
		st := &accountState{name: acc.Name, vaults: acc.Vaults, locked: acc.Locked}
		for _, v := range acc.Vaults {
			if other, dup := a.byVault[v]; dup {
				return nil, fmt.Errorf("1password backend: the vault %q is in the accounts %q and %q", v, other.name, acc.Name)
			}
			a.byVault[v] = st
		}
		switch {
		case !acc.Token.Empty():
			b, err := newBackend(Options{
				Account: acc.Name, Vaults: acc.Vaults, Token: acc.Token, Version: opts.Version, Logger: a.log,
				CallTimeout: opts.CallTimeout, RetryAfter: opts.RetryAfter, Now: opts.Now,
			}, connect(acc.Name))
			if err != nil {
				return nil, err
			}
			st.backend, st.locked = b, ""
		case acc.Locked == "":
			return nil, fmt.Errorf("1password backend: the account %q has no token and no reason", acc.Name)
		default:
			a.log.Warn("the 1password account has no token; each request for its vaults ends with backend_locked",
				"account", acc.Name, "reason", acc.Locked)
		}
		a.byName[acc.Name] = st
		a.order = append(a.order, acc.Name)
	}
	return a, nil
}

// lockedErr returns the error for a request to an account without token.
func (st *accountState) lockedErr(op string) error {
	return &Error{Op: op, Class: "no_token", Kind: backend.ErrLocked,
		Text: fmt.Sprintf("the account %q has no token: %s; store it and restart the daemon", st.name, st.locked)}
}

// route returns the account of the vault of ref.
func (a *Accounts) route(op string, vault string) (*accountState, error) {
	st, ok := a.byVault[vault]
	if !ok {
		// D12: the vault does not exist for a38. No SDK call.
		return nil, notFound(op, "the vault is not in the configuration")
	}
	if st.backend == nil {
		return nil, st.lockedErr(op)
	}
	return st, nil
}

// Vaults returns the vaults of the configuration that the service
// accounts can see, account by account in the order of the configuration.
// An account that fails adds no vault. Only if no account gives an answer,
// Vaults returns the error of the first account.
func (a *Accounts) Vaults(ctx context.Context) ([]string, error) {
	out := []string{}
	var first error
	answered := false
	for _, name := range a.order {
		st := a.byName[name]
		if st.backend == nil {
			if first == nil {
				first = st.lockedErr("list vaults")
			}
			continue
		}
		vs, err := st.backend.Vaults(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if first == nil {
				first = err
			}
			continue
		}
		answered = true
		out = append(out, vs...)
	}
	if !answered {
		return nil, first
	}
	return out, nil
}

// Resolve sends the request to the account of the vault.
func (a *Accounts) Resolve(ctx context.Context, ref protocol.Reference) (backend.Names, error) {
	if err := ctx.Err(); err != nil {
		return backend.Names{}, err
	}
	st, err := a.route("resolve", ref.Vault)
	if err != nil {
		return backend.Names{}, err
	}
	return st.backend.Resolve(ctx, ref)
}

// Read sends the request to the account of the vault.
func (a *Accounts) Read(ctx context.Context, ref protocol.Reference) (protocol.Secret, error) {
	if err := ctx.Err(); err != nil {
		return protocol.Secret{}, err
	}
	st, err := a.route("read", ref.Vault)
	if err != nil {
		return protocol.Secret{}, err
	}
	return st.backend.Read(ctx, ref)
}

// AccountState is the state of one account for the log after the start.
type AccountState struct {
	// Name is the name of the account.
	Name string
	// Configured is the number of vaults of the account.
	Configured int
	// Visible is the number of these vaults that the service account can
	// see; zero if Err is set.
	Visible int
	// Err is the failure of the account; nil if it works.
	Err error
}

// Check asks each account for its vaults. It reads no item. The daemon
// logs the result after its start (D73).
func (a *Accounts) Check(ctx context.Context) []AccountState {
	out := make([]AccountState, 0, len(a.order))
	for _, name := range a.order {
		st := a.byName[name]
		s := AccountState{Name: name, Configured: len(st.vaults)}
		if st.backend == nil {
			s.Err = st.lockedErr("list vaults")
		} else if vs, err := st.backend.Vaults(ctx); err != nil {
			s.Err = err
		} else {
			s.Visible = len(vs)
		}
		out = append(out, s)
	}
	return out
}

const accountsText = "onepassword.Accounts"

// String returns a fixed text. It shows no token and no value.
func (a *Accounts) String() string { return accountsText }

// GoString returns a fixed text.
func (a *Accounts) GoString() string { return accountsText }

// Format writes a fixed text for each verb.
func (a *Accounts) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(accountsText)) }

// LogValue returns a fixed text for package slog.
func (a *Accounts) LogValue() slog.Value { return slog.StringValue(accountsText) }
