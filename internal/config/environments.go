package config

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Delivery is a way to deliver values to an agent (D62).
type Delivery string

// The delivery ways.
const (
	// DeliveryMCP is the answer of "request_credentials". Only dev can
	// allow it.
	DeliveryMCP Delivery = "mcp"
	// DeliveryExec is the exec way: "a38 mcp" starts a command with the
	// values in its environment.
	DeliveryExec Delivery = "exec"
)

// Valid reports whether d is mcp or exec. The comparison is exact.
func (d Delivery) Valid() bool { return d == DeliveryMCP || d == DeliveryExec }

// Binding is the scope of a grant to commands (D63).
type Binding string

// The bindings.
const (
	// BindingNone: a grant covers the references, with each way and each
	// command.
	BindingNone Binding = "none"
	// BindingProgram: a grant covers the references with the same program
	// and the same variable names.
	BindingProgram Binding = "program"
	// BindingCommand: a grant covers the references with the same program,
	// arguments, working directory and variable names.
	BindingCommand Binding = "command"
)

// Valid reports whether b is none, program or command. The comparison is
// exact.
func (b Binding) Valid() bool {
	return b == BindingNone || b == BindingProgram || b == BindingCommand
}

// Limits of the values of D66.
const (
	// DefaultDevGrantLifetime is the grant lifetime of dev. It is the
	// default of the deprecated key grant.lifetime too.
	DefaultDevGrantLifetime = DefaultGrantLifetime
	// DefaultTestGrantLifetime is the grant lifetime of test.
	DefaultTestGrantLifetime = 30 * time.Minute
	// MaxGrantLifetime is the largest valid grant lifetime.
	MaxGrantLifetime = 24 * time.Hour
	// DefaultExecTimeout is the default of exec.default_timeout.
	DefaultExecTimeout = 2 * time.Minute
	// DefaultExecMaxTimeout is the default of exec.max_timeout.
	DefaultExecMaxTimeout = 10 * time.Minute
	// MaxExecTimeout is the largest valid exec.max_timeout.
	MaxExecTimeout = time.Hour
	// DefaultExecOutputLimit is the default of exec.output_limit in bytes.
	DefaultExecOutputLimit = 64 << 10
	// MinExecOutputLimit is the smallest valid exec.output_limit in bytes.
	MinExecOutputLimit = 1 << 10
	// MaxExecOutputLimit is the largest valid exec.output_limit in bytes.
	MaxExecOutputLimit = 1 << 20
)

// EnvironmentPolicy holds the settings of one environment (D66).
type EnvironmentPolicy struct {
	// Delivery lists the ways that the environment allows. The list is
	// empty if the way is off. Do not change it.
	Delivery []Delivery
	// GrantLifetime is the lifetime of a grant. Zero means a grant for
	// exactly one delivery (D63).
	GrantLifetime time.Duration
	// Binding is the binding of a grant.
	Binding Binding
}

// Exec holds the limits of the exec way (D64, D66).
type Exec struct {
	// DefaultTimeout is the time limit of a command if the request names
	// none.
	DefaultTimeout time.Duration
	// MaxTimeout is the largest time limit that a request can name.
	MaxTimeout time.Duration
	// OutputLimit is the number of bytes of each output stream that the
	// result keeps.
	OutputLimit int
}

// Deprecation names a key that is still valid but replaced.
type Deprecation struct {
	// Key is the deprecated key, for example "grant.lifetime".
	Key string
	// Replacement is the key that replaces it.
	Replacement string
}

// String returns the one-sentence form that a38 config validate and a38
// doctor print.
func (d Deprecation) String() string {
	return fmt.Sprintf("the key %s is deprecated; use %s", d.Key, d.Replacement)
}

// environmentPolicies is the fixed order of the environments.
var environmentOrder = []Environment{EnvDev, EnvTest, EnvProd}

// Policy returns the settings of an environment. For a value that is not
// dev, test or prod it returns the strictest settings: no delivery, no
// lifetime and the binding command. The returned Delivery slice is a copy.
func (c *Config) Policy(env Environment) EnvironmentPolicy {
	p, ok := c.Environments[env]
	if !ok {
		return EnvironmentPolicy{Delivery: []Delivery{}, Binding: BindingCommand}
	}
	p.Delivery = slices.Clone(p.Delivery)
	if p.Delivery == nil {
		p.Delivery = []Delivery{}
	}
	return p
}

// DeliveryAllowed reports whether the environment allows the delivery way.
// A way that the configuration does not name, and an unknown environment,
// give false. The rule that mcp is only for dev holds in Parse; this
// method does not repeat it.
func (c *Config) DeliveryAllowed(env Environment, way Delivery) bool {
	return slices.Contains(c.Environments[env].Delivery, way)
}

// GrantLifetime returns the grant lifetime of the environment. Zero means a
// grant for exactly one delivery. An unknown environment gives zero.
func (c *Config) GrantLifetime(env Environment) time.Duration {
	return c.Environments[env].GrantLifetime
}

// Binding returns the binding of a grant in the environment. An unknown
// environment gives the strictest binding, command.
func (c *Config) Binding(env Environment) Binding {
	if p, ok := c.Environments[env]; ok {
		return p.Binding
	}
	return BindingCommand
}

type rawEnvironment struct {
	Delivery      yaml.Node `yaml:"delivery"`
	GrantLifetime yaml.Node `yaml:"grant_lifetime"`
	Binding       yaml.Node `yaml:"binding"`
}

type rawEnvironments struct {
	Dev  rawEnvironment `yaml:"dev"`
	Test rawEnvironment `yaml:"test"`
	Prod rawEnvironment `yaml:"prod"`
}

type rawExec struct {
	DefaultTimeout yaml.Node `yaml:"default_timeout"`
	MaxTimeout     yaml.Node `yaml:"max_timeout"`
	OutputLimit    yaml.Node `yaml:"output_limit"`
}

// parseEnvironments checks the sections "environments" and "grant". It sets
// cfg.Environments, cfg.Grant and cfg.Deprecations.
func parseEnvironments(raw rawEnvironments, legacy yaml.Node, cfg *Config, fail func(string, ...any)) {
	devLifetime := DefaultDevGrantLifetime
	_, legacyPresent, _ := scalar(legacy)
	if legacyPresent {
		d, err := durationValue(legacy, DefaultGrantLifetime)
		if err != nil {
			fail("grant.lifetime: %v", err)
		} else {
			devLifetime = d
		}
		if raw.Dev.GrantLifetime.Kind != 0 {
			fail("grant.lifetime: the key cannot stand together with environments.dev.grant_lifetime; remove grant.lifetime")
		} else if err == nil {
			cfg.Deprecations = append(cfg.Deprecations, Deprecation{Key: "grant.lifetime", Replacement: "environments.dev.grant_lifetime"})
		}
	}

	cfg.Environments = map[Environment]EnvironmentPolicy{}
	for _, env := range environmentOrder {
		var r rawEnvironment
		def := EnvironmentPolicy{}
		switch env {
		case EnvDev:
			r = raw.Dev
			def = EnvironmentPolicy{Delivery: []Delivery{DeliveryMCP, DeliveryExec}, GrantLifetime: devLifetime, Binding: BindingNone}
		case EnvTest:
			r = raw.Test
			def = EnvironmentPolicy{Delivery: []Delivery{DeliveryExec}, GrantLifetime: DefaultTestGrantLifetime, Binding: BindingProgram}
		case EnvProd:
			r = raw.Prod
			def = EnvironmentPolicy{Delivery: []Delivery{}, GrantLifetime: 0, Binding: BindingCommand}
		}
		key := func(k string) string { return "environments." + string(env) + "." + k }

		p := def
		var err error
		if p.Delivery, err = deliveryValue(r.Delivery, def.Delivery, env); err != nil {
			fail("%s: %v", key("delivery"), err)
		}
		if p.GrantLifetime, err = lifetimeValue(r.GrantLifetime, def.GrantLifetime, env); err != nil {
			fail("%s: %v", key("grant_lifetime"), err)
		}
		if p.Binding, err = bindingValue(r.Binding, def.Binding, env); err != nil {
			fail("%s: %v", key("binding"), err)
		}
		cfg.Environments[env] = p
	}
	// Code that still reads Grant.Lifetime gets the lifetime of dev.
	cfg.Grant.Lifetime = cfg.Environments[EnvDev].GrantLifetime
}

func deliveryValue(n yaml.Node, def []Delivery, env Environment) ([]Delivery, error) {
	if n.Kind == 0 {
		return slices.Clone(def), nil
	}
	if n.Kind != yaml.SequenceNode {
		return def, errors.New(`the value must be a list, for example [exec] or []`)
	}
	out := []Delivery{}
	for _, item := range n.Content {
		if item.Kind != yaml.ScalarNode {
			return def, errors.New("each entry must be mcp or exec")
		}
		d := Delivery(item.Value)
		switch {
		case !d.Valid():
			return def, fmt.Errorf("%q is not a delivery way; use mcp or exec", item.Value)
		case d == DeliveryMCP && env != EnvDev:
			return def, errors.New("the way mcp is only for dev; a value of test or prod never goes into an MCP answer")
		case slices.Contains(out, d):
			return def, fmt.Errorf("the way %s is in the list more than once", d)
		}
		out = append(out, d)
	}
	return out, nil
}

func lifetimeValue(n yaml.Node, def time.Duration, env Environment) (time.Duration, error) {
	text, present, err := scalar(n)
	if err != nil || !present {
		return def, err
	}
	d, err := time.ParseDuration(text)
	if err != nil {
		return def, errors.New(`the value must be a duration with a unit, for example "4h", "30m" or "0s"`)
	}
	switch {
	case d < 0 || d > MaxGrantLifetime:
		return def, errors.New("the duration must be between 0s and 24h")
	case env == EnvProd && d != 0:
		return def, errors.New("the value for prod must be 0s; each command in prod needs its own approval")
	}
	return d, nil
}

func bindingValue(n yaml.Node, def Binding, env Environment) (Binding, error) {
	text, present, err := scalar(n)
	if err != nil || !present {
		return def, err
	}
	b := Binding(text)
	switch {
	case !b.Valid():
		return def, errors.New("the value must be none, program or command")
	case env == EnvTest && b == BindingNone:
		return def, errors.New("the value for test must be program or command, not none")
	case env == EnvProd && b != BindingCommand:
		return def, errors.New("the value for prod must be command")
	}
	return b, nil
}

// parseExec checks the section "exec".
func parseExec(raw rawExec, cfg *Config, fail func(string, ...any)) {
	cfg.Exec = Exec{DefaultTimeout: DefaultExecTimeout, MaxTimeout: DefaultExecMaxTimeout, OutputLimit: DefaultExecOutputLimit}
	maxOK := true
	if d, err := durationValue(raw.MaxTimeout, DefaultExecMaxTimeout); err != nil {
		fail("exec.max_timeout: %v", err)
		maxOK = false
	} else if d > MaxExecTimeout {
		fail("exec.max_timeout: the duration must be 1h or less")
		maxOK = false
	} else {
		cfg.Exec.MaxTimeout = d
	}
	if d, err := durationValue(raw.DefaultTimeout, DefaultExecTimeout); err != nil {
		fail("exec.default_timeout: %v", err)
	} else if maxOK && d > cfg.Exec.MaxTimeout {
		fail("exec.default_timeout: the duration must not be more than exec.max_timeout (%s)", cfg.Exec.MaxTimeout)
	} else {
		cfg.Exec.DefaultTimeout = d
	}
	if n, err := sizeValue(raw.OutputLimit, DefaultExecOutputLimit); err != nil {
		fail("exec.output_limit: %v", err)
	} else {
		cfg.Exec.OutputLimit = n
	}
}

var sizePattern = regexp.MustCompile(`^([0-9]+)(KiB|MiB)$`)

// sizeValue reads a size such as "64KiB" and returns bytes. The range is
// 1KiB to 1MiB.
func sizeValue(n yaml.Node, def int) (int, error) {
	text, present, err := scalar(n)
	if err != nil || !present {
		return def, err
	}
	m := sizePattern.FindStringSubmatch(text)
	if m == nil {
		return def, errors.New(`the value must be a size with the unit KiB or MiB, for example "64KiB"`)
	}
	v, err := strconv.Atoi(m[1])
	if err != nil || v > MaxExecOutputLimit {
		return def, errors.New("the size must be between 1KiB and 1MiB")
	}
	if m[2] == "MiB" {
		v <<= 20
	} else {
		v <<= 10
	}
	if v < MinExecOutputLimit || v > MaxExecOutputLimit {
		return def, errors.New("the size must be between 1KiB and 1MiB")
	}
	return v, nil
}

// FormatSize writes a size in bytes in the form of exec.output_limit, for
// example "64KiB". A size that is not a whole number of KiB is written in
// bytes with the unit "B".
func FormatSize(n int) string {
	switch {
	case n%(1<<20) == 0:
		return strconv.Itoa(n>>20) + "MiB"
	case n%(1<<10) == 0:
		return strconv.Itoa(n>>10) + "KiB"
	}
	return strconv.Itoa(n) + "B"
}

// FormatDelivery writes the delivery ways as a list for a person, for
// example "mcp, exec". An empty list gives "none (off)".
func FormatDelivery(ways []Delivery) string {
	if len(ways) == 0 {
		return "none (off)"
	}
	parts := make([]string, len(ways))
	for i, w := range ways {
		parts[i] = string(w)
	}
	return strings.Join(parts, ", ")
}
