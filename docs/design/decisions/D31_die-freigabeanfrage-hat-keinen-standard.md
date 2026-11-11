---
render_macros: true
id: 'D31'
titel: 'Die Freigabeanfrage hat keinen Standard-Button, eine Verzögerung und nur Touch ID als Bestätigung'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 5
headless: true
status: gueltig
use_cases: [UC1, UC4]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D31 — Die Freigabeanfrage hat keinen Standard-Button, eine Verzögerung und nur Touch ID als Bestätigung

{{ design_kopf(page) }}

**Entscheidung:** Kein Button reagiert auf die Eingabetaste. Kein Häkchen ist vorab gesetzt. Die Bestätigung ist erst nach 1,5 Sekunden aktiv. Nur Touch ID bestätigt; ein Passwort als Ersatz gibt es nicht. Das Fenster nimmt beim Erscheinen nicht den Tastaturfokus.

**Begründung:** Ein Popup erscheint oft während des Tippens. Eine versehentliche Freigabe muss unmöglich sein ({{ ref("RI6") }}).

**Quelle:** [Spike Freigabe-UI](https://github.com/seletz/passierschein/issues/4). Der echte Dialog von Touch ID ist nicht getestet ({{ ref("G4") }}).
