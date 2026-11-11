// Package clock gives the daemon its time source. Production code uses
// [System]. A test uses [Fake] and moves the time itself, so that a bundle
// window, an approval timeout or a grant lifetime does not need real time.
//
// Code of the daemon never calls time.Now, time.After or time.NewTimer
// directly. It receives a Clock.
package clock

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Clock is a time source.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// NewTimer returns a timer that fires once after d.
	NewTimer(d time.Duration) Timer
}

// Timer fires once.
type Timer interface {
	// C returns the channel that receives the fire time. The channel has a
	// buffer of one, so the clock never blocks on a reader.
	C() <-chan time.Time
	// Stop prevents the timer from firing. It returns true if the call
	// stopped the timer, and false if the timer already fired or stopped.
	Stop() bool
}

// System is the real clock of the operating system.
type System struct{}

// Now returns time.Now().
func (System) Now() time.Time { return time.Now() }

// NewTimer returns a timer of package time.
func (System) NewTimer(d time.Duration) Timer { return systemTimer{time.NewTimer(d)} }

type systemTimer struct{ t *time.Timer }

func (s systemTimer) C() <-chan time.Time { return s.t.C }
func (s systemTimer) Stop() bool          { return s.t.Stop() }

// Fake is a clock for tests. The time moves only when the test calls
// Advance. All methods are safe for concurrent use.
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	timers  []*fakeTimer
	changed chan struct{}
}

// NewFake returns a Fake that shows the time start.
func NewFake(start time.Time) *Fake {
	return &Fake{now: start, changed: make(chan struct{})}
}

// Now returns the time of the fake clock.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// NewTimer returns a timer that fires when Advance moves the clock to or
// past its end time. A duration of zero or less fires on the next Advance,
// also on Advance(0).
func (f *Fake) NewTimer(d time.Duration) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{clock: f, when: f.now.Add(max(d, 0)), c: make(chan time.Time, 1)}
	f.timers = append(f.timers, t)
	f.notify()
	return t
}

// Advance moves the clock forward by d and fires each timer whose end time
// is reached, in the sequence of the end times. A negative d does nothing.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d < 0 {
		return
	}
	f.now = f.now.Add(d)
	sort.SliceStable(f.timers, func(i, j int) bool { return f.timers[i].when.Before(f.timers[j].when) })
	kept := f.timers[:0]
	for _, t := range f.timers {
		if t.when.After(f.now) {
			kept = append(kept, t)
			continue
		}
		t.c <- t.when
	}
	f.timers = kept
	f.notify()
}

// Pending returns the number of timers that did not fire and did not stop.
func (f *Fake) Pending() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.timers)
}

// BlockUntil waits until exactly n timers are pending. A test calls it
// before Advance, so that the code under test has started its timer. It
// returns the error of ctx if ctx ends first.
func (f *Fake) BlockUntil(ctx context.Context, n int) error {
	for {
		f.mu.Lock()
		pending, changed := len(f.timers), f.changed
		f.mu.Unlock()
		if pending == n {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// notify wakes each BlockUntil. The caller holds f.mu.
func (f *Fake) notify() {
	close(f.changed)
	f.changed = make(chan struct{})
}

type fakeTimer struct {
	clock *Fake
	when  time.Time
	c     chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time { return t.c }

func (t *fakeTimer) Stop() bool {
	f := t.clock
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, other := range f.timers {
		if other == t {
			f.timers = append(f.timers[:i], f.timers[i+1:]...)
			f.notify()
			return true
		}
	}
	return false
}
