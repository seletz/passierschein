---
render_macros: true
id: 'T48'
titel: 'Freigabeanfrage, Audit-Log und a38 tui nennen das Konto des Vaults'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F26, D74]
---
# T48 — Freigabeanfrage, Audit-Log und a38 tui nennen das Konto des Vaults

{{ design_kopf(page) }}

**Form:** Go-Test über den Broker mit Fake-UI und Audit-Log in SQLite; Test der Swift-App für die Anzeige.

**Schritte:** Ein Bündel mit je einem Eintrag aus den Konten `privat` und `firma`; Freigabe; Auslieferung.

**Erwartung:** Die Nachricht an die Freigabe-App nennt je Eintrag das Konto. Die Ereignisse im Audit-Log speichern das Konto, und `a38 audit verify` prüft das Feld in der Kette. `a38 tui` zeigt die Spalte „Konto“.

**Scheitert, wenn:** ein Eintrag ohne Konto erscheint oder das Konto nicht aus der Konfiguration stammt.
