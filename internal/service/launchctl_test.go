package service

import (
	"testing"
)

const printRunning = `gui/501/io.github.seletz.passierschein.daemon = {
	active count = 1
	path = /Users/beispiel/Library/LaunchAgents/io.github.seletz.passierschein.daemon.plist
	type = LaunchAgent
	state = running

	program = /Applications/Passierschein.app/Contents/MacOS/a38
	arguments = {
		/Applications/Passierschein.app/Contents/MacOS/a38
		serve
		pid = 99
	}

	inherited environment = {
		SSH_AUTH_SOCK => /private/tmp/x
	}

	domain = gui/501 [100005]
	runs = 1
	pid = 4243
	last exit code = (never exited)
}
`

const printStopped = `gui/501/io.github.seletz.passierschein.daemon = {
	state = not running
	program = /Applications/Passierschein.app/Contents/MacOS/a38
	runs = 1
	last exit code = 1
}
`

const printSignal = `gui/501/io.github.seletz.passierschein.daemon = {
	state = not running
	last exit code = (never exited)
	last terminating signal = Segmentation fault: 11
}
`

func TestParsePrint(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		running bool
		pid     int
		exit    *Exit
		program string
	}{
		{"running", printRunning, true, 4243, nil, "/Applications/Passierschein.app/Contents/MacOS/a38"},
		{"stopped with exit code 1", printStopped, false, 0, &Exit{Code: 1}, "/Applications/Passierschein.app/Contents/MacOS/a38"},
		{"crash", printSignal, false, 0, &Exit{Signal: 11}, ""},
		{"unknown form", "something else\n", false, 0, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := ParsePrint(tt.out)
			if !j.Loaded || j.Running() != tt.running || j.PID != tt.pid || j.Program != tt.program {
				t.Errorf("job = %+v", j)
			}
			switch {
			case tt.exit == nil && j.LastExit != nil:
				t.Errorf("LastExit = %+v, want none", *j.LastExit)
			case tt.exit != nil && (j.LastExit == nil || *j.LastExit != *tt.exit):
				t.Errorf("LastExit = %v, want %+v", j.LastExit, *tt.exit)
			}
		})
	}
}

func TestParseDisabled(t *testing.T) {
	out := "disabled services = {\n\t\"com.example.other\" => disabled\n\t\"" + Label + "\" => disabled\n}\n"
	if !ParseDisabled(out, Label) {
		t.Error("the label is disabled")
	}
	if ParseDisabled("disabled services = {\n\t\""+Label+"\" => enabled\n}\n", Label) {
		t.Error("the label is enabled")
	}
	if ParseDisabled("disabled services = {\n\t\""+Label+".x\" => disabled\n}\n", Label) {
		t.Error("another label counts")
	}
}

func TestCheckArgs(t *testing.T) {
	// Only the subcommands of the package pass; a test never needs
	// another one.
	for _, ok := range [][]string{{"print", "gui/501/x"}, {"bootstrap", "gui/501", "/x.plist"}, {"bootout", "gui/501/x"}, {"enable", "gui/501/x"}, {"kickstart", "-k", "gui/501/x"}, {"print-disabled", "gui/501"}} {
		if err := checkArgs(ok); err != nil {
			t.Errorf("checkArgs(%q) = %v", ok, err)
		}
	}
	for _, bad := range [][]string{nil, {"load", "-w", "/x.plist"}, {"unload", "/x.plist"}, {"disable", "gui/501/x"}, {"setenv", "A", "B"}, {"submit"}} {
		if err := checkArgs(bad); err == nil {
			t.Errorf("checkArgs(%q) passed", bad)
		}
	}
}
