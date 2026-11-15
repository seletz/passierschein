---
render_macros: true
id: 'T47'
titel: 'a38 doctor zeigt je Konto Quelle, Eintrag und Vaults und meldet den alten Eintrag'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F25, F20, D74, D75]
---
# T47 — a38 doctor zeigt je Konto Quelle, Eintrag und Vaults und meldet den alten Eintrag

{{ design_kopf(page) }}

**Form:** Go-Test der Prüfung „1password token“ mit Attrappen für Schlüsselbund und Dateisystem (`TestTokenSourceAccounts` in `internal/doctor`).

**Schritte:**

1. `privat` mit Quelle `keychain` und vorhandenem Eintrag, `firma` mit Quelle `file` ohne Datei, der Daemon läuft mit `--backend 1password`.
2. Zusätzlich der alte Eintrag `a38.onepassword/service-account`.
3. Ein Konto ohne Vault.

**Erwartung:** 1 zeigt je Konto Quelle, Eintrag oder Datei und die Vaults; für `firma` ein Befund mit dem nächsten Schritt `a38 config onepassword-token`. 2 meldet den alten Eintrag als Befund mit `a38 config onepassword-token delete --legacy`. 3 ist ein Hinweis. Keine Prüfung liest ein Token.

**Scheitert, wenn:** ein Konto fehlt, der alte Eintrag nicht erscheint oder eine Prüfung einen Wert liest.
