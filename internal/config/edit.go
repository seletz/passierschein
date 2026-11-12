package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Edit is the result of a change of the configuration file.
type Edit struct {
	// Data is the new content of the file.
	Data []byte
	// Notes lists findings that do not stop the change, for example a
	// project path that does not exist yet.
	Notes []string
}

// AddVault adds a vault at the end of the vault list of the file content
// data. It checks the change with the rules of [Parse] before it returns
// it, and it keeps all other bytes of the file (comments, order, blank
// lines). The name must not be in the list, and the environment must be
// dev, test or prod.
func AddVault(data []byte, name string, env Environment) (Edit, error) {
	cfg, err := loadForEdit(data)
	if err != nil {
		return Edit{}, err
	}
	if err := checkName(name); err != nil {
		return Edit{}, fmt.Errorf("the vault name %v", err)
	}
	if !env.Valid() {
		return Edit{}, fmt.Errorf("the environment %q is not dev, test or prod", string(env))
	}
	if _, ok := cfg.Vault(name); ok {
		return Edit{}, fmt.Errorf("the vault %q is in the list already", name)
	}
	item := []string{"- name: " + quote(name), "  environment: " + string(env)}
	return splice(data, func(e *splicer) error { return e.appendItem("vaults", item) })
}

// RemoveVault removes a vault from the vault list. It refuses if a project
// still names the vault; the error names the projects.
func RemoveVault(data []byte, name string) (Edit, error) {
	cfg, err := loadForEdit(data)
	if err != nil {
		return Edit{}, err
	}
	idx := slices.IndexFunc(cfg.Vaults, func(v Vault) bool { return v.Name == name })
	if idx < 0 {
		return Edit{}, fmt.Errorf("the vault %q is not in the list", name)
	}
	var users []string
	for _, p := range cfg.Projects {
		if slices.Contains(p.Vaults, name) {
			users = append(users, fmt.Sprintf("%q", p.Name))
		}
	}
	if len(users) > 0 {
		return Edit{}, fmt.Errorf("the vault %q is still used by the projects %s; remove the vault from these projects first (a38 config project remove)",
			name, strings.Join(users, ", "))
	}
	return splice(data, func(e *splicer) error { return e.removeItem("vaults", idx) })
}

// AddProject adds a project at the end of the project list. Each path
// must be absolute or start with "~/" (the file keeps the text as given).
// Each vault must be in the vault list. A path that does not exist is a
// note, not a failure.
func AddProject(data []byte, name string, paths, vaults []string) (Edit, error) {
	cfg, err := loadForEdit(data)
	if err != nil {
		return Edit{}, err
	}
	if err := checkName(name); err != nil {
		return Edit{}, fmt.Errorf("the project name %v", err)
	}
	if _, ok := cfg.Project(name); ok {
		return Edit{}, fmt.Errorf("the project %q is in the list already", name)
	}
	if len(paths) == 0 {
		return Edit{}, errors.New("a project needs at least one path (--path)")
	}
	if len(vaults) == 0 {
		return Edit{}, errors.New("a project needs at least one vault (--vault)")
	}
	for _, p := range paths {
		if _, err := absPath(p); err != nil {
			return Edit{}, fmt.Errorf("the path %q %v", p, err)
		}
	}
	for _, v := range vaults {
		if _, ok := cfg.Vault(v); !ok {
			return Edit{}, fmt.Errorf("the vault %q is not in the vault list; add it first (a38 config vault add)", v)
		}
	}
	item := []string{"- name: " + quote(name), "  paths:"}
	for _, p := range paths {
		item = append(item, "    - "+quote(p))
	}
	item = append(item, "  vaults:")
	for _, v := range vaults {
		item = append(item, "    - "+quote(v))
	}
	out, err := splice(data, func(e *splicer) error { return e.appendItem("projects", item) })
	if err != nil {
		return Edit{}, err
	}
	// Parse(out) succeeded in splice, so it succeeds here.
	if added, err := Parse(out.Data); err == nil {
		if p, ok := added.Project(name); ok && len(p.MissingPaths) > 0 {
			out.Notes = append(out.Notes, "these paths do not exist yet: "+strings.Join(p.MissingPaths, ", "))
		}
	}
	return out, nil
}

// RemoveProject removes a project from the project list.
func RemoveProject(data []byte, name string) (Edit, error) {
	cfg, err := loadForEdit(data)
	if err != nil {
		return Edit{}, err
	}
	idx := slices.IndexFunc(cfg.Projects, func(p Project) bool { return p.Name == name })
	if idx < 0 {
		return Edit{}, fmt.Errorf("the project %q is not in the list", name)
	}
	return splice(data, func(e *splicer) error { return e.removeItem("projects", idx) })
}

// loadForEdit parses the file before a change. A file that does not load
// is not edited: the human fixes it first.
func loadForEdit(data []byte) (*Config, error) {
	if strings.Contains(string(data), "\r\n") {
		return nil, errors.New("the file has Windows line endings (CRLF); convert it to LF before you use this command")
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("the file is not valid, so a38 does not change it; fix it first (a38 config validate):\n%w", err)
	}
	return cfg, nil
}

// quote returns s as a YAML scalar. A text without a special character
// stays plain.
func quote(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	return strings.TrimRight(string(out), "\n")
}

// splicer changes the lines of a file. It reads the YAML node tree only
// to find lines; the text of all other lines stays as it is. (A
// re-encoding of the tree would drop the blank lines and change the
// indentation.)
type splicer struct {
	lines []string
	root  *yaml.Node
}

// splice parses data, runs the change on the lines, checks the result
// with [Parse] and returns it.
func splice(data []byte, change func(*splicer) error) (Edit, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return Edit{}, errors.New("the file is not a YAML mapping")
	}
	s := &splicer{lines: strings.Split(string(data), "\n"), root: doc.Content[0]}
	if err := change(s); err != nil {
		return Edit{}, err
	}
	out := []byte(strings.Join(s.lines, "\n"))
	if _, err := Parse(out); err != nil {
		return Edit{}, fmt.Errorf("the change makes the file invalid, so a38 does not write it:\n%w", err)
	}
	return Edit{Data: out}, nil
}

// entry returns the key and the value node of a top-level key.
func (s *splicer) entry(key string) (k, v *yaml.Node) {
	for i := 0; i+1 < len(s.root.Content); i += 2 {
		if s.root.Content[i].Value == key {
			return s.root.Content[i], s.root.Content[i+1]
		}
	}
	return nil, nil
}

var errStyle = "the key %q is written in a style that a38 does not edit (for example a list in flow style [..] over many lines); edit the file by hand"

// keyLine matches the line of a key that has no value, or an empty list.
func keyLine(key string) *regexp.Regexp {
	return regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(key) + `:)[ \t]*(\[\s*\]|~|null)?[ \t]*(#.*)?$`)
}

// setKeyLine rewrites the line of the key. With empty true it writes
// "key: []", else "key:". A trailing comment stays.
func (s *splicer) setKeyLine(k *yaml.Node, key string, empty bool) error {
	i := k.Line - 1
	m := keyLine(key).FindStringSubmatch(s.lines[i])
	if m == nil {
		return fmt.Errorf(errStyle, key)
	}
	line := m[1]
	if empty {
		line += " []"
	}
	if m[3] != "" {
		line += " " + m[3]
	}
	s.lines[i] = line
	return nil
}

// isEmptyValue reports whether the list has no items: absent value, null
// or "[]".
func isEmptyValue(v *yaml.Node) bool {
	return (v.Kind == yaml.ScalarNode && v.Tag == "!!null") || (v.Kind == yaml.SequenceNode && len(v.Content) == 0)
}

// lastLine returns the last line (1-based) that the node uses.
func lastLine(n *yaml.Node) int {
	last := n.Line
	for _, c := range n.Content {
		last = max(last, lastLine(c))
	}
	return last
}

func indentLines(item []string, indent string) []string {
	out := make([]string, len(item))
	for i, l := range item {
		out[i] = indent + l
	}
	return out
}

// insertAfter inserts lines after the 1-based line n.
func (s *splicer) insertAfter(n int, lines []string) {
	s.lines = slices.Insert(s.lines, n, lines...)
}

// appendItem adds a list item at the end of the list under the key.
func (s *splicer) appendItem(key string, item []string) error {
	k, v := s.entry(key)
	switch {
	case k == nil:
		// The key is absent: add it at the end of the file.
		for len(s.lines) > 0 && s.lines[len(s.lines)-1] == "" {
			s.lines = s.lines[:len(s.lines)-1]
		}
		s.lines = append(s.lines, "", key+":")
		s.lines = append(s.lines, indentLines(item, "  ")...)
		s.lines = append(s.lines, "")
	case isEmptyValue(v):
		if err := s.setKeyLine(k, key, false); err != nil {
			return err
		}
		s.insertAfter(k.Line, indentLines(item, strings.Repeat(" ", k.Column-1+2)))
	case v.Kind == yaml.SequenceNode && v.Style&yaml.FlowStyle == 0:
		first := v.Content[0]
		dash := first.Column - 1 - 2
		if dash < 0 || !strings.HasPrefix(strings.TrimLeft(s.lines[first.Line-1], " "), "- ") {
			return fmt.Errorf(errStyle, key)
		}
		s.insertAfter(lastLine(v.Content[len(v.Content)-1]), indentLines(item, strings.Repeat(" ", dash)))
	default:
		return fmt.Errorf(errStyle, key)
	}
	return nil
}

// removeItem removes the list item with the index. A comment line above
// the item stays.
func (s *splicer) removeItem(key string, idx int) error {
	k, v := s.entry(key)
	if k == nil || v.Kind != yaml.SequenceNode || v.Style&yaml.FlowStyle != 0 || idx >= len(v.Content) {
		return fmt.Errorf(errStyle, key)
	}
	item := v.Content[idx]
	start, end := item.Line, lastLine(item)
	if !strings.HasPrefix(strings.TrimLeft(s.lines[start-1], " "), "- ") {
		return fmt.Errorf(errStyle, key)
	}
	s.lines = slices.Delete(s.lines, start-1, end)
	if len(v.Content) == 1 {
		return s.setKeyLine(k, key, true)
	}
	return nil
}

// Inspection tells where the values of a file come from.
type Inspection struct {
	// Present lists each key that the file sets, with the dotted name,
	// for example "grant.lifetime". A key that is not here has its
	// default.
	Present map[string]bool
	// Written maps a project name to its paths as written in the file.
	Written map[string][]string
}

// Inspect reads the keys of a file. It does not check the values; use
// [Parse] for that.
func Inspect(data []byte) (Inspection, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Inspection{}, err
	}
	in := Inspection{Present: map[string]bool{}, Written: map[string][]string{}}
	if len(doc.Content) == 1 {
		walkKeys(doc.Content[0], "", in.Present)
	}
	var raw rawFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Inspection{}, err
	}
	for _, p := range raw.Projects {
		in.Written[p.Name] = p.Paths
	}
	return in, nil
}

// walkKeys records the dotted names of the scalar keys. The lists
// "vaults" and "projects" count as one key each.
func walkKeys(n *yaml.Node, prefix string, out map[string]bool) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		name := prefix + n.Content[i].Value
		out[name] = true
		walkKeys(n.Content[i+1], name+".", out)
	}
}

// TildePath writes a path below the home directory with a leading "~".
func TildePath(path string) string {
	home, err := homeDir()
	if err != nil {
		return path
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + rest
	}
	return path
}
