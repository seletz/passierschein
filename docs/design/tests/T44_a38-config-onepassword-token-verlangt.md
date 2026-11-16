---
render_macros: true
id: 'T44'
titel: 'a38 config onepassword-token verlangt das Konto und zeigt je Konto nur Metadaten'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F25, D72]
---
# T44 — a38 config onepassword-token verlangt das Konto und zeigt je Konto nur Metadaten

{{ design_kopf(page) }}

**Form:** Go-Test über `runTokenWith` mit einem Speicher im Arbeitsspeicher statt des Schlüsselbunds (`TestTokenAccounts` in `cmd/a38`). Kein Test liest den Eintrag `a38.onepassword`.

**Schritte:**

1. `set` ohne `--account`,
2. `set --account unbekannt`, das Konto steht nicht in der Konfiguration,
3. `set --account privat` mit einem Testwert am Terminal,
4. `status` ohne `--account`, mit den Konten `privat` und `firma` und dem alten Eintrag,
5. `status --account firma` mit der Quelle `file`,
6. `delete --account privat` und `delete --legacy`.

**Erwartung:** 1 endet mit Exit-Code 2 und liest nichts. 2 endet mit Exit-Code 1 und liest nichts. 3 legt den Eintrag mit dem Konto `privat` an. 4 zeigt je Konto Quelle und Eintrag und nennt den alten Eintrag mit `delete --legacy`. 5 zeigt nur die Datei des Kontos. 6 löscht genau den genannten Eintrag. Keine Ausgabe enthält den Testwert.

**Scheitert, wenn:** ein Befehl ohne Konto ein Token liest, einen fremden Eintrag ändert oder einen Wert zeigt.
