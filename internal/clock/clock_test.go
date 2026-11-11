package clock

import (
	"context"
	"testing"
	"time"
)

var start = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func fired(tm Timer) bool {
	select {
	case <-tm.C():
		return true
	default:
		return false
	}
}

func TestFakeNowAndAdvance(t *testing.T) {
	f := NewFake(start)
	if !f.Now().Equal(start) {
		t.Errorf("Now() = %v, want %v", f.Now(), start)
	}
	f.Advance(90 * time.Minute)
	if want := start.Add(90 * time.Minute); !f.Now().Equal(want) {
		t.Errorf("Now() = %v, want %v", f.Now(), want)
	}
	f.Advance(-time.Hour)
	if want := start.Add(90 * time.Minute); !f.Now().Equal(want) {
		t.Errorf("a negative step moved the clock to %v", f.Now())
	}
}

func TestFakeTimer(t *testing.T) {
	tests := []struct {
		name      string
		duration  time.Duration
		steps     []time.Duration
		wantFired []bool
	}{
		{"fires at the end", 3 * time.Second, []time.Duration{2999 * time.Millisecond, time.Millisecond}, []bool{false, true}},
		{"fires after the end", 3 * time.Second, []time.Duration{time.Hour}, []bool{true}},
		{"zero fires at once", 0, []time.Duration{0}, []bool{true}},
		{"fires once", time.Second, []time.Duration{time.Second, time.Second}, []bool{true, false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFake(start)
			tm := f.NewTimer(tt.duration)
			for i, step := range tt.steps {
				f.Advance(step)
				if got := fired(tm); got != tt.wantFired[i] {
					t.Errorf("after step %d: fired = %v, want %v", i, got, tt.wantFired[i])
				}
			}
		})
	}
}

func TestFakeTimerValueAndOrder(t *testing.T) {
	f := NewFake(start)
	late := f.NewTimer(2 * time.Second)
	early := f.NewTimer(time.Second)
	f.Advance(5 * time.Second)
	if got := <-early.C(); !got.Equal(start.Add(time.Second)) {
		t.Errorf("early timer value = %v, want its own end time", got)
	}
	if got := <-late.C(); !got.Equal(start.Add(2 * time.Second)) {
		t.Errorf("late timer value = %v, want its own end time", got)
	}
}

func TestFakeTimerStop(t *testing.T) {
	f := NewFake(start)
	tm := f.NewTimer(time.Second)
	if !tm.Stop() {
		t.Error("Stop() of a pending timer = false")
	}
	if tm.Stop() {
		t.Error("second Stop() = true")
	}
	f.Advance(time.Minute)
	if fired(tm) {
		t.Error("a stopped timer fired")
	}
	if f.Pending() != 0 {
		t.Errorf("Pending() = %d, want 0", f.Pending())
	}
	done := f.NewTimer(time.Second)
	f.Advance(time.Second)
	if done.Stop() {
		t.Error("Stop() of a fired timer = true")
	}
}

func TestFakeBlockUntil(t *testing.T) {
	f := NewFake(start)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		f.NewTimer(time.Second)
		f.NewTimer(time.Second)
	}()
	if err := f.BlockUntil(ctx, 2); err != nil {
		t.Fatalf("BlockUntil: %v", err)
	}
	if f.Pending() != 2 {
		t.Errorf("Pending() = %d, want 2", f.Pending())
	}
	short, cancelShort := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelShort()
	if err := f.BlockUntil(short, 3); err == nil {
		t.Error("BlockUntil returned without a third timer")
	}
}

func TestSystem(t *testing.T) {
	var c Clock = System{}
	before := time.Now()
	if got := c.Now(); got.Before(before) {
		t.Errorf("Now() = %v is before %v", got, before)
	}
	tm := c.NewTimer(time.Millisecond)
	select {
	case <-tm.C():
	case <-time.After(5 * time.Second):
		t.Fatal("the system timer did not fire")
	}
	if c.NewTimer(time.Hour).Stop() != true {
		t.Error("Stop() of a pending system timer = false")
	}
}
