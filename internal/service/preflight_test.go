package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/service"
	"github.com/seletz/passierschein/internal/token"
)

func TestPreflightRefusals(t *testing.T) {
	// F15: install refuses a service that cannot start. Each refusal
	// names the finding and the next step; nothing is written, launchctl
	// changes nothing.
	tests := []struct {
		name    string
		prepare func(t *testing.T, w *world, o *service.Options)
		want    string
		next    string
	}{
		{"configuration does not load (T22 part)", func(t *testing.T, w *world, _ *service.Options) {
			// T22 (part): a vault without an environment stops the start.
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("version: 1\nvaults:\n  - name: beispiel.app.k8s-dev\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			w.cfgPath = path
			w.probe.LoadConfig = config.Load
		}, "beispiel.app.k8s-dev", "configuration file"},
		{"configuration missing", func(t *testing.T, w *world, _ *service.Options) {
			w.cfgPath = filepath.Join(t.TempDir(), "none.yaml")
			w.probe.LoadConfig = config.Load
		}, "does not load", "a38 doctor"},
		{"socket path too long", func(t *testing.T, w *world, _ *service.Options) {
			w.cfg.Paths.SocketDir = "/tmp/" + strings.Repeat("x", 100)
		}, "longer than 103 bytes", "paths.socket_dir"},
		{"approval app missing", func(t *testing.T, w *world, _ *service.Options) {
			if err := os.Remove(w.app); err != nil {
				t.Fatal(err)
			}
		}, "approval app", "/Applications"},
		{"approval app group writable", func(t *testing.T, w *world, _ *service.Options) {
			if err := os.Chmod(w.app, 0o775); err != nil {
				t.Fatal(err)
			}
		}, "writable by group", "--ui-app"},
		{"no keychain item", func(_ *testing.T, w *world, _ *service.Options) {
			w.item = token.StoreInfo{Exists: false}
		}, "no token", "a38 config onepassword-token set"},
		{"keychain error", func(_ *testing.T, w *world, _ *service.Options) {
			w.itemErr = errors.New("the keychain is locked")
		}, "cannot be read", "unlock"},
		{"no keychain in this build", func(_ *testing.T, w *world, _ *service.Options) {
			w.probe.TokenItem = nil
		}, "no keychain", "macOS build"},
		{"token file with mode 0644", func(t *testing.T, w *world, _ *service.Options) {
			path := filepath.Join(t.TempDir(), "onepassword-token")
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			w.cfg.Backend.OnePassword = config.OnePassword{Accounts: []config.Account{{Name: "privat", TokenSource: config.TokenSourceFile, TokenFile: path}}}
		}, "token file", "0600"},
		{"token file missing", func(t *testing.T, w *world, _ *service.Options) {
			w.cfg.Backend.OnePassword = config.OnePassword{Accounts: []config.Account{{Name: "privat", TokenSource: config.TokenSourceFile, TokenFile: filepath.Join(t.TempDir(), "none")}}}
		}, "token file", "0600"},
		{"no vault with an account", func(_ *testing.T, w *world, _ *service.Options) {
			w.cfg.Vaults = nil
		}, "no vault with a 1Password account", "a38 config account add"},
		{"fake file missing", func(t *testing.T, _ *world, o *service.Options) {
			o.Backend, o.UI, o.FakeFile = "fake", "auto-approve", filepath.Join(t.TempDir(), "none.yaml")
		}, "fake backend", "--fake-file"},
		{"other daemon answers", func(_ *testing.T, w *world, _ *service.Options) {
			w.answers = true
		}, "a daemon answers", "stop the daemon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			o := w.opts()
			tt.prepare(t, w, &o)
			o.ConfigPath = w.cfgPath
			r, err := w.m.Install(context.Background(), o, w.probe)
			if !errors.Is(err, service.ErrRefused) {
				t.Fatalf("err = %v, want a refusal", err)
			}
			if len(r.Preflight.Findings) == 0 {
				t.Fatal("no finding")
			}
			f := r.Preflight.Findings[0]
			if !strings.Contains(f.What, tt.want) || !strings.Contains(f.Next, tt.next) {
				t.Errorf("finding = %+v, want %q and next step %q", f, tt.want, tt.next)
			}
			if _, err := os.Stat(w.plistPath()); !os.IsNotExist(err) {
				t.Error("the plist was written")
			}
			if got := w.lc.Changing(); len(got) != 0 {
				t.Errorf("launchctl calls after a refusal: %q", got)
			}
		})
	}
}

func TestPreflightPassesWithoutReadingToken(t *testing.T) {
	// D51: the preflight reads only the metadata of the keychain item and
	// only the mode of the token file.
	w := newWorld(t)
	path := filepath.Join(t.TempDir(), "onepassword-token")
	if err := os.WriteFile(path, []byte("beispiel-wert-marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	w.cfg.Backend.OnePassword = config.OnePassword{Accounts: []config.Account{{Name: "privat", TokenSource: config.TokenSourceFile, TokenFile: path}}}
	r, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(r.Plist)+strings.Join(r.Warnings, ""), "beispiel-wert-marker") {
		t.Error("the token appears in the output")
	}
}

func TestPreflightWarnsForItemOfOtherBuild(t *testing.T) {
	w := newWorld(t)
	w.item.ReadableWithoutDialog = false
	r, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(r.Warnings, "\n"), "--replace") {
		t.Errorf("warnings = %q", r.Warnings)
	}
}

func TestOptionsValidate(t *testing.T) {
	// D40: a real backend only with --ui app.
	base := service.Options{Backend: "1password", UI: "app", ConfigPath: "/x/config.yaml"}
	tests := []struct {
		name   string
		change func(o *service.Options)
		want   string
	}{
		{"default", func(*service.Options) {}, ""},
		{"fake", func(o *service.Options) { o.Backend, o.UI, o.FakeFile = "fake", "auto-approve", "/x/fake.yaml" }, ""},
		{"1password with auto-approve", func(o *service.Options) { o.UI = "auto-approve" }, "needs --ui app"},
		{"script UI", func(o *service.Options) { o.Backend, o.UI, o.FakeFile = "fake", "script", "/x/f" }, "--ui must be"},
		{"unknown backend", func(o *service.Options) { o.Backend = "vault" }, "--backend must be"},
		{"fake without file", func(o *service.Options) { o.Backend, o.UI = "fake", "auto-deny" }, "needs --fake-file"},
		{"fake file with 1password", func(o *service.Options) { o.FakeFile = "/x/f" }, "needs --backend fake"},
		{"ui-app without app", func(o *service.Options) { o.Backend, o.UI, o.FakeFile, o.UIApp = "fake", "auto-deny", "/x/f", "/x/app" }, "--ui-app needs"},
		{"relative config", func(o *service.Options) { o.ConfigPath = "config.yaml" }, "absolute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := base
			tt.change(&o)
			err := o.Validate()
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Errorf("Validate = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestServeArgs(t *testing.T) {
	o := service.Options{Backend: "fake", UI: "app", FakeFile: "/x/fake.yaml", UIApp: "/x/P", ConfigPath: "/x/c.yaml"}
	if got := strings.Join(o.ServeArgs(), " "); got != "serve --backend fake --ui app --fake-file /x/fake.yaml --ui-app /x/P" {
		t.Errorf("args = %s", got)
	}
	o.ConfigGiven = true
	if got := strings.Join(o.ServeArgs(), " "); !strings.HasSuffix(got, "--config /x/c.yaml") {
		t.Errorf("args = %s", got)
	}
}

func TestPreflightOneAccountWithoutToken(t *testing.T) {
	// T46 (part): an account without token does not stop the install; the
	// daemon starts and locks only the vaults of this account (D73).
	w := newWorld(t)
	w.cfg.Vaults = append(w.cfg.Vaults, config.Vault{Name: "datenbank-dev", Environment: config.EnvDev, Account: "firma"})
	w.cfg.Backend.OnePassword.Accounts = append(w.cfg.Backend.OnePassword.Accounts, config.Account{Name: "firma", TokenSource: config.TokenSourceKeychain})
	w.probe.TokenItem = func(account string) (token.StoreInfo, error) {
		return token.StoreInfo{Name: "a38.onepassword/" + account, Exists: account == "privat", ReadableWithoutDialog: true}, nil
	}
	r, err := w.m.Install(context.Background(), w.opts(), w.probe)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	got := strings.Join(r.Warnings, "\n")
	for _, want := range []string{`account "firma"`, "locks the vaults of this account", "a38 config onepassword-token set --account firma"} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings %q lack %q", got, want)
		}
	}
	if strings.Contains(got, `account "privat"`) {
		t.Errorf("warnings name the account privat: %q", got)
	}
}
