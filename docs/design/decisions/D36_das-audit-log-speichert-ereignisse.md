---
render_macros: true
id: 'D36'
titel: 'Das Audit-Log speichert Ereignisse mit SHA-256-Kette, und a38 audit verify prüft die Kette'
bereich: 'Audit'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: gueltig
use_cases: [UC8, UC6]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Vorschlag des Designlaufs, von Stefan im Review am 2026-11-11 abgenommen
---
# D36 — Das Audit-Log speichert Ereignisse mit SHA-256-Kette, und a38 audit verify prüft die Kette

{{ design_kopf(page) }}

**Entscheidung:** Jeder Eintrag hat eine laufende Nummer, einen Zeitpunkt, eine Ereignisart, die Sitzung, die geprüften Fakten, die Angaben des Agents mit dem Namen der Sitzung, die Referenzen und das Ergebnis. Ereignisarten: Anfrage, Bündel, Entscheidung, Grant, Auslieferung, Ablehnung, Widerruf, Ablauf, Fehler. Der Hash eines Eintrags ist SHA-256 über den Hash des vorigen Eintrags und die kanonische Form des Eintrags. `a38 audit verify` berechnet die Kette neu.

**Begründung:** Die Auswertung nach Projekt, Agent, Zugangsdatum, Zeit und Ergebnis braucht diese Felder.

**Offen:** Aufbewahrung und Löschung ({{ ref("G8") }}).
