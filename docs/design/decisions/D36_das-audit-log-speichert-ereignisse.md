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

**Zeitpunkt:** Der Daemon schreibt jedes Ereignis, sobald es eintritt. Das gilt auch für jeden Fehlschlag: Ablehnung, Ablauf und Fehler. Der Daemon liefert einen Wert erst aus, nachdem der Eintrag für die Auslieferung geschrieben ist.

**Fehler beim Schreiben:** Schlägt das Schreiben in das Audit-Log fehl, liefert der Daemon nichts aus und vergibt keinen Grant. Der Agent erhält den Code `audit_failed`. Der Daemon zeigt dem Menschen sofort ein Popup mit der Meldung. Das Popup informiert nur. Der Mensch beseitigt die Ursache selbst, zum Beispiel Platz schaffen oder den Daemon neu starten. `a38 doctor` hilft bei der Fehlersuche. Der Fehler ist eine Störung des Daemons: Die Auslieferung bleibt danach global gesperrt ({{ ref("D48") }}). Der Daemon schreibt den Fehler zusätzlich in sein Log. Stefan hat das am 2026-11-11 festgelegt; die Regel „keine Auslieferung“ gilt zunächst.

**Begründung:** Die Auswertung nach Projekt, Agent, Zugangsdatum, Zeit und Ergebnis braucht diese Felder. Eine Auslieferung ohne Eintrag wäre nicht nachvollziehbar.

**Offen:** Aufbewahrung und Löschung ({{ ref("G8") }}). Form und Aktionen der Meldung ({{ ref("G20") }}).
