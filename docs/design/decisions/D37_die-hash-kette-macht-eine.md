---
render_macros: true
id: 'D37'
titel: 'Die Hash-Kette macht eine Änderung erkennbar, verhindert sie aber nicht'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 3
headless: true
status: gueltig
use_cases: [UC8]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D37 — Die Hash-Kette macht eine Änderung erkennbar, verhindert sie aber nicht

{{ design_kopf(page) }}

**Entscheidung:** Im MVP gibt es keine Verankerung der Kette außerhalb des Macs. Das Audit-Log gilt als Hilfe für die Auswertung und nicht als Beweis.

**Begründung:** Die Datei gehört dem Benutzer. Wer sie ändern kann, kann die Kette neu berechnen ({{ ref("RI7") }}). Eine Verankerung außerhalb ist ein eigener Baustein.
