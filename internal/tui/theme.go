package tui

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
)

// The colours of the palette. They are the series colours of the owner's
// other terminal views, so that the views look alike. Each colour has a
// role in theme.
var (
	paletteBlue   = lipgloss.Color("#4e79a7")
	paletteOrange = lipgloss.Color("#f28e2b")
	paletteGreen  = lipgloss.Color("#59a14f")
	paletteRed    = lipgloss.Color("#e15759")
	palettePurple = lipgloss.Color("#b07aa1")
	paletteTeal   = lipgloss.Color("#76b7b2")
)

// theme is the palette and the styles for one background of the
// terminal. The view starts with the dark theme and changes to the light
// theme when the terminal names a light background.
//
// Colour is only an aid. Each coloured part also differs in a word, a
// glyph, the weight or the position: the active tab is in brackets, the
// selected row has the mark ▸, a state has its word. NO_COLOR removes the
// colours and keeps the rest.
type theme struct {
	// Dark is the background the theme is for.
	Dark bool

	// Accent is the main colour of the frame: the program name, the
	// active tab, the headings, the keys in the footer and the bars.
	// Accent2 and Accent3 are the other stops of the program name.
	Accent, Accent2, Accent3 color.Color
	// Good is a delivery or an approval, Bad a denial or an error,
	// Caution a timeout or a deviation.
	Good, Bad, Caution color.Color
	// Selection is the background of the selected row.
	Selection color.Color

	// Title is a heading, Label a column head or a field name, Strong
	// plain bold, Muted secondary text, Rule the colour of the rules and
	// Border the rounded border of the help.
	Title, Label, Strong, Muted, Rule, Border lipgloss.Style
	// Active is the active tab, Mark the mark ▸ and a key, Selected the
	// selected row, AccentText text in the accent colour.
	Active, Mark, Selected, AccentText lipgloss.Style
	// Figure is a number of the overview.
	Figure lipgloss.Style
	// GoodText, BadText and CautionText colour a state. Alarm is the
	// broken hash chain.
	GoodText, BadText, CautionText, Alarm lipgloss.Style
	// Bar is the colour of the bars of the chart.
	Bar lipgloss.Style
}

// newTheme returns the theme for a dark or a light background.
func newTheme(dark bool) theme {
	t := theme{Dark: dark}
	ld := lipgloss.LightDark(dark)
	t.Accent = t.tone(paletteBlue)
	t.Accent2 = t.tone(paletteTeal)
	t.Accent3 = t.tone(palettePurple)
	t.Good = t.tone(paletteGreen)
	t.Bad = t.tone(paletteRed)
	t.Caution = t.tone(paletteOrange)
	t.Selection = ld(lipgloss.Color("#dbe5f1"), lipgloss.Color("#243447"))

	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	t.Title = fg(t.Accent).Bold(true)
	t.Label = fg(t.Accent).Bold(true)
	t.Strong = lipgloss.NewStyle().Bold(true)
	t.Muted = lipgloss.NewStyle().Faint(true)
	t.Rule = fg(ld(lipgloss.Color("#9fb3c8"), lipgloss.Color("#3a5b82")))
	t.Border = fg(t.Accent)
	t.Active = lipgloss.NewStyle().Background(t.Accent).Foreground(lipgloss.Color("#ffffff")).Bold(true)
	t.Mark = fg(t.Accent).Bold(true)
	t.Selected = lipgloss.NewStyle().Background(t.Selection).Bold(true)
	t.AccentText = fg(t.Accent)
	t.Figure = fg(t.Accent2).Bold(true)
	t.GoodText = fg(t.Good)
	t.BadText = fg(t.Bad).Bold(true)
	t.CautionText = fg(t.Caution)
	t.Alarm = lipgloss.NewStyle().Background(t.Bad).Foreground(lipgloss.Color("#ffffff")).Bold(true)
	t.Bar = fg(t.Accent)
	return t
}

// tone adapts a colour of the palette to the background: as it is on a
// dark one, darker on a light one, where the middle tones lack contrast.
func (t theme) tone(c color.Color) color.Color {
	if t.Dark {
		return c
	}
	return lipgloss.Darken(c, 0.25)
}

// gradient renders s bold, with the colour of its runes blended from
// Accent over Accent2 to Accent3. It is the program name in the header.
func (t theme) gradient(s string) string {
	runes := []rune(s)
	if len(runes) < 3 {
		return t.Title.Render(s)
	}
	colours := lipgloss.Blend1D(len(runes), t.Accent, t.Accent2, t.Accent3)
	var b strings.Builder
	for i, r := range runes {
		b.WriteString(t.Title.Foreground(colours[i]).Render(string(r)))
	}
	return b.String()
}

// help returns the styles of the key hints: the keys in the accent
// colour, the descriptions and the separators muted.
func (t theme) help() help.Styles {
	ld := lipgloss.LightDark(t.Dark)
	key := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	desc := lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#5f6b7a"), lipgloss.Color("#9aa5b1")))
	sep := lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#b8c2cc"), lipgloss.Color("#4a5563")))
	return help.Styles{
		Ellipsis:       sep,
		ShortKey:       key,
		ShortDesc:      desc,
		ShortSeparator: sep,
		FullKey:        key,
		FullDesc:       desc,
		FullSeparator:  sep,
	}
}
