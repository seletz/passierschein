package opguard

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// wordText returns the text of a word after quote removal, as far as the
// word is literal. Each non-literal part (parameter, command substitution,
// arithmetic, glob, brace expansion) becomes the marker hole.
func wordText(w *syntax.Word) string {
	var b strings.Builder
	for _, part := range w.Parts {
		writePart(&b, part)
	}
	return b.String()
}

// writePart writes one part of a word.
func writePart(b *strings.Builder, part syntax.WordPart) {
	switch p := part.(type) {
	case *syntax.Lit:
		b.WriteString(unescapeUnquoted(p.Value))
	case *syntax.SglQuoted:
		if p.Dollar {
			b.WriteString(unescapeANSI(p.Value))
		} else {
			b.WriteString(p.Value)
		}
	case *syntax.DblQuoted:
		for _, inner := range p.Parts {
			if lit, ok := inner.(*syntax.Lit); ok {
				b.WriteString(unescapeDouble(lit.Value))
			} else {
				b.WriteString(hole)
			}
		}
	default:
		b.WriteString(hole)
	}
}

// unescapeUnquoted removes the backslashes of an unquoted literal: "\x" is
// "x", and a backslash before a newline joins the lines.
func unescapeUnquoted(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == '\n' {
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// unescapeDouble removes the backslashes that are special inside double
// quotes: before "$", "`", "\"", "\\" and a newline.
func unescapeDouble(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("$`\"\\\n", s[i+1]) >= 0 {
			i++
			if s[i] == '\n' {
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// unescapeANSI handles the common escapes of $'...'. An escape that it does
// not know makes the text non-literal (best effort).
func unescapeANSI(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '\\', '\'', '"', '?':
			b.WriteByte(s[i])
		default:
			b.WriteString(hole)
		}
	}
	return b.String()
}
