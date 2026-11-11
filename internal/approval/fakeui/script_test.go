package fakeui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
)

func newScript(t *testing.T, clk clock.Clock, args ...string) *fakeui.Script {
	t.Helper()
	s, err := fakeui.NewScript(append([]string{helperPath}, args...), time.Minute, clk)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// T23 (part), T6 (part): what a script can do, and that each failure is
// never an approval.
func TestScriptApprove(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		wantOutcome approval.Outcome
		wantIDs     []string
		wantTTL     int64
	}{
		{name: "tick two", args: []string{"-tick", "0,2"}, wantOutcome: approval.OutcomeApproved, wantIDs: []string{"c1", "c3"}, wantTTL: 3600},
		{name: "tick all", args: []string{"-tick", "all"}, wantOutcome: approval.OutcomeApproved, wantIDs: []string{"c1", "c2", "c3"}, wantTTL: 3600},
		{name: "shorter lifetime", args: []string{"-tick", "1", "-ttl", "half"}, wantOutcome: approval.OutcomeApproved, wantIDs: []string{"c2"}, wantTTL: 1800},
		{name: "no tick is a denial", args: []string{"-tick", "none", "-outcome", "approved"}, wantOutcome: approval.OutcomeDenied, wantIDs: []string{}},
		{name: "deny", args: []string{"-outcome", "denied"}, wantOutcome: approval.OutcomeDenied, wantIDs: []string{}},
		{name: "entry not in request", args: []string{"-tick", "id:c1,zzz"}, wantErr: true},
		{name: "longer lifetime", args: []string{"-tick", "0", "-ttl", "plus1"}, wantErr: true},
		{name: "zero lifetime", args: []string{"-tick", "0", "-ttl", "0"}, wantErr: true},
		{name: "negative lifetime", args: []string{"-tick", "0", "-ttl", "-1"}, wantErr: true},
		{name: "garbage lifetime", args: []string{"-tick", "0", "-ttl", "abc"}, wantErr: true},
		{name: "exit with error", args: []string{"-tick", "0", "-exit", "3", "-raw", "x"}, wantErr: true},
		{name: "no json", args: []string{"-raw", "hello"}, wantErr: true},
		{name: "empty output", args: []string{"-raw", " "}, wantErr: true},
		{name: "two values", args: []string{"-raw", "{} {}"}, wantErr: true},
		{name: "approved with exit 1", args: []string{"-tick", "all", "-exit", "1"}, wantErr: true},
		{name: "outcome error", args: []string{"-outcome", "error"}, wantErr: true},
		{name: "missing program", args: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScript(t, clock.NewFake(time.Unix(0, 0)), tt.args...)
			if tt.name == "missing program" {
				var err error
				s, err = fakeui.NewScript([]string{filepath.Join(t.TempDir(), "nope")}, time.Minute, nil)
				if err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.Approve(context.Background(), sampleRequest())
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if d.Outcome == approval.OutcomeApproved {
					t.Fatal("an error came with an approval")
				}
				return
			}
			if d.Outcome != tt.wantOutcome || d.TTLSeconds != tt.wantTTL || !reflect.DeepEqual(d.ApprovedCredentialIDs, tt.wantIDs) {
				t.Errorf("decision = %+v", d)
			}
		})
	}
}

// T6 (part): a script that never answers ends with timeout. The fake clock
// moves the time.
func TestScriptTimeout(t *testing.T) {
	clk := clock.NewFake(time.Unix(0, 0))
	s := newScript(t, clk, "-hang")
	type res struct {
		d   approval.Decision
		err error
	}
	ch := make(chan res, 1)
	go func() {
		d, err := s.Approve(context.Background(), sampleRequest())
		ch <- res{d, err}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := clk.BlockUntil(ctx, 1); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	select {
	case r := <-ch:
		if r.err != nil || r.d.Outcome != approval.OutcomeTimeout || len(r.d.ApprovedCredentialIDs) != 0 {
			t.Errorf("result = %+v, %v", r.d, r.err)
		}
	case <-ctx.Done():
		t.Fatal("Approve did not return")
	}
}

func TestScriptContextEnds(t *testing.T) {
	s := newScript(t, clock.NewFake(time.Unix(0, 0)), "-hang")
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { _, err := s.Approve(ctx, sampleRequest()); errc <- err }()
	cancel()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("want an error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Approve did not return")
	}
}

// T26 (part), T28 (part): the script gets the request and the notices, so
// that a test can store and count them.
func TestScriptRecordsRequestAndNotices(t *testing.T) {
	rec := filepath.Join(t.TempDir(), "rec.jsonl")
	s := newScript(t, clock.NewFake(time.Unix(0, 0)), "-record", rec, "-tick", "all")
	if _, err := s.Approve(context.Background(), sampleRequest()); err != nil {
		t.Fatal(err)
	}
	n := approval.Notice{Version: 1, Type: approval.TypeNotice, NoticeID: "n1", Kind: approval.NoticeDaemonFault, Code: protocol.Code("daemon_faulted"), Message: "Run a38 doctor."}
	for range 2 {
		if err := s.Notify(context.Background(), n); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(rec)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("recorded %d messages, want 3", len(lines))
	}
	var req approval.Request
	if err := json.Unmarshal(lines[0], &req); err != nil || req.RequestID != "req-1" || len(req.Credentials) != 3 {
		t.Errorf("recorded request = %+v, %v", req, err)
	}
	notices := 0
	for _, l := range lines[1:] {
		var got approval.Notice
		if err := json.Unmarshal(l, &got); err == nil && got.Type == approval.TypeNotice && got.NoticeID == "n1" {
			notices++
		}
	}
	if notices != 2 {
		t.Errorf("recorded %d notices, want 2", notices)
	}
}

func TestScriptNotifyFailure(t *testing.T) {
	s := newScript(t, clock.NewFake(time.Unix(0, 0)), "-exit", "2")
	if err := s.Notify(context.Background(), approval.Notice{}); err == nil {
		t.Error("want an error for exit code 2")
	}
}

func TestNewScriptNeedsCommand(t *testing.T) {
	if _, err := fakeui.NewScript(nil, 0, nil); err == nil {
		t.Error("want an error")
	}
}
