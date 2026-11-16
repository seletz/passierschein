// Package onepassword is the 1Password backend (F12). It reads names and
// values through the official Go SDK of 1Password with the token of a
// service account (D1, D39).
//
// # Build
//
// Only the file sdk_darwin.go imports the SDK. On each other platform
// [New] works, but the first call fails with "not available on this
// platform". The command a38 imports this package only in its darwin
// build, so the linux build of a38 contains neither this package nor the
// SDK (D2, D42, T13). The logic of the package has no build constraint, so
// that its unit tests run on the linux CI runner too.
//
// # Names (D12, D27, G23)
//
// The backend asks the SDK only for the vaults that the configuration
// lists. A vault that the service account can see but the configuration
// does not list does not exist for a38; the backend never lists its items.
// Vault, item and field are compared exactly with the titles in 1Password,
// with no normalisation, as in the fake backend. The item segment is a
// title, never an item ID. The field segment is the label of a field; the
// field can be in any section of the item. A field segment with "?" (an
// attribute such as "?attribute=otp") is out of scope (G25) and is
// not_found.
//
// # Stable IDs (G30)
//
// Resolve finds the IDs of vault, item and field and pins them to the
// reference for the life of the daemon. Read uses only the pinned IDs, so
// it reads the item that the human approved, also if somebody renamed it
// or created another item with the same title. If a later Resolve of the
// same reference finds another item, the backend fails closed with
// backend_unavailable; a restart of the daemon (which ends all grants,
// D33) accepts the new item. Two items, two fields or two vaults with the
// same title also end with backend_unavailable, never with not_found: the
// reference exists, but the backend cannot say which entry it means. The
// log names the vault and the title, so that the human can rename one.
//
// # Errors
//
// Each error wraps [backend.ErrLocked], [backend.ErrUnavailable] or
// [backend.ErrNotFound]. A rejected token is locked: the human must act. A
// network error, a rate limit, a time limit and each unknown error are
// unavailable (fail closed). No error and no log line contains a value,
// the token or the text of an SDK error. The log names the class of the
// error. For the classes unknown, input and integration_info it adds a
// hint: the words of the SDK message that are on the fixed list
// hintWords, never another part of the message (issue 89).
//
// # Connection
//
// [New] does not connect. The first call builds the client of the SDK. If
// that fails (bad token, no network), the call fails, and the backend
// tries again at the first call after RetryAfter. So the daemon runs and
// says why it does not deliver, and a38 doctor shows the state.
package onepassword

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
)

// Defaults of [Options].
const (
	// DefaultCallTimeout is the time limit of one SDK call.
	DefaultCallTimeout = 20 * time.Second
	// DefaultRetryAfter is the time between two attempts to build the
	// client of the SDK after a failure.
	DefaultRetryAfter = 30 * time.Second
)

// Options configures a [Backend].
type Options struct {
	// Account is the name of the 1Password account (D71). Each log line of
	// the backend names it; empty for a backend without a name.
	Account string
	// Vaults lists the vault names of the configuration (D12). The
	// backend asks for no other vault.
	Vaults []string
	// Token is the token of the service account.
	Token token.Token
	// Version is the version of a38. The SDK sends it to 1Password as
	// the version of the integration.
	Version string
	// Logger receives warnings without values. nil discards them.
	Logger *slog.Logger
	// CallTimeout is the time limit of one SDK call; zero means
	// DefaultCallTimeout.
	CallTimeout time.Duration
	// RetryAfter is the time between two attempts to connect; zero means
	// DefaultRetryAfter.
	RetryAfter time.Duration
	// Now returns the time; nil means time.Now.
	Now func() time.Time
}

// ids are the stable IDs of one reference.
type ids struct {
	vault, item, field string
}

// Backend is the 1Password backend. Create it with [New]. All methods are
// safe for concurrent use.
type Backend struct {
	allowed map[string]bool
	order   []string
	token   token.Token
	version string
	log     *slog.Logger
	timeout time.Duration
	retry   time.Duration
	now     func() time.Time
	connect connectFunc

	// connMu serializes the building of the client.
	connMu  sync.Mutex
	api     api
	lastErr error
	nextTry time.Time

	mu        sync.Mutex
	vaultPins map[string]string
	pins      map[protocol.Reference]ids
}

var _ backend.Backend = (*Backend)(nil)

// New returns the backend. It does not connect to 1Password.
func New(opts Options) (*Backend, error) {
	return newBackend(opts, connectSDK)
}

func newBackend(opts Options, connect connectFunc) (*Backend, error) {
	if opts.Token.Empty() {
		return nil, errors.New("1password backend: the token is empty")
	}
	if len(opts.Vaults) == 0 {
		return nil, errors.New("1password backend: the configuration lists no vault")
	}
	b := &Backend{
		allowed:   map[string]bool{},
		token:     opts.Token,
		version:   opts.Version,
		log:       opts.Logger,
		timeout:   opts.CallTimeout,
		retry:     opts.RetryAfter,
		now:       opts.Now,
		connect:   connect,
		vaultPins: map[string]string{},
		pins:      map[protocol.Reference]ids{},
	}
	for _, v := range opts.Vaults {
		if !b.allowed[v] {
			b.order = append(b.order, v)
		}
		b.allowed[v] = true
	}
	if b.log == nil {
		b.log = slog.New(slog.DiscardHandler)
	}
	if opts.Account != "" {
		b.log = b.log.With("account", opts.Account)
	}
	if b.timeout <= 0 {
		b.timeout = DefaultCallTimeout
	}
	if b.retry <= 0 {
		b.retry = DefaultRetryAfter
	}
	if b.now == nil {
		b.now = time.Now
	}
	b.version = integrationVersion(b.version)
	return b, nil
}

// Error is an error of the backend. It names the operation and the class
// of the failure. It never holds the text of the SDK.
type Error struct {
	// Op is the operation, for example "list items".
	Op string
	// Class is the class of the failure, for example "authentication".
	Class string
	// Text is the finding in one sentence, without a value.
	Text string
	// Kind is backend.ErrLocked, backend.ErrUnavailable or
	// backend.ErrNotFound.
	Kind error
	// hint holds the words of the SDK message that are on the list of
	// hintWords, for the log; never another part of the message.
	hint string
}

func (e *Error) Error() string {
	return fmt.Sprintf("1password backend: %s: %s (%s)", e.Op, e.Text, e.Class)
}

// Unwrap returns the kind, so that errors.Is finds it.
func (e *Error) Unwrap() error { return e.Kind }

// The classes of a failure.
const (
	classAuth      = "authentication"
	classNetwork   = "network"
	classRateLimit = "rate_limit"
	classTimeout   = "timeout"
	classNotFound  = "not_found"
	classUnknown   = "unknown"
	classAmbiguous = "not_unique"
	classChanged   = "changed"
	classScope     = "out_of_scope"
	classPlatform  = "platform"
	// classIntegration: the SDK rejects the integration name or version
	// that a38 sends (issue 89). This is a fault of a38, not of the human.
	classIntegration = "integration_info"
	// classInput: the SDK reports other invalid input of a38.
	classInput = "input"
)

// maxIntegrationVersion is the longest integration version that the SDK
// accepts (SDK v0.4.1: "integration version was too long. Max allowed: 20
// chars").
const maxIntegrationVersion = 20

// allowedIntegrationRune reports whether the SDK accepts r in an
// integration version: letters, digits, space, "-", "." and "_" (measured
// with SDK v0.4.1 for each printable ASCII character).
func allowedIntegrationRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return r == ' ' || r == '-' || r == '.' || r == '_'
}

// integrationVersion maps the version of a38 to an integration version
// that the SDK accepts. "SNAPSHOT" becomes "snap", each other character
// becomes "_", and the result is cut to maxIntegrationVersion. An empty
// version becomes "dev". The version only names a38 to 1Password; the
// daemon logs and the audit log keep the full version.
func integrationVersion(v string) string {
	v = strings.ReplaceAll(v, "SNAPSHOT", "snap")
	v = strings.Map(func(r rune) rune {
		if allowedIntegrationRune(r) {
			return r
		}
		return '_'
	}, v)
	if v == "" {
		return "dev"
	}
	if r := []rune(v); len(r) > maxIntegrationVersion {
		v = string(r[:maxIntegrationVersion])
	}
	return v
}

// hintWords are the words of an SDK message that the log may show for an
// error of class unknown or input. Each is a fixed word of the SDK, never
// a name, an ID or a value. The hint shows them in this order.
var hintWords = []string{
	"invalid user input", "integration name", "integration version", "too long", "unallowed char",
	"not specified", "service account token", "invalid format", "base64", "region", "account",
	"vault", "item", "field", "permission", "plugin", "wasm", "memory", "serializ", "unsupported",
	"internal error", "rate limit", "version",
}

// hintOf returns the words of hintWords that msg contains, separated by
// ", "; empty if none.
func hintOf(msg string) string {
	msg = strings.ToLower(msg)
	var found []string
	for _, w := range hintWords {
		if strings.Contains(msg, w) {
			found = append(found, w)
		}
	}
	return strings.Join(found, ", ")
}

func notFound(op, text string) error {
	return &Error{Op: op, Class: classNotFound, Text: text, Kind: backend.ErrNotFound}
}

// classify turns an error of the SDK into an *Error. It reads the text of
// the SDK error to find the class, and drops the text. The SDK has typed
// errors only for the rate limit; the other classes depend on the words
// of the message, so each unknown message is unavailable (fail closed).
func classify(op string, err error) *Error {
	if errors.Is(err, errRateLimit) {
		return &Error{Op: op, Class: classRateLimit, Text: "1Password limits the number of requests; try again later", Kind: backend.ErrUnavailable}
	}
	if errors.Is(err, errNotAvailable) {
		return &Error{Op: op, Class: classPlatform, Text: errNotAvailable.Error(), Kind: backend.ErrUnavailable}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Op: op, Class: classTimeout, Text: "1Password did not answer within the time limit", Kind: backend.ErrUnavailable}
	}
	msg := strings.ToLower(err.Error())
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(msg, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("integration name", "integration version"):
		return &Error{Op: op, Class: classIntegration, Text: "the 1Password SDK rejects the integration name or version that a38 sends; this is a fault of a38, report it", Kind: backend.ErrUnavailable, hint: hintOf(msg)}
	case has("unauthorized", "unauthenticated", "authenticat", "service account token", "invalid token",
		"token is invalid", "expired", "revoked", "forbidden", "invalid user authorization", "sign in", "signin"):
		return &Error{Op: op, Class: classAuth, Text: "1Password rejects the token of the service account; replace the token file and restart the daemon", Kind: backend.ErrLocked}
	case has("invalid user input"):
		return &Error{Op: op, Class: classInput, Text: "the 1Password SDK rejects input of a38; the log names the words of the SDK message", Kind: backend.ErrUnavailable, hint: hintOf(msg)}
	case has("not found", "does not exist", "doesn't exist", "no such item", "no such vault") && !has("no such host"):
		return &Error{Op: op, Class: classNotFound, Text: "the entry does not exist in 1Password", Kind: backend.ErrNotFound}
	case has("no such host", "connection refused", "connection reset", "network", "dial tcp", "timeout", "timed out", "tls", "eof", "unreachable"):
		return &Error{Op: op, Class: classNetwork, Text: "1Password cannot be reached", Kind: backend.ErrUnavailable}
	}
	return &Error{Op: op, Class: classUnknown, Text: "the call of the 1Password SDK failed", Kind: backend.ErrUnavailable, hint: hintOf(msg)}
}

// fail logs the class of a failed call and returns the error.
func (b *Backend) fail(e *Error) error {
	attrs := []any{"op", e.Op, "class", e.Class, "code", string(backend.Code(e))}
	if e.hint != "" {
		attrs = append(attrs, "hint", e.hint)
	}
	b.log.Warn("the 1password backend failed", attrs...)
	return e
}

// client returns the client of the SDK. It builds the client at the first
// call and after a failure at most once per retry interval.
func (b *Backend) client(ctx context.Context) (api, error) {
	b.connMu.Lock()
	defer b.connMu.Unlock()
	if b.api != nil {
		return b.api, nil
	}
	if b.lastErr != nil && b.now().Before(b.nextTry) {
		return nil, b.lastErr
	}
	a, err := guard(ctx, b.timeout, func(ctx context.Context) (api, error) {
		return b.connect(ctx, b.token, b.version)
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		b.lastErr = b.fail(classify("connect", err))
		b.nextTry = b.now().Add(b.retry)
		return nil, b.lastErr
	}
	b.api, b.lastErr = a, nil
	b.log.Info("the 1password backend is connected")
	return a, nil
}

// guard runs f with a time limit. It returns at the time limit also if f
// does not obey its context; a late result of f is dropped.
func guard[T any](ctx context.Context, limit time.Duration, f func(context.Context) (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	cctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	ch := make(chan result, 1)
	go func() {
		v, err := f(cctx)
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil && cctx.Err() != nil {
			var zero T
			return zero, cctx.Err()
		}
		return r.v, r.err
	case <-cctx.Done():
		var zero T
		return zero, cctx.Err()
	}
}

// call runs one SDK call with the time limit and maps its error. The end
// of ctx gives the error of ctx, so that the caller sees that the request
// ended and not the backend.
func call[T any](ctx context.Context, b *Backend, op string, f func(context.Context, api) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	a, err := b.client(ctx)
	if err != nil {
		return zero, err
	}
	v, err := guard(ctx, b.timeout, func(ctx context.Context) (T, error) { return f(ctx, a) })
	if err != nil {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		return zero, b.fail(classify(op, err))
	}
	return v, nil
}

// ambiguous logs and returns the error for a title that is not unique.
func (b *Backend) ambiguous(op, what string, attrs ...any) error {
	b.log.Warn("the 1password backend found a title that is not unique; rename one entry in 1Password",
		append([]any{"op", op, "what", what}, attrs...)...)
	return &Error{Op: op, Class: classAmbiguous, Kind: backend.ErrUnavailable,
		Text: "the title of the " + what + " is not unique; rename one in 1Password"}
}

// Vaults returns the vaults of the configuration that the service account
// can see, in the order of the configuration. It is one SDK call and reads
// no item.
func (b *Backend) Vaults(ctx context.Context) ([]string, error) {
	vs, err := call(ctx, b, "list vaults", func(ctx context.Context, a api) ([]vaultInfo, error) { return a.ListVaults(ctx) })
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, v := range vs {
		if b.allowed[v.Title] {
			seen[v.Title] = true
		}
	}
	out := []string{}
	for _, name := range b.order {
		if seen[name] {
			out = append(out, name)
		}
	}
	return out, nil
}

// vaultID returns the ID of a vault of the configuration. It lists the
// vaults once and pins the ID for the life of the daemon.
func (b *Backend) vaultID(ctx context.Context, name string) (string, error) {
	b.mu.Lock()
	id, ok := b.vaultPins[name]
	b.mu.Unlock()
	if ok {
		return id, nil
	}
	vs, err := call(ctx, b, "list vaults", func(ctx context.Context, a api) ([]vaultInfo, error) { return a.ListVaults(ctx) })
	if err != nil {
		return "", err
	}
	var found []string
	for _, v := range vs {
		if v.Title == name {
			found = append(found, v.ID)
		}
	}
	switch len(found) {
	case 0:
		return "", notFound("resolve", "the service account cannot see the vault")
	case 1:
	default:
		return "", b.ambiguous("resolve", "vault", "vault", name)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if old, ok := b.vaultPins[name]; ok {
		return old, nil
	}
	b.vaultPins[name] = found[0]
	return found[0], nil
}

// Resolve returns the names of ref as 1Password writes them and pins the
// IDs of vault, item and field. The SDK loads the item with its values to
// show the field labels; the backend keeps only titles and IDs and drops
// the item at once.
func (b *Backend) Resolve(ctx context.Context, ref protocol.Reference) (backend.Names, error) {
	if err := ctx.Err(); err != nil {
		return backend.Names{}, err
	}
	if !b.allowed[ref.Vault] {
		// D12: the vault does not exist for a38. No SDK call.
		return backend.Names{}, notFound("resolve", "the vault is not in the configuration")
	}
	if strings.ContainsAny(ref.Field, "?=") {
		// G25: no attributes and no query in a reference.
		return backend.Names{}, &Error{Op: "resolve", Class: classScope, Kind: backend.ErrNotFound,
			Text: "a reference with an attribute (for example ?attribute=otp) is not supported"}
	}
	vid, err := b.vaultID(ctx, ref.Vault)
	if err != nil {
		return backend.Names{}, err
	}
	items, err := call(ctx, b, "list items", func(ctx context.Context, a api) ([]itemInfo, error) { return a.ListItems(ctx, vid) })
	if err != nil {
		return backend.Names{}, err
	}
	var itemIDs []string
	for _, it := range items {
		if it.Title == ref.Item {
			itemIDs = append(itemIDs, it.ID)
		}
	}
	switch len(itemIDs) {
	case 0:
		return backend.Names{}, notFound("resolve", "the vault has no item with this title")
	case 1:
	default:
		return backend.Names{}, b.ambiguous("resolve", "item", "vault", ref.Vault, "item", ref.Item, "count", len(itemIDs))
	}
	item, err := call(ctx, b, "get item", func(ctx context.Context, a api) (itemData, error) { return a.GetItem(ctx, vid, itemIDs[0]) })
	if err != nil {
		return backend.Names{}, err
	}
	if item.ID != itemIDs[0] {
		return backend.Names{}, b.fail(&Error{Op: "get item", Class: classUnknown, Kind: backend.ErrUnavailable,
			Text: "the SDK returned another item"})
	}
	var fieldIDs []string
	for _, f := range item.Fields {
		if f.Title == ref.Field {
			fieldIDs = append(fieldIDs, f.ID)
		}
	}
	item = itemData{} // drop the values at once (D34)
	switch len(fieldIDs) {
	case 0:
		return backend.Names{}, notFound("resolve", "the item has no field with this label")
	case 1:
	default:
		return backend.Names{}, b.ambiguous("resolve", "field", "vault", ref.Vault, "item", ref.Item, "field", ref.Field)
	}
	got := ids{vault: vid, item: itemIDs[0], field: fieldIDs[0]}
	b.mu.Lock()
	defer b.mu.Unlock()
	if old, ok := b.pins[ref]; ok && old != got {
		b.log.Warn("the 1password backend found another entry behind a reference that it resolved before; restart the daemon to accept it",
			"vault", ref.Vault, "item", ref.Item, "field", ref.Field)
		return backend.Names{}, &Error{Op: "resolve", Class: classChanged, Kind: backend.ErrUnavailable,
			Text: "the entry behind the reference changed since the daemon resolved it first; restart the daemon"}
	}
	b.pins[ref] = got
	return backend.Names{Vault: ref.Vault, Item: ref.Item, Field: ref.Field}, nil
}

// Read returns the value of the field with the IDs that Resolve pinned. It
// keeps no copy. A reference that Resolve did not pin is unavailable.
func (b *Backend) Read(ctx context.Context, ref protocol.Reference) (protocol.Secret, error) {
	if err := ctx.Err(); err != nil {
		return protocol.Secret{}, err
	}
	b.mu.Lock()
	pin, ok := b.pins[ref]
	b.mu.Unlock()
	if !ok {
		return protocol.Secret{}, b.fail(&Error{Op: "read", Class: classUnknown, Kind: backend.ErrUnavailable,
			Text: "the reference was not resolved in this run of the daemon"})
	}
	item, err := call(ctx, b, "get item", func(ctx context.Context, a api) (itemData, error) { return a.GetItem(ctx, pin.vault, pin.item) })
	if err != nil {
		return protocol.Secret{}, err
	}
	if item.ID != pin.item {
		return protocol.Secret{}, b.fail(&Error{Op: "read", Class: classUnknown, Kind: backend.ErrUnavailable,
			Text: "the SDK returned another item"})
	}
	for _, f := range item.Fields {
		if f.ID == pin.field {
			return f.Value, nil
		}
	}
	return protocol.Secret{}, notFound("read", "the approved item has no such field any more")
}

const text = "onepassword.Backend"

// String returns a fixed text. It shows no token and no value.
func (b *Backend) String() string { return text }

// GoString returns a fixed text.
func (b *Backend) GoString() string { return text }

// Format writes a fixed text for each verb.
func (b *Backend) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(text)) }

// LogValue returns a fixed text for package slog.
func (b *Backend) LogValue() slog.Value { return slog.StringValue(text) }
