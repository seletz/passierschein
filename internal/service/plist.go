package service

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Spec is the content of the plist that "a38 service install" writes.
type Spec struct {
	// Program is the absolute real path of a38.
	Program string
	// Args are the arguments after the program, for example
	// serve --backend 1password --ui app.
	Args []string
	// LogPath receives stdout and stderr of the daemon.
	LogPath string
	// Env holds environment variables for the daemon. Only names of
	// [AllowedEnv] are written; a secret never goes here.
	Env map[string]string
}

// AllowedEnv lists the environment variables that the plist may pass to
// the daemon. They move the default paths of the configuration and of the
// sockets (D49); none of them holds a secret.
var AllowedEnv = []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME"}

// umask is the file mode creation mask of the daemon: 077, as decimal for
// the plist. The log file and each file of the daemon belong to the user
// alone.
const umask = 0o077

// exitTimeout is the time in seconds between SIGTERM and SIGKILL when
// launchd stops the daemon. The default of an agent is 5 seconds; the
// daemon needs the time to end the grants with an audit event (T16) and to
// close the audit log.
const exitTimeout = 20

// Render returns the plist as XML. The output is the same for the same
// spec: the keys have a fixed sequence.
func (s Spec) Render() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<!-- " + Marker + " -->\n")
	b.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	key := func(k string) { b.WriteString("\t<key>" + esc(k) + "</key>\n") }
	str := func(indent, v string) { b.WriteString(indent + "<string>" + esc(v) + "</string>\n") }

	key("Label")
	str("\t", Label)
	key("ProgramArguments")
	b.WriteString("\t<array>\n")
	str("\t\t", s.Program)
	for _, a := range s.Args {
		str("\t\t", a)
	}
	b.WriteString("\t</array>\n")
	key("RunAtLoad")
	b.WriteString("\t<true/>\n")
	key("KeepAlive")
	b.WriteString("\t<false/>\n")
	key("LimitLoadToSessionType")
	str("\t", "Aqua")
	key("ProcessType")
	str("\t", "Interactive")
	key("AssociatedBundleIdentifiers")
	b.WriteString("\t<array>\n")
	str("\t\t", BundleID)
	b.WriteString("\t</array>\n")
	key("ExitTimeOut")
	b.WriteString("\t<integer>" + strconv.Itoa(exitTimeout) + "</integer>\n")
	key("WorkingDirectory")
	str("\t", "/")
	key("Umask")
	b.WriteString("\t<integer>" + strconv.Itoa(umask) + "</integer>\n")
	key("StandardOutPath")
	str("\t", s.LogPath)
	key("StandardErrorPath")
	str("\t", s.LogPath)
	var names []string
	for k := range s.Env {
		if slices.Contains(AllowedEnv, k) {
			names = append(names, k)
		}
	}
	if len(names) > 0 {
		slices.Sort(names)
		key("EnvironmentVariables")
		b.WriteString("\t<dict>\n")
		for _, k := range names {
			b.WriteString("\t\t<key>" + esc(k) + "</key>\n")
			str("\t\t", s.Env[k])
		}
		b.WriteString("\t</dict>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes()
}

// esc escapes a text for XML.
func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// HasMarker reports whether data is a plist that "a38 service install"
// wrote.
func HasMarker(data []byte) bool { return bytes.Contains(data, []byte("<!-- "+Marker+" -->")) }

// Plist is the part of a plist that the package reads back.
type Plist struct {
	// Label is the value of the key Label.
	Label string
	// ProgramArguments is the value of the key ProgramArguments.
	ProgramArguments []string
	// Values holds each top-level key with its decoded value: string,
	// bool, int64, []any or map[string]any.
	Values map[string]any
}

// Program returns the first element of ProgramArguments, or "".
func (p Plist) Program() string {
	if len(p.ProgramArguments) == 0 {
		return ""
	}
	return p.ProgramArguments[0]
}

// Arg returns the value of the flag name (for example "--config") in
// ProgramArguments, or "".
func (p Plist) Arg(name string) string {
	for i, a := range p.ProgramArguments {
		if a == name && i+1 < len(p.ProgramArguments) {
			return p.ProgramArguments[i+1]
		}
		if v, ok := strings.CutPrefix(a, name+"="); ok {
			return v
		}
	}
	return ""
}

// ParsePlist reads an XML plist with a dictionary at the top. It knows the
// value types of launchd.plist: string, integer, true, false, array and
// dict. A binary plist is an error.
func ParsePlist(data []byte) (Plist, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = true
	var p Plist
	for {
		tok, err := d.Token()
		if err != nil {
			return p, fmt.Errorf("the file is no XML plist: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local != "plist" {
				return p, errors.New("the file is no XML plist")
			}
			break
		}
	}
	v, err := decodeNext(d)
	if err != nil {
		return p, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return p, errors.New("the plist has no dictionary at the top")
	}
	p.Values = m
	p.Label, _ = m["Label"].(string)
	if args, ok := m["ProgramArguments"].([]any); ok {
		for _, a := range args {
			s, ok := a.(string)
			if !ok {
				return p, errors.New("ProgramArguments holds a value that is not a string")
			}
			p.ProgramArguments = append(p.ProgramArguments, s)
		}
	}
	return p, nil
}

// errEnd marks the end element of an array or a dict.
var errEnd = errors.New("end")

// decodeNext decodes the next value. It returns errEnd at an end element.
func decodeNext(d *xml.Decoder) (any, error) {
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.New("the plist ends too early")
			}
			return nil, fmt.Errorf("the file is no XML plist: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil, errEnd
		case xml.StartElement:
			return decodeElement(d, t)
		}
	}
}

func decodeElement(d *xml.Decoder, se xml.StartElement) (any, error) {
	switch se.Name.Local {
	case "string", "key":
		var s string
		if err := d.DecodeElement(&s, &se); err != nil {
			return nil, err
		}
		return s, nil
	case "integer":
		var s string
		if err := d.DecodeElement(&s, &se); err != nil {
			return nil, err
		}
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("the plist has an invalid integer %q", s)
		}
		return n, nil
	case "true", "false":
		if err := d.Skip(); err != nil {
			return nil, err
		}
		return se.Name.Local == "true", nil
	case "array":
		out := []any{}
		for {
			v, err := decodeNext(d)
			if errors.Is(err, errEnd) {
				return out, nil
			}
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	case "dict":
		out := map[string]any{}
		for {
			k, err := decodeNext(d)
			if errors.Is(err, errEnd) {
				return out, nil
			}
			if err != nil {
				return nil, err
			}
			name, ok := k.(string)
			if !ok {
				return nil, errors.New("the plist has a dictionary without a key")
			}
			v, err := decodeNext(d)
			if err != nil {
				return nil, fmt.Errorf("the key %q has no value", name)
			}
			out[name] = v
		}
	}
	return nil, fmt.Errorf("the plist has the unknown element %q", se.Name.Local)
}
