// Package fake is a backend that reads vaults, items and fields from a
// YAML file. It is for tests and local development (decision D38).
//
// # File format
//
// The file has one document with the key "vaults". Each entry is a mapping
// with a "name" and a list of "items". Each item has a "name" and a list of
// "fields". Each field has a "name" and a "value". Lists keep the order of
// the file. Unknown keys are an error.
//
//	vaults:
//	  - name: beispiel.app.k8s-dev
//	    items:
//	      - name: datenbank-dev
//	        fields:
//	          - name: username
//	            value: beispiel-wert-1
//
// A name is not empty. A vault name is unique in the file. An item name is
// unique in its vault. A field name is unique in its item. Names are
// compared exactly, with no normalisation (decision D12). Resolve returns
// the names as the file writes them.
//
// # Test controls
//
// The fake counts each call of Resolve and Read, per reference and in
// total. A call counts if it reaches the lookup, also if the reference does
// not exist or the state is locked or unavailable. A call with a canceled
// context does not count. SetLocked and SetUnavailable switch the error
// states. SetValue changes a value between two reads.
//
// The fake never logs or prints a value. An error never contains a value.
// Formatting a [Backend] with fmt prints a fixed text.
package fake

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"go.yaml.in/yaml/v3"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
)

type file struct {
	Vaults []vaultEntry `yaml:"vaults"`
}

type vaultEntry struct {
	Name  string      `yaml:"name"`
	Items []itemEntry `yaml:"items"`
}

type itemEntry struct {
	Name   string       `yaml:"name"`
	Fields []fieldEntry `yaml:"fields"`
}

type fieldEntry struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Backend is the fake backend. Create it with [Load] or [Parse].
type Backend struct {
	vaults []string
	// values maps a reference to its value. Only SetValue changes it.
	values map[protocol.Reference]protocol.Secret

	mu          sync.Mutex
	locked      bool
	unavailable bool
	resolves    map[protocol.Reference]int
	reads       map[protocol.Reference]int
	nResolves   int
	nReads      int
}

var _ backend.Backend = (*Backend)(nil)

// Load reads and parses the YAML file at path. An error names the path and
// never contains a value.
func Load(path string) (*Backend, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fake backend: read file: %w", err)
	}
	b, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("fake backend: %s: %w", path, err)
	}
	return b, nil
}

// Parse parses the YAML text. An error never contains a value.
func Parse(data []byte) (*Backend, error) {
	var f file
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("the file is empty")
		}
		// The decoder error text can quote a value; report the kind only.
		return nil, errors.New("the file is not valid YAML for the fake backend")
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("the file must hold one YAML document")
	}
	if len(f.Vaults) == 0 {
		return nil, errors.New("the file has no vaults")
	}
	b := &Backend{
		values:   map[protocol.Reference]protocol.Secret{},
		resolves: map[protocol.Reference]int{},
		reads:    map[protocol.Reference]int{},
	}
	seenVault := map[string]bool{}
	for vi, v := range f.Vaults {
		if v.Name == "" {
			return nil, fmt.Errorf("vault %d has no name", vi+1)
		}
		if seenVault[v.Name] {
			return nil, fmt.Errorf("duplicate vault name %q", v.Name)
		}
		seenVault[v.Name] = true
		b.vaults = append(b.vaults, v.Name)
		seenItem := map[string]bool{}
		for ii, it := range v.Items {
			if it.Name == "" {
				return nil, fmt.Errorf("vault %q: item %d has no name", v.Name, ii+1)
			}
			if seenItem[it.Name] {
				return nil, fmt.Errorf("vault %q: duplicate item name %q", v.Name, it.Name)
			}
			seenItem[it.Name] = true
			seenField := map[string]bool{}
			for fi, fl := range it.Fields {
				if fl.Name == "" {
					return nil, fmt.Errorf("vault %q, item %q: field %d has no name", v.Name, it.Name, fi+1)
				}
				if seenField[fl.Name] {
					return nil, fmt.Errorf("vault %q, item %q: duplicate field name %q", v.Name, it.Name, fl.Name)
				}
				seenField[fl.Name] = true
				ref := protocol.Reference{Vault: v.Name, Item: it.Name, Field: fl.Name}
				b.values[ref] = protocol.NewSecret(fl.Value)
			}
		}
	}
	return b, nil
}

// state returns the error for the current error state, or nil. Locked wins
// over unavailable. The caller holds b.mu.
func (b *Backend) state() error {
	switch {
	case b.locked:
		return backend.ErrLocked
	case b.unavailable:
		return backend.ErrUnavailable
	}
	return nil
}

// Vaults returns the vault names in the order of the file.
func (b *Backend) Vaults(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.state(); err != nil {
		return nil, err
	}
	return append([]string(nil), b.vaults...), nil
}

// Resolve returns the names of ref as the file writes them. It counts as a
// name resolution.
func (b *Backend) Resolve(ctx context.Context, ref protocol.Reference) (backend.Names, error) {
	if err := ctx.Err(); err != nil {
		return backend.Names{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.resolves[ref]++
	b.nResolves++
	if err := b.state(); err != nil {
		return backend.Names{}, err
	}
	if _, ok := b.values[ref]; !ok {
		return backend.Names{}, backend.ErrNotFound
	}
	return backend.Names{Vault: ref.Vault, Item: ref.Item, Field: ref.Field}, nil
}

// Read returns the current value of ref. It counts as a value read. The
// fake keeps no copy: a value set with SetValue shows at the next call.
func (b *Backend) Read(ctx context.Context, ref protocol.Reference) (protocol.Secret, error) {
	if err := ctx.Err(); err != nil {
		return protocol.Secret{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reads[ref]++
	b.nReads++
	if err := b.state(); err != nil {
		return protocol.Secret{}, err
	}
	s, ok := b.values[ref]
	if !ok {
		return protocol.Secret{}, backend.ErrNotFound
	}
	return s, nil
}

// SetLocked switches the locked state. Each call fails with
// [backend.ErrLocked] while it is on. It wins over the unavailable state.
func (b *Backend) SetLocked(on bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.locked = on
}

// SetUnavailable switches the unavailable state. Each call fails with
// [backend.ErrUnavailable] while it is on.
func (b *Backend) SetUnavailable(on bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.unavailable = on
}

// SetValue changes the value of an existing field. It returns
// [backend.ErrNotFound] if the field does not exist. It does not count as a
// read.
func (b *Backend) SetValue(ref protocol.Reference, value string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.values[ref]; !ok {
		return backend.ErrNotFound
	}
	b.values[ref] = protocol.NewSecret(value)
	return nil
}

// Resolves returns the number of Resolve calls.
func (b *Backend) Resolves() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.nResolves
}

// ResolvesOf returns the number of Resolve calls for ref.
func (b *Backend) ResolvesOf(ref protocol.Reference) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.resolves[ref]
}

// Reads returns the number of Read calls.
func (b *Backend) Reads() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.nReads
}

// ReadsOf returns the number of Read calls for ref.
func (b *Backend) ReadsOf(ref protocol.Reference) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reads[ref]
}

// ResetCounters sets all counters to zero.
func (b *Backend) ResetCounters() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.resolves = map[protocol.Reference]int{}
	b.reads = map[protocol.Reference]int{}
	b.nResolves, b.nReads = 0, 0
}

const text = "fake.Backend"

// String returns a fixed text. It shows no value.
func (b *Backend) String() string { return text }

// GoString returns a fixed text. It shows no value.
func (b *Backend) GoString() string { return text }

// Format writes a fixed text for each verb. It shows no value.
func (b *Backend) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(text)) }
