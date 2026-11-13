package service

import (
	"strings"
	"testing"
)

// exits are the ways the daemon ends: each exit code of the convention, an
// unknown code, SIGTERM, SIGKILL and the crash signals.
func exits() []Exit {
	var e []Exit
	for _, m := range ExitCodes {
		e = append(e, Exit{Code: m.Code})
	}
	return append(e, Exit{Code: 78}, Exit{Signal: 15}, Exit{Signal: 9}, Exit{Signal: 6}, Exit{Signal: 11})
}

func TestGeneratedPlistNeverRestarts(t *testing.T) {
	// D48, F15: launchd never restarts the daemon by itself. A restart
	// would lift the global lock without the human, and a configuration
	// error would loop.
	p, err := ParsePlist(goldenSpec().Render())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range exits() {
		if Restarts(p, e) {
			t.Errorf("launchd restarts the daemon after %+v (%s)", e, Explain(e))
		}
	}
}

func TestRestartsFollowsLaunchdPlist(t *testing.T) {
	// The model of KeepAlive in Restarts follows launchd.plist(5); the
	// table shows why the plist does not use the other forms.
	plist := func(body string) Plist {
		p, err := ParsePlist([]byte(`<plist version="1.0"><dict>` + body + `</dict></plist>`))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	configError := Exit{Code: 1}
	tests := []struct {
		name string
		p    Plist
		e    Exit
		want bool
	}{
		{"no KeepAlive", plist(""), configError, false},
		{"KeepAlive true loops on a configuration error", plist(`<key>KeepAlive</key><true/>`), configError, true},
		{"KeepAlive true restarts after a normal stop", plist(`<key>KeepAlive</key><true/>`), Exit{}, true},
		{"SuccessfulExit false loops on a configuration error", plist(`<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>`), configError, true},
		{"SuccessfulExit false keeps a normal stop", plist(`<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>`), Exit{}, false},
		{"Crashed true restarts after SIGSEGV", plist(`<key>KeepAlive</key><dict><key>Crashed</key><true/></dict>`), Exit{Signal: 11}, true},
		{"Crashed true keeps exit code 1", plist(`<key>KeepAlive</key><dict><key>Crashed</key><true/></dict>`), configError, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Restarts(tt.p, tt.e); got != tt.want {
				t.Errorf("Restarts = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExplain(t *testing.T) {
	tests := []struct {
		e    Exit
		want string
	}{
		{Exit{Code: 0}, "stopped normally"},
		{Exit{Code: 1}, "configuration"},
		{Exit{Code: 2}, "panic"},
		{Exit{Code: 42}, "unknown"},
		{Exit{Signal: 11}, "crashed"},
		{Exit{Signal: 9}, "from outside"},
	}
	for _, tt := range tests {
		if got := Explain(tt.e); !strings.Contains(got, tt.want) {
			t.Errorf("Explain(%+v) = %q, want %q", tt.e, got, tt.want)
		}
	}
}
