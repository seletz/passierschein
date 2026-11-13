package tui

import "charm.land/bubbles/v2/key"

// keyMap holds the key bindings. The footer and the help are made from
// it, so they always show the keys that work. The views switch by number,
// and tab and shift+tab go to the next and the previous view.
type keyMap struct {
	Overview, Events, Sessions, Views key.Binding
	NextView, PrevView                key.Binding
	Help, Back, Quit, ForceQuit       key.Binding
	Reload                            key.Binding

	Up, Down, PageUp, PageDown, Top, Bottom, Scroll key.Binding
	Open, Detail, Timeline                          key.Binding
	Search, Filter, Clear, Range                    key.Binding

	Apply, Cancel key.Binding
}

func newKeyMap() keyMap {
	b := func(keys []string, shown, desc string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(shown, desc))
	}
	return keyMap{
		Overview:  b([]string{"1"}, "1", "overview"),
		Events:    b([]string{"2"}, "2", "events"),
		Sessions:  b([]string{"3"}, "3", "sessions"),
		Views:     b([]string{"1", "2", "3"}, "1/2/3", "view"),
		Help:      b([]string{"?"}, "?", "help"),
		Back:      b([]string{"esc", "backspace"}, "esc", "back"),
		Quit:      b([]string{"q"}, "q", "quit"),
		ForceQuit: b([]string{"ctrl+c"}, "^c", "quit"),
		NextView:  b([]string{"tab"}, "tab", "next view"),
		PrevView:  b([]string{"shift+tab"}, "⇧tab", "previous view"),
		Reload:    b([]string{"r", "R"}, "r", "reload"),

		Up:       b([]string{"up", "k"}, "↑/k", "up"),
		Down:     b([]string{"down", "j"}, "↓/j", "down"),
		PageUp:   b([]string{"pgup", "b"}, "pgup/b", "page up"),
		PageDown: b([]string{"pgdown", "space"}, "pgdn/space", "page down"),
		Top:      b([]string{"home", "g"}, "g/home", "top"),
		Bottom:   b([]string{"end", "G"}, "G/end", "bottom"),
		Scroll:   b([]string{"up", "k", "down", "j"}, "↑/↓", "scroll"),
		Open:     b([]string{"enter"}, "enter", "open"),
		Detail:   b([]string{"enter"}, "enter", "detail"),
		Timeline: b([]string{"enter"}, "enter", "timeline"),

		Search: b([]string{"/"}, "/", "search"),
		Filter: b([]string{"f"}, "f", "filter"),
		Clear:  b([]string{"c"}, "c", "clear filter"),
		Range:  b([]string{"t"}, "t", "range"),

		Apply:  b([]string{"enter"}, "enter", "apply"),
		Cancel: b([]string{"esc"}, "esc", "cancel"),
	}
}

// global returns the global hints of the footer.
func (k keyMap) global() []key.Binding {
	return []key.Binding{k.Views, k.Help, k.Quit}
}

// fullGlobal returns the global keys of the help as one column.
func (k keyMap) fullGlobal() []key.Binding {
	return []key.Binding{k.Overview, k.Events, k.Sessions, k.NextView, k.PrevView, k.Reload, k.Help, k.Back, k.Quit, k.ForceQuit}
}
