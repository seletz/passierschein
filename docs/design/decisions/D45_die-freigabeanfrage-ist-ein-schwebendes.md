---
render_macros: true
id: 'D45'
titel: 'Die Freigabeanfrage ist ein schwebendes NSPanel, das den Fokus nicht nimmt'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 7
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D45 — Die Freigabeanfrage ist ein schwebendes NSPanel, das den Fokus nicht nimmt

{{ design_kopf(page) }}

**Entscheidung:** Das Fenster ist ein `NSPanel` mit `.nonactivatingPanel` auf der Ebene `.floating`, ohne Schließen-Button, in einer App ohne Dock-Icon. OmniWM braucht dafür keine App-Regel.

**Begründung:** Im Spike hat OmniWM 0.8.0 das Fenster in sechs Läufen nicht gekachelt und nicht als Fenster geführt.

**Grenze:** Die Sichtprüfung über gekachelten Fenstern, auf allen Arbeitsflächen und im Vollbild steht aus ({{ ref("G4") }}).

**Quelle:** [Spike Freigabe-UI](https://github.com/seletz/passierschein/issues/4).
