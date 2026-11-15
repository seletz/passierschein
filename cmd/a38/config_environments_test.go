package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
)

func writeEnvConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(configHome(t), "c.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigValidateDeprecatedLifetime(t *testing.T) {
	// T37: row 7. grant.lifetime alone loads; validate names the key as
	// deprecated with the new key and exits with 0.
	p := writeEnvConfig(t, "version: 1\ngrant:\n  lifetime: 2h\n")
	code, out, errOut := a38(t, "config", "validate", "--config", p)
	if code != 0 {
		t.Fatalf("code %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	for _, want := range []string{"grant.lifetime", "deprecated", "environments.dev.grant_lifetime"} {
		if !strings.Contains(out, want) {
			t.Errorf("validate lacks %q:\n%s", want, out)
		}
	}
}

func TestConfigValidateBothLifetimes(t *testing.T) {
	// T37: row 6. Both keys together stop loading.
	p := writeEnvConfig(t, "version: 1\ngrant:\n  lifetime: 2h\nenvironments:\n  dev:\n    grant_lifetime: 1h\n")
	code, out, _ := a38(t, "config", "validate", "--config", p)
	if code != 1 || !strings.Contains(out, "grant.lifetime") {
		t.Errorf("code %d\n%s", code, out)
	}
}

func TestConfigShowEnvironments(t *testing.T) {
	// F22 (part), T37: show lists the new keys with their origin.
	p := writeEnvConfig(t, "version: 1\nenvironments:\n  prod:\n    delivery: [exec]\nexec:\n  max_timeout: 5m\n")
	out := mustA38(t, "config", "show", "--config", p)
	for _, want := range []string{
		"environments.dev.delivery", "environments.test.grant_lifetime", "environments.prod.binding",
		"exec.default_timeout", "exec.max_timeout", "exec.output_limit", "64KiB", "mcp, exec",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
	var prodDelivery, maxTimeout string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "environments.prod.delivery"):
			prodDelivery = line
		case strings.Contains(line, "exec.max_timeout"):
			maxTimeout = line
		}
	}
	if !strings.Contains(prodDelivery, "[file]") || !strings.Contains(maxTimeout, "[file]") {
		t.Errorf("origin of a set key is not file:\n%s\n%s", prodDelivery, maxTimeout)
	}
	if strings.Contains(out, "grant.lifetime ") {
		t.Errorf("show lists the deprecated key although the file does not set it:\n%s", out)
	}
	// The JSON form has the same keys.
	js := mustA38(t, "config", "show", "--config", p, "--json")
	if !strings.Contains(js, `"environments.prod.delivery"`) {
		t.Errorf("JSON lacks the key:\n%s", js)
	}
}

func TestConfigShowDeprecatedLifetime(t *testing.T) {
	p := writeEnvConfig(t, "version: 1\ngrant:\n  lifetime: 2h\n")
	out := mustA38(t, "config", "show", "--config", p)
	for _, want := range []string{"grant.lifetime", "deprecated", "environments.dev.grant_lifetime"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
}

func TestConfigInitTemplateHasNewKeys(t *testing.T) {
	// F22 (part): the new file names the new keys, not the deprecated one,
	// and loads with the defaults of D66.
	data, err := config.Template()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"environments:", "grant_lifetime: 4h", "grant_lifetime: 30m", "grant_lifetime: 0s",
		"binding: program", "binding: command", "exec:", "default_timeout: 2m", "max_timeout: 10m", "output_limit: 64KiB", "delivery: []"} {
		if !strings.Contains(text, want) {
			t.Errorf("template lacks %q", want)
		}
	}
	if strings.Contains(text, "\ngrant:") {
		t.Error("template still has the deprecated section grant")
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("template does not load: %v", err)
	}
	if len(cfg.Deprecations) != 0 {
		t.Errorf("template uses a deprecated key: %v", cfg.Deprecations)
	}
	if cfg.GrantLifetime(config.EnvDev) != config.DefaultDevGrantLifetime || cfg.DeliveryAllowed(config.EnvProd, config.DeliveryExec) {
		t.Errorf("template does not carry the defaults: %+v", cfg.Environments)
	}
}
