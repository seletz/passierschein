package faultstore_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/faultstore"
	"github.com/seletz/passierschein/internal/audit/sqlite"
)

// T26 (part): a failing Append returns an error and writes nothing.
func TestFailAppends(t *testing.T) {
	ctx := context.Background()
	inner, err := sqlite.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = inner.Close() }()
	s := faultstore.New(inner)

	if _, err := s.Append(ctx, audit.Event{Type: audit.EventRequest}); err != nil {
		t.Fatalf("append without fault: %v", err)
	}
	s.FailAfter(1, nil)
	if _, err := s.Append(ctx, audit.Event{Type: audit.EventBundle}); err != nil {
		t.Fatalf("append that may pass: %v", err)
	}
	if _, err := s.Append(ctx, audit.Event{Type: audit.EventGrant}); !errors.Is(err, faultstore.ErrInjected) {
		t.Fatalf("append err = %v, want ErrInjected", err)
	}
	s.FailWhen(func(ev audit.Event) bool { return ev.Type == audit.EventDelivery })
	s.Heal()
	s.FailWhen(func(ev audit.Event) bool { return ev.Type == audit.EventDelivery })
	if _, err := s.Append(ctx, audit.Event{Type: audit.EventGrant}); err != nil {
		t.Fatalf("append of other type: %v", err)
	}
	if _, err := s.Append(ctx, audit.Event{Type: audit.EventDelivery}); err == nil {
		t.Fatal("delivery append: want error")
	}
	if got := s.Failures(); got != 2 {
		t.Errorf("Failures = %d, want 2", got)
	}
	got, err := s.List(ctx, audit.Filter{})
	if err != nil || len(got) != 3 {
		t.Fatalf("List = %d entries, err %v; want 3 (failed appends write nothing)", len(got), err)
	}
	s.Heal()
	if _, err := s.Append(ctx, audit.Event{Type: audit.EventDelivery}); err != nil {
		t.Fatalf("append after Heal: %v", err)
	}
}

// T28 (part): Probe fails while the next Append would fail, and passes
// after Heal. It writes nothing.
func TestProbe(t *testing.T) {
	ctx := context.Background()
	inner, err := sqlite.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = inner.Close() }()
	s := faultstore.New(inner)
	if err := s.Probe(ctx); err != nil {
		t.Fatalf("probe without fault: %v", err)
	}
	s.FailAppends(nil)
	if err := s.Probe(ctx); !errors.Is(err, faultstore.ErrInjected) {
		t.Fatalf("probe with fault = %v", err)
	}
	s.Heal()
	s.FailWhen(func(audit.Event) bool { return true })
	if err := s.Probe(ctx); !errors.Is(err, faultstore.ErrInjected) {
		t.Fatalf("probe with a rule = %v", err)
	}
	s.Heal()
	if err := s.Probe(ctx); err != nil {
		t.Fatalf("probe after heal: %v", err)
	}
	if v, err := inner.Verify(ctx); err != nil || v.Entries != 0 {
		t.Fatalf("probe wrote: %+v, %v", v, err)
	}
}
