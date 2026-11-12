package appui_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/approval/appui"
)

// fakeSigner is a CodeSigner with fixed answers per path.
type fakeSigner struct {
	sigs      map[string]appui.CodeSignature
	descErr   map[string]error
	verifyErr map[string]error
	verified  []string // "path team" of each VerifyTeam call
}

func (f *fakeSigner) Describe(_ context.Context, path string) (appui.CodeSignature, error) {
	if err := f.descErr[path]; err != nil {
		return appui.CodeSignature{}, err
	}
	return f.sigs[path], nil
}

func (f *fakeSigner) VerifyTeam(_ context.Context, path, team string) error {
	f.verified = append(f.verified, path+" "+team)
	return f.verifyErr[path]
}

const (
	selfPath = "/Applications/Passierschein.app/Contents/MacOS/a38"
	appPath  = "/Applications/Passierschein.app/Contents/MacOS/Passierschein"
	team     = "ABCDE12345"
)

var (
	devID      = appui.CodeSignature{Signed: true, Runtime: true, Identifier: "io.github.seletz.passierschein", TeamID: team}
	adHoc      = appui.CodeSignature{Signed: true, AdHoc: true, Runtime: true, Identifier: "a38"}
	linkerSign = appui.CodeSignature{Signed: true, AdHoc: true, Identifier: "a.out"}
	unsigned   = appui.CodeSignature{}
)

// G5 (part): when a38 itself has no Developer ID signature (development,
// CI, the demo install), the check is off. The daemon logs this once at
// the start and never calls codesign for the app.
func TestSignatureVerifierSkipsWithoutDeveloperID(t *testing.T) {
	for name, self := range map[string]appui.CodeSignature{
		"ad-hoc": adHoc, "linker-signed": linkerSign, "unsigned": unsigned,
		"signed without team": {Signed: true, Runtime: true, Identifier: "a38"},
	} {
		t.Run(name, func(t *testing.T) {
			s := &fakeSigner{sigs: map[string]appui.CodeSignature{selfPath: self, appPath: unsigned}}
			var log bytes.Buffer
			v := appui.NewSignatureVerifier(context.Background(), appui.SignatureOptions{
				Self: selfPath, Signer: s, Logger: slog.New(slog.NewTextHandler(&log, nil)),
			})
			for range 3 {
				if err := v.Verify(context.Background(), appPath); err != nil {
					t.Fatalf("Verify: %v, want nil (check off)", err)
				}
			}
			if len(s.verified) != 0 {
				t.Errorf("codesign ran for the app: %v", s.verified)
			}
			if n := strings.Count(log.String(), "\n"); n != 1 {
				t.Errorf("log has %d lines, want 1:\n%s", n, log.String())
			}
			if !strings.Contains(log.String(), "signature check of the approval app is off") {
				t.Errorf("log does not say that the check is off:\n%s", log.String())
			}
		})
	}
}

// G5 (part): a38 with a Developer ID signature accepts only an app with a
// valid Developer ID signature of the same team and the hardened runtime.
func TestSignatureVerifierTeamCheck(t *testing.T) {
	tests := []struct {
		name      string
		app       appui.CodeSignature
		verifyErr error
		wantErr   string
	}{
		{name: "same team", app: devID},
		{name: "requirement fails", app: devID, verifyErr: errors.New("codesign: test-requirement failed"), wantErr: "not valid for team"},
		{name: "other team", app: appui.CodeSignature{Signed: true, Runtime: true, TeamID: "ZZZZZ99999"}, wantErr: "team ZZZZZ99999"},
		{name: "ad-hoc app", app: adHoc, wantErr: "ad-hoc"},
		{name: "unsigned app", app: unsigned, wantErr: "not signed"},
		{name: "no hardened runtime", app: appui.CodeSignature{Signed: true, TeamID: team}, wantErr: "hardened runtime"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &fakeSigner{
				sigs:      map[string]appui.CodeSignature{selfPath: devID, appPath: tt.app},
				verifyErr: map[string]error{},
			}
			if tt.verifyErr != nil {
				s.verifyErr[appPath] = tt.verifyErr
			}
			var log bytes.Buffer
			v := appui.NewSignatureVerifier(context.Background(), appui.SignatureOptions{
				Self: selfPath, Signer: s, Logger: slog.New(slog.NewTextHandler(&log, nil)),
			})
			if !strings.Contains(log.String(), team) {
				t.Errorf("start log does not name the team:\n%s", log.String())
			}
			err := v.Verify(context.Background(), appPath)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Verify: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("Verify: nil, want an error with %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("Verify: %v, want %q", err, tt.wantErr)
			}
			if len(s.verified) != 1 || s.verified[0] != appPath+" "+team {
				t.Errorf("VerifyTeam calls %v, want one for the app with the team", s.verified)
			}
		})
	}
}

// Fail closed: if the daemon cannot read its own signature, or the team
// ID of a38 is not a valid team ID, no app starts.
func TestSignatureVerifierFailsClosed(t *testing.T) {
	tests := map[string]*fakeSigner{
		"self unreadable": {descErr: map[string]error{selfPath: errors.New("codesign timed out")}},
		"bad team id":     {sigs: map[string]appui.CodeSignature{selfPath: {Signed: true, Runtime: true, TeamID: `X" or true`}}},
	}
	for name, s := range tests {
		t.Run(name, func(t *testing.T) {
			v := appui.NewSignatureVerifier(context.Background(), appui.SignatureOptions{Self: selfPath, Signer: s})
			if err := v.Verify(context.Background(), appPath); err == nil {
				t.Fatal("Verify: nil, want an error")
			}
			if len(s.verified) != 0 {
				t.Errorf("codesign ran for the app: %v", s.verified)
			}
		})
	}
}

// Without a path of a38 the daemon cannot know its own signature: fail
// closed.
func TestSignatureVerifierNoSelf(t *testing.T) {
	v := appui.NewSignatureVerifier(context.Background(), appui.SignatureOptions{Signer: &fakeSigner{}})
	if err := v.Verify(context.Background(), appPath); err == nil {
		t.Fatal("Verify: nil, want an error")
	}
}

func TestParseCodesignOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want appui.CodeSignature
	}{
		{
			name: "developer id",
			out: `Executable=/Applications/Passierschein.app/Contents/MacOS/Passierschein
Identifier=io.github.seletz.passierschein
Format=app bundle with Mach-O universal (x86_64 arm64)
CodeDirectory v=20500 size=6626 flags=0x10000(runtime) hashes=196+7 location=embedded
Signature size=9046
Authority=Developer ID Application: Beispiel (ABCDE12345)
Authority=Developer ID Certification Authority
Authority=Apple Root CA
Timestamp=12.11.2026 at 11:00:00
Info.plist entries=10
TeamIdentifier=ABCDE12345
Runtime Version=26.0.0
Sealed Resources version=2 rules=13 files=0
Internal requirements count=1 size=220
`,
			want: devID,
		},
		{
			name: "ad-hoc",
			out: `Executable=/tmp/a38
Identifier=a38
Format=Mach-O thin (arm64)
CodeDirectory v=20400 size=5000 flags=0x10002(adhoc,runtime) hashes=150+2 location=embedded
Signature=adhoc
Info.plist=not bound
TeamIdentifier=not set
`,
			want: adHoc,
		},
		{
			name: "linker-signed",
			out: `Executable=/tmp/a38
Identifier=a.out
Format=Mach-O thin (arm64)
CodeDirectory v=20400 size=5000 flags=0x20002(adhoc,linker-signed) hashes=150+0 location=embedded
Signature=adhoc
TeamIdentifier=not set
`,
			want: linkerSign,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appui.ParseCodesignOutput(tt.out); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
