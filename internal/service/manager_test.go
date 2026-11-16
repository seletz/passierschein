package service_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/service"
	"github.com/seletz/passierschein/internal/service/fakelaunchctl"
	"github.com/seletz/passierschein/internal/token"
)

// world is a temporary HOME with an a38, an approval app, a configuration,
// a fake launchctl and a probe for the preflight. It never touches the
// real ~/Library/LaunchAgents.
type world struct {
	home    string
	self    string
	app     string
	cfgPath string
	cfg     *config.Config
	lc      *fakelaunchctl.Fake
	m       service.Manager
	probe   service.Probe
	item    token.StoreInfo
	itemErr error
	answers bool
}

func newWorld(t *testing.T) *world {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	short, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })
	w := &world{home: filepath.Join(root, "home"), lc: fakelaunchctl.New()}
	if err := os.MkdirAll(w.home, 0o700); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "Applications", "Passierschein.app", "Contents", "MacOS")
	w.self = writeExe(t, filepath.Join(bundle, "a38"))
	w.app = writeExe(t, filepath.Join(bundle, "Passierschein"))
	w.cfgPath = filepath.Join(root, "config.yaml")
	cfg, err := config.Parse([]byte("version: 1\npaths:\n  socket_dir: " + short + "/run\n  audit_file: " + short + "/audit.db\n" +
		"vaults:\n  - {name: beispiel.app.k8s-dev, environment: dev, account: privat}\n" +
		"backend:\n  onepassword:\n    accounts:\n      - name: privat\n"))
	if err != nil {
		t.Fatal(err)
	}
	w.cfg = cfg
	w.item = token.StoreInfo{Name: "a38.onepassword/privat", Exists: true, ReadableWithoutDialog: true}
	w.m = service.Manager{Layout: service.Layout{Home: w.home, UID: 501}, Launchctl: w.lc, Self: w.self}
	w.probe = service.Probe{
		LoadConfig: func(string) (*config.Config, error) { return w.cfg, nil },
		AppPath:    func(*config.Config) string { return w.app },
		TokenItem:  func(string) (token.StoreInfo, error) { return w.item, w.itemErr },
		DaemonAnswers: func(context.Context, string) bool {
			return w.answers
		},
		UID: os.Getuid(),
	}
	t.Setenv("SSH_CONNECTION", "")
	return w
}

func writeExe(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func (w *world) opts() service.Options {
	return service.Options{Backend: "1password", UI: "app", ConfigPath: w.cfgPath}
}

func (w *world) plistPath() string {
	return filepath.Join(w.home, "Library", "LaunchAgents", service.Label+".plist")
}

func TestInstallWritesPlistAndLoadsJob(t *testing.T) {
	// F15, UC11 step 4: install writes the plist with the mode 0644 and
	// loads the job: enable, then bootstrap into gui/<uid>.
	w := newWorld(t)
	r, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Written || !r.Loaded || r.Unchanged {
		t.Errorf("result = %+v", r)
	}
	fi, err := os.Stat(w.plistPath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %04o, want 0644", fi.Mode().Perm())
	}
	data, _ := os.ReadFile(w.plistPath())
	if !bytes.Equal(data, r.Plist) || !service.HasMarker(data) {
		t.Error("the file differs from the plist of the result or has no marker")
	}
	p, err := service.ParsePlist(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{w.self, "serve", "--backend", "1password", "--ui", "app"}
	if !slices.Equal(p.ProgramArguments, want) {
		t.Errorf("ProgramArguments = %q, want %q (no --config without a given file)", p.ProgramArguments, want)
	}
	if got := w.lc.Changing(); !slices.Equal(got, []string{"enable", "bootstrap"}) {
		t.Errorf("launchctl calls = %q", got)
	}
	for _, c := range w.lc.Calls {
		if c[0] == "bootstrap" && (c[1] != "gui/501" || c[2] != w.plistPath()) {
			t.Errorf("bootstrap %q", c)
		}
		if c[0] == "enable" && c[1] != "gui/501/"+service.Label {
			t.Errorf("enable %q", c)
		}
	}
	li, err := os.Stat(filepath.Join(w.home, "Library", "Logs", "a38", "serve.log"))
	if err != nil || li.Mode().Perm() != 0o600 {
		t.Errorf("log file: %v, %v", li, err)
	}
	if di, err := os.Stat(filepath.Join(w.home, "Library", "Logs", "a38")); err != nil || di.Mode().Perm() != 0o700 {
		t.Errorf("log directory: %v, %v", di, err)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	// A second install with the same options changes nothing.
	w := newWorld(t)
	if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(w.plistPath())
	w.lc.Reset()
	r, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Unchanged || r.Written || r.Loaded {
		t.Errorf("result = %+v", r)
	}
	if got := w.lc.Changing(); len(got) != 0 {
		t.Errorf("launchctl changed the state: %q", got)
	}
	after, _ := os.Stat(w.plistPath())
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("the plist was written again")
	}
}

func TestInstallRunningDaemonIsOwnJob(t *testing.T) {
	// The running LaunchAgent answers on the admin socket. This is no
	// reason to refuse a second install.
	w := newWorld(t)
	if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
		t.Fatal(err)
	}
	w.answers = true
	if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
		t.Fatal(err)
	}
}

func TestInstallWithChangedOptionsReloads(t *testing.T) {
	// A new configuration path changes the plist: bootout the old job,
	// write, enable, bootstrap.
	w := newWorld(t)
	if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
		t.Fatal(err)
	}
	w.lc.Reset()
	o := w.opts()
	o.ConfigGiven = true
	o.Env = map[string]string{"XDG_STATE_HOME": "/Users/beispiel/state"}
	r, err := w.m.Install(context.Background(), o, w.probe)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Written || !r.Loaded {
		t.Errorf("result = %+v", r)
	}
	if got := w.lc.Changing(); !slices.Equal(got, []string{"bootout", "enable", "bootstrap"}) {
		t.Errorf("launchctl calls = %q", got)
	}
	data, _ := os.ReadFile(w.plistPath())
	p, _ := service.ParsePlist(data)
	if p.Arg("--config") != w.cfgPath {
		t.Errorf("--config = %q", p.Arg("--config"))
	}
}

func TestInstallLoadsInstalledButUnloadedJob(t *testing.T) {
	w := newWorld(t)
	if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
		t.Fatal(err)
	}
	w.lc.Loaded, w.lc.PID = false, 0
	w.lc.Reset()
	r, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err != nil {
		t.Fatal(err)
	}
	if r.Written || !r.Loaded {
		t.Errorf("result = %+v", r)
	}
	if got := w.lc.Changing(); !slices.Equal(got, []string{"enable", "bootstrap"}) {
		t.Errorf("launchctl calls = %q", got)
	}
}

func TestInstallRefusesForeignPlist(t *testing.T) {
	// A plist that a38 did not write stays; --force replaces it.
	foreign := []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>Label</key><string>` + service.Label + `</string></dict></plist>`)
	for _, kind := range []string{"file", "link"} {
		t.Run(kind, func(t *testing.T) {
			w := newWorld(t)
			if err := os.MkdirAll(filepath.Dir(w.plistPath()), 0o755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(w.home, "elsewhere.plist")
			if err := os.WriteFile(target, foreign, 0o644); err != nil {
				t.Fatal(err)
			}
			if kind == "file" {
				if err := os.WriteFile(w.plistPath(), foreign, 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(target, w.plistPath()); err != nil {
				t.Fatal(err)
			}
			_, err := w.m.Install(context.Background(), w.opts(), w.probe)
			if !errors.Is(err, service.ErrRefused) || !strings.Contains(err.Error(), "--force") {
				t.Fatalf("err = %v", err)
			}
			if got := w.lc.Changing(); len(got) != 0 {
				t.Errorf("launchctl calls after a refusal: %q", got)
			}
			if data, _ := os.ReadFile(target); !bytes.Equal(data, foreign) {
				t.Error("the foreign file changed")
			}
			o := w.opts()
			o.Force = true
			if _, err := w.m.Install(context.Background(), o, w.probe); err != nil {
				t.Fatal(err)
			}
			if fi, _ := os.Lstat(w.plistPath()); !fi.Mode().IsRegular() {
				t.Error("the plist is not a regular file after --force")
			}
			if data, _ := os.ReadFile(target); !bytes.Equal(data, foreign) {
				t.Error("--force wrote through the link")
			}
		})
	}
}

func TestInstallDryRunChangesNothing(t *testing.T) {
	w := newWorld(t)
	o := w.opts()
	o.DryRun = true
	r, err := w.m.Install(context.Background(), o, w.probe)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Plist) == 0 || r.Written || r.Loaded {
		t.Errorf("result = %+v", r)
	}
	if _, err := os.Stat(w.plistPath()); !os.IsNotExist(err) {
		t.Error("dry run wrote the plist")
	}
	if got := w.lc.Changing(); len(got) != 0 {
		t.Errorf("dry run called %q", got)
	}
}

func TestInstallBootstrapFails(t *testing.T) {
	// System Settings can forbid the login item; bootstrap then fails.
	w := newWorld(t)
	w.lc.BootstrapCode = 1
	_, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err == nil || !strings.Contains(err.Error(), "bootstrap") || !strings.Contains(err.Error(), "System Settings") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallRefusesBadProgram(t *testing.T) {
	// D51: a group writable a38 would run at each login.
	w := newWorld(t)
	if err := os.Chmod(w.self, 0o775); err != nil {
		t.Fatal(err)
	}
	_, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if !errors.Is(err, service.ErrRefused) || !strings.Contains(err.Error(), "writable") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(w.plistPath()); !os.IsNotExist(err) {
		t.Error("the plist was written")
	}
}

func TestInstallWarnsOutsideBundle(t *testing.T) {
	w := newWorld(t)
	w.m.Self = writeExe(t, filepath.Join(w.home, ".local", "bin", "a38"))
	t.Setenv("SSH_CONNECTION", "192.0.2.1 50000 192.0.2.2 22")
	r, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(r.Warnings, "\n")
	for _, want := range []string{"Passierschein.app", "SSH session"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings %q do not contain %q", r.Warnings, want)
		}
	}
}

func TestUninstall(t *testing.T) {
	t.Run("own plist", func(t *testing.T) {
		w := newWorld(t)
		if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
			t.Fatal(err)
		}
		w.lc.Reset()
		r, err := w.m.Uninstall(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		if !r.WasInstalled || !r.Unloaded || !r.Removed {
			t.Errorf("result = %+v", r)
		}
		if got := w.lc.Changing(); !slices.Equal(got, []string{"bootout"}) {
			t.Errorf("launchctl calls = %q", got)
		}
		if _, err := os.Stat(w.plistPath()); !os.IsNotExist(err) {
			t.Error("the plist is still there")
		}
		if _, err := os.Stat(filepath.Join(w.home, "Library", "Logs", "a38", "serve.log")); err != nil {
			t.Error("uninstall removed the log")
		}
	})
	t.Run("not installed", func(t *testing.T) {
		w := newWorld(t)
		r, err := w.m.Uninstall(context.Background(), false)
		if err != nil || r.WasInstalled {
			t.Fatalf("result = %+v, %v", r, err)
		}
		if got := w.lc.Changing(); len(got) != 0 {
			t.Errorf("launchctl calls = %q", got)
		}
	})
	t.Run("foreign plist", func(t *testing.T) {
		w := newWorld(t)
		if err := os.MkdirAll(filepath.Dir(w.plistPath()), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(w.plistPath(), []byte("<plist/>"), 0o644); err != nil {
			t.Fatal(err)
		}
		w.lc.Loaded = true
		_, err := w.m.Uninstall(context.Background(), false)
		if !errors.Is(err, service.ErrRefused) {
			t.Fatalf("err = %v", err)
		}
		if _, err := os.Stat(w.plistPath()); err != nil {
			t.Error("the foreign plist is gone")
		}
		if got := w.lc.Changing(); len(got) != 0 {
			t.Errorf("launchctl calls = %q", got)
		}
		r, err := w.m.Uninstall(context.Background(), true)
		if err != nil || !r.Removed || !r.Unloaded {
			t.Fatalf("--force: %+v, %v", r, err)
		}
	})
	t.Run("loaded without plist", func(t *testing.T) {
		w := newWorld(t)
		w.lc.Loaded = true
		r, err := w.m.Uninstall(context.Background(), false)
		if err != nil || !r.Unloaded || r.Removed {
			t.Fatalf("result = %+v, %v", r, err)
		}
	})
}

func TestRestart(t *testing.T) {
	w := newWorld(t)
	if err := w.m.Restart(context.Background()); !errors.Is(err, service.ErrRefused) {
		t.Fatalf("restart without install: %v", err)
	}
	if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
		t.Fatal(err)
	}
	pid := w.lc.PID
	w.lc.Reset()
	if err := w.m.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := w.lc.Changing(); !slices.Equal(got, []string{"kickstart"}) {
		t.Errorf("launchctl calls = %q", got)
	}
	if w.lc.Calls[len(w.lc.Calls)-1][1] != "-k" {
		t.Errorf("kickstart without -k: %q", w.lc.Calls[len(w.lc.Calls)-1])
	}
	if w.lc.PID == pid {
		t.Error("the daemon has the same pid")
	}
	w.lc.Loaded, w.lc.PID = false, 0
	w.lc.Reset()
	if err := w.m.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := w.lc.Changing(); !slices.Equal(got, []string{"enable", "bootstrap"}) {
		t.Errorf("launchctl calls = %q", got)
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, w *world)
		check   func(t *testing.T, s service.Status)
	}{
		{"not installed", func(*testing.T, *world) {}, func(t *testing.T, s service.Status) {
			if s.Installed || s.Loaded || s.Running || s.OK() || s.LogSize != -1 {
				t.Errorf("status = %+v", s)
			}
		}},
		{"running", func(t *testing.T, w *world) { w.install(t) }, func(t *testing.T, s service.Status) {
			if !s.OK() || s.PID == 0 || s.State != "running" || s.Disabled || s.LogSize != 0 {
				t.Errorf("status = %+v", s)
			}
		}},
		{"start failed", func(t *testing.T, w *world) {
			w.lc.StartFails = true
			w.install(t)
		}, func(t *testing.T, s service.Status) {
			if s.OK() || s.Running || !s.Loaded || s.LastExit == nil || s.LastExit.Code != 1 || !strings.Contains(s.LastExitMeaning, "configuration") {
				t.Errorf("status = %+v", s)
			}
		}},
		{"program moved", func(t *testing.T, w *world) {
			w.install(t)
			if err := os.Remove(w.self); err != nil {
				t.Fatal(err)
			}
		}, func(t *testing.T, s service.Status) {
			if s.OK() || !s.ProgramMissing {
				t.Errorf("status = %+v", s)
			}
		}},
		{"other a38 runs the command", func(t *testing.T, w *world) {
			w.install(t)
			w.m.Self = writeExe(t, filepath.Join(w.home, "other", "a38"))
		}, func(t *testing.T, s service.Status) {
			if s.OK() || !s.ProgramDiffers || s.ProgramMissing {
				t.Errorf("status = %+v", s)
			}
		}},
		{"disabled", func(t *testing.T, w *world) {
			w.install(t)
			w.lc.Disabled = true
		}, func(t *testing.T, s service.Status) {
			if !s.Disabled {
				t.Errorf("status = %+v", s)
			}
		}},
		{"foreign", func(t *testing.T, w *world) {
			if err := os.MkdirAll(filepath.Dir(w.plistPath()), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(w.plistPath(), []byte(`<plist version="1.0"><dict><key>ProgramArguments</key><array><string>/bin/sh</string></array></dict></plist>`), 0o644); err != nil {
				t.Fatal(err)
			}
		}, func(t *testing.T, s service.Status) {
			if !s.Installed || !s.Foreign || s.OK() || s.Program != "/bin/sh" {
				t.Errorf("status = %+v", s)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			tt.prepare(t, w)
			w.lc.Reset()
			s := w.m.Status(context.Background())
			tt.check(t, s)
			if got := w.lc.Changing(); len(got) != 0 {
				t.Errorf("status changed the state: %q", got)
			}
		})
	}
}

func (w *world) install(t *testing.T) {
	t.Helper()
	if _, err := w.m.Install(context.Background(), w.opts(), w.probe); err != nil {
		t.Fatal(err)
	}
}
