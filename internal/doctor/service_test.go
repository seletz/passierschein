package doctor

import (
	"context"
	"strings"
	"testing"
)

func TestServiceCheck(t *testing.T) {
	// F15, F20: the doctor reports the LaunchAgent. Not installed is no
	// finding; a service that does not run or starts another program is.
	installed := ServiceState{
		PlistPath: "/Users/beispiel/Library/LaunchAgents/io.github.seletz.passierschein.daemon.plist",
		Installed: true, Loaded: true, Running: true, PID: 4242,
		Program: "/Applications/Passierschein.app/Contents/MacOS/a38", Current: "/Applications/Passierschein.app/Contents/MacOS/a38",
		LogPath: "/Users/beispiel/Library/Logs/a38/serve.log",
	}
	tests := []struct {
		name   string
		change func(s *ServiceState)
		wantOK bool
		want   string
		next   string
	}{
		{"not installed", func(s *ServiceState) { *s = ServiceState{} }, true, "not installed", ""},
		{"running", func(*ServiceState) {}, true, "pid 4242", ""},
		{"not running", func(s *ServiceState) {
			s.Running, s.PID, s.LastExit = false, 0, "exit code 1: the daemon did not start"
		}, false, "exit code 1", "serve.log"},
		{"not loaded", func(s *ServiceState) { s.Loaded, s.Running = false, false }, false, "exists", "a38 service restart"},
		{"disabled", func(s *ServiceState) { s.Disabled, s.Loaded, s.Running = true, false, false }, false, "Login Items", "Login Items"},
		{"program missing", func(s *ServiceState) { s.ProgramMissing = true }, false, "does not exist", "a38 service install"},
		{"other program", func(s *ServiceState) {
			s.ProgramDiffers, s.Current = true, "/Users/beispiel/.local/bin/a38"
		}, false, "/Users/beispiel/.local/bin/a38", "a38 service install"},
		{"foreign plist", func(s *ServiceState) { s.Foreign = true }, false, "LaunchAgents", "--force"},
		{"launchd error", func(s *ServiceState) { s.Error = "launchctl print did not end" }, false, "did not end", "a38 service status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := installed
			tt.change(&s)
			env := SystemEnv{ServiceState: func(context.Context) *ServiceState { return &s }}
			res := serviceCheck{env}.Run(context.Background(), &State{})
			if len(res) != 1 || res[0].OK != tt.wantOK {
				t.Fatalf("result = %+v", res)
			}
			if got := res[0].Detail + res[0].Cause; !strings.Contains(got, tt.want) {
				t.Errorf("text %q does not contain %q", got, tt.want)
			}
			if !tt.wantOK && !strings.Contains(res[0].NextStep, tt.next) {
				t.Errorf("next step %q does not contain %q", res[0].NextStep, tt.next)
			}
		})
	}
}

func TestServiceCheckSilentWithoutLaunchd(t *testing.T) {
	// D53: without launchd (linux) the check says nothing.
	if res := (serviceCheck{SystemEnv{}}).Run(context.Background(), &State{}); res != nil {
		t.Errorf("result = %+v", res)
	}
}
