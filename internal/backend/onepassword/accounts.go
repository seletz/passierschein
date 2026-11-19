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
		}
		// A locked account is logged once, by the start of the daemon
		// (openAccounts); each request names it in its error.
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
// It asks the accounts side by side, so that a slow account does not use
// up the time of the others (D78). An account that fails or does not
// answer before ctx ends adds no vault. Only if no account gives an
// answer, Vaults returns the error of ctx or of the first account.
func (a *Accounts) Vaults(ctx context.Context) ([]string, error) {
	results := a.askAll(ctx, func(context.Context) context.Context { return ctx })
	out := []string{}
	var first error
	answered := false
	for _, r := range results {
		if r.err != nil {
			if first == nil {
				first = r.err
			}
			continue
		}
		answered = true
		out = append(out, r.vaults...)
	}
	if !answered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, first
	}
	return out, nil
}

// askResult is the answer of one account to askAll.
type askResult struct {
	vaults []string
	err    error
	late   bool // the account gave no answer before its context ended
}

// askAll asks each account for its vaults, side by side. ctxFor gives the
// context of each account. The result has the order of the configuration.
// askAll returns when each account has answered or its context has
// ended, also if a call does not obey its context.
func (a *Accounts) askAll(ctx context.Context, ctxFor func(context.Context) context.Context) []askResult {
	out := make([]askResult, len(a.order))
	done := make(chan struct{}, len(a.order))
	for i, name := range a.order {
		st := a.byName[name]
		if st.backend == nil {
			out[i] = askResult{err: st.lockedErr("list vaults")}
			done <- struct{}{}
			continue
		}
		go func() {
			out[i] = ask(ctxFor(ctx), st.backend)
			done <- struct{}{}
		}()
	}
	for range a.order {
		<-done
	}
	return out
}

// ask asks one account for its vaults. It returns when the context ends,
// also if the call waits longer, for example for the lock of the client.
func ask(ctx context.Context, b *Backend) askResult {
	ch := make(chan askResult, 1)
	go func() {
		vs, err := b.Vaults(ctx)
		ch <- askResult{vaults: vs, err: err}
	}()
	select {
	case r := <-ch:
		if r.err != nil && ctx.Err() != nil {
			return askResult{err: ctx.Err(), late: true}
		}
		return r
	case <-ctx.Done():
		return askResult{err: ctx.Err(), late: true}
	}
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

// AccountState is the state of one account; see [backend.AccountState].
type AccountState = backend.AccountState

var _ backend.AccountChecker = (*Accounts)(nil)

// Check asks each account for its vaults, side by side, each with its own
// time limit limit (D78). It reads no item. A hanging or locked account
// does not change the state of the other accounts. An account that gives
// no answer within limit has the state backend_unavailable; the error
// names the limit. The daemon logs the result after its start and shows
// it in its status.
func (a *Accounts) Check(ctx context.Context, limit time.Duration) []AccountState {
	cancels := make(chan context.CancelFunc, len(a.order))
	results := a.askAll(ctx, func(ctx context.Context) context.Context {
		c, cancel := context.WithTimeout(ctx, limit)
		cancels <- cancel
		return c
	})
	close(cancels)
	for cancel := range cancels {
		cancel()
	}
	out := make([]AccountState, 0, len(a.order))
	for i, name := range a.order {
		st := a.byName[name]
		s := AccountState{Name: name, Configured: len(st.vaults)}
		r := results[i]
		switch {
		case r.late && ctx.Err() == nil:
			s.Err = &Error{Op: "list vaults", Class: classTimeout, Kind: backend.ErrUnavailable,
				Text: fmt.Sprintf("the account %q gave no answer within %s", name, limit)}
		case r.err != nil:
			s.Err = r.err
		default:
			s.Visible = len(r.vaults)
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
