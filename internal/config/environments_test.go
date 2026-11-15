package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentDefaults(t *testing.T) {
	// T37: row 1, no "environments" and no "exec": the defaults of D66.
	setHome(t)
	cfg, err := Parse([]byte(minimal))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := map[Environment]EnvironmentPolicy{
		EnvDev:  {Delivery: []Delivery{DeliveryMCP, DeliveryExec}, GrantLifetime: 4 * time.Hour, Binding: BindingNone},
		EnvTest: {Delivery: []Delivery{DeliveryExec}, GrantLifetime: 30 * time.Minute, Binding: BindingProgram},
		EnvProd: {Delivery: []Delivery{}, GrantLifetime: 0, Binding: BindingCommand},
	}
	for env, w := range want {
		if got := cfg.Policy(env); !reflect.DeepEqual(got, w) {
			t.Errorf("Policy(%s) = %+v, want %+v", env, got, w)
		}
	}
	wantExec := Exec{DefaultTimeout: 2 * time.Minute, MaxTimeout: 10 * time.Minute, OutputLimit: 64 << 10}
	if cfg.Exec != wantExec {
		t.Errorf("Exec = %+v, want %+v", cfg.Exec, wantExec)
	}
}

func TestEnvironmentAccessors(t *testing.T) {
	// T37 (part): the typed accessors for the broker.
	setHome(t)
	cfg, err := Parse([]byte(minimal + "environments:\n  prod:\n    delivery: [exec]\n  test:\n    grant_lifetime: 10m\n    binding: command\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tests := []struct {
		env      Environment
		mcp      bool
		exec     bool
		lifetime time.Duration
		binding  Binding
	}{
		{EnvDev, true, true, 4 * time.Hour, BindingNone},
		{EnvTest, false, true, 10 * time.Minute, BindingCommand},
		{EnvProd, false, true, 0, BindingCommand},
		{Environment("staging"), false, false, 0, BindingCommand},
	}
	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			if got := cfg.DeliveryAllowed(tt.env, DeliveryMCP); got != tt.mcp {
				t.Errorf("DeliveryAllowed(mcp) = %v, want %v", got, tt.mcp)
			}
			if got := cfg.DeliveryAllowed(tt.env, DeliveryExec); got != tt.exec {
				t.Errorf("DeliveryAllowed(exec) = %v, want %v", got, tt.exec)
			}
			if got := cfg.GrantLifetime(tt.env); got != tt.lifetime {
				t.Errorf("GrantLifetime = %v, want %v", got, tt.lifetime)
			}
			if got := cfg.Binding(tt.env); got != tt.binding {
				t.Errorf("Binding = %q, want %q", got, tt.binding)
			}
		})
	}
}

func TestEnvironmentRejects(t *testing.T) {
	// T37: rows 2 to 6, 8 and 9 stop loading; the message names the key.
	setHome(t)
	tests := []struct {
		name     string
		yaml     string
		wantText string
	}{
		{"mcp for test", "environments:\n  test:\n    delivery: [mcp]\n", "environments.test.delivery"},
		{"mcp for prod", "environments:\n  prod:\n    delivery: [mcp, exec]\n", "environments.prod.delivery"},
		{"unknown way", "environments:\n  dev:\n    delivery: [file]\n", "environments.dev.delivery"},
		{"way twice", "environments:\n  dev:\n    delivery: [exec, exec]\n", "environments.dev.delivery"},
		{"delivery not a list", "environments:\n  dev:\n    delivery: exec\n", "environments.dev.delivery"},
		{"prod lifetime 1h", "environments:\n  prod:\n    grant_lifetime: 1h\n", "environments.prod.grant_lifetime"},
		{"dev lifetime 25h", "environments:\n  dev:\n    grant_lifetime: 25h\n", "environments.dev.grant_lifetime"},
		{"dev lifetime negative", "environments:\n  dev:\n    grant_lifetime: -1s\n", "environments.dev.grant_lifetime"},
		{"dev lifetime no unit", "environments:\n  dev:\n    grant_lifetime: 60\n", "environments.dev.grant_lifetime"},
		{"prod binding program", "environments:\n  prod:\n    binding: program\n", "environments.prod.binding"},
		{"test binding none", "environments:\n  test:\n    binding: none\n", "environments.test.binding"},
		{"unknown binding", "environments:\n  dev:\n    binding: all\n", "environments.dev.binding"},
		{"both lifetimes", "grant:\n  lifetime: 2h\nenvironments:\n  dev:\n    grant_lifetime: 1h\n", "grant.lifetime"},
		{"default above max", "exec:\n  default_timeout: 20m\n  max_timeout: 10m\n", "exec.default_timeout"},
		{"max above 1h", "exec:\n  max_timeout: 2h\n", "exec.max_timeout"},
		{"max zero", "exec:\n  max_timeout: 0s\n", "exec.max_timeout"},
		{"default zero", "exec:\n  default_timeout: 0s\n", "exec.default_timeout"},
		{"limit 0KiB", "exec:\n  output_limit: 0KiB\n", "exec.output_limit"},
		{"limit 2MiB", "exec:\n  output_limit: 2MiB\n", "exec.output_limit"},
		{"limit without unit", "exec:\n  output_limit: 65536\n", "exec.output_limit"},
		{"limit with KB", "exec:\n  output_limit: 64KB\n", "exec.output_limit"},
		{"environment staging", "environments:\n  staging:\n    delivery: [exec]\n", "staging"},
		{"unknown key in environment", "environments:\n  dev:\n    lifetime: 1h\n", "lifetime"},
		{"unknown key in exec", "exec:\n  timeout: 1m\n", "timeout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(minimal + tt.yaml))
			if err == nil {
				t.Fatal("Parse succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("error %q does not name %q", err, tt.wantText)
			}
		})
	}
}

func TestEnvironmentAccepts(t *testing.T) {
	setHome(t)
	tests := []struct {
		name string
		yaml string
		test func(t *testing.T, c *Config)
	}{
		{"prod on", "environments:\n  prod:\n    delivery: [exec]\n", func(t *testing.T, c *Config) {
			if !c.DeliveryAllowed(EnvProd, DeliveryExec) {
				t.Error("exec for prod is off")
			}
		}},
		{"dev narrower", "environments:\n  dev:\n    delivery: [exec]\n    grant_lifetime: 0s\n    binding: command\n", func(t *testing.T, c *Config) {
			if c.DeliveryAllowed(EnvDev, DeliveryMCP) || c.GrantLifetime(EnvDev) != 0 || c.Binding(EnvDev) != BindingCommand {
				t.Errorf("dev = %+v", c.Policy(EnvDev))
			}
		}},
		{"test 24h", "environments:\n  test:\n    grant_lifetime: 24h\n", func(t *testing.T, c *Config) {
			if c.GrantLifetime(EnvTest) != 24*time.Hour {
				t.Errorf("lifetime = %v", c.GrantLifetime(EnvTest))
			}
		}},
		{"prod explicit fixed values", "environments:\n  prod:\n    grant_lifetime: 0s\n    binding: command\n    delivery: []\n", func(t *testing.T, c *Config) {
			if c.DeliveryAllowed(EnvProd, DeliveryExec) {
				t.Error("exec for prod is on")
			}
		}},
		{"exec limits", "exec:\n  default_timeout: 30s\n  max_timeout: 1h\n  output_limit: 1MiB\n", func(t *testing.T, c *Config) {
			want := Exec{DefaultTimeout: 30 * time.Second, MaxTimeout: time.Hour, OutputLimit: 1 << 20}
			if c.Exec != want {
				t.Errorf("Exec = %+v", c.Exec)
			}
		}},
		{"smallest limit", "exec:\n  output_limit: 1KiB\n", func(t *testing.T, c *Config) {
			if c.Exec.OutputLimit != 1024 {
				t.Errorf("OutputLimit = %d", c.Exec.OutputLimit)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse([]byte(minimal + tt.yaml))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			tt.test(t, cfg)
		})
	}
}

func TestGrantLifetimeTransition(t *testing.T) {
	// T37: row 7, "grant.lifetime" alone still loads, sets dev, and is
	// reported as deprecated with the new key.
	setHome(t)
	cfg, err := Parse([]byte(minimal + "grant:\n  lifetime: 2h\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := cfg.GrantLifetime(EnvDev); got != 2*time.Hour {
		t.Errorf("GrantLifetime(dev) = %v, want 2h", got)
	}
	if cfg.Grant.Lifetime != 2*time.Hour {
		t.Errorf("Grant.Lifetime = %v, want 2h", cfg.Grant.Lifetime)
	}
	want := []Deprecation{{Key: "grant.lifetime", Replacement: "environments.dev.grant_lifetime"}}
	if !reflect.DeepEqual(cfg.Deprecations, want) {
		t.Errorf("Deprecations = %+v, want %+v", cfg.Deprecations, want)
	}

	// The new key alone is not deprecated and also sets Grant.Lifetime for
	// the code that still reads it.
	cfg, err = Parse([]byte(minimal + "environments:\n  dev:\n    grant_lifetime: 1h\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Deprecations) != 0 || cfg.Grant.Lifetime != time.Hour {
		t.Errorf("Deprecations = %+v, Grant.Lifetime = %v", cfg.Deprecations, cfg.Grant.Lifetime)
	}
}
