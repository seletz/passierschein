---
render_macros: true
id: 'T46'
titel: 'Der Daemon startet mit einem Konto ohne Token und sperrt nur dessen Vaults'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 7
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F25, D73]
---
# T46 — Der Daemon startet mit einem Konto ohne Token und sperrt nur dessen Vaults

{{ design_kopf(page) }}

**Form:** Go-Test über das Lesen der Tokens beim Start (`TestOpenAccounts` in `cmd/a38`) mit Quellen im Arbeitsspeicher.

**Schritte:**

1. Zwei Konten, beide Quellen liefern ein Token.
2. Die Quelle von `firma` hat kein Token.
3. Keine Quelle hat ein Token.
4. Ein drittes Konto ohne Vault.

**Erwartung:** 1 liefert beide Tokens. 2 liefert das Token von `privat`, sperrt `firma`, und das Log nennt `firma` mit dem nächsten Schritt. 3 bricht den Start ab; die Meldung nennt beide Konten. 4 liest die Quelle des dritten Kontos nicht. Kein Log und keine Meldung enthält den Testwert.

**Scheitert, wenn:** der Daemon mit einem fehlenden Token eines Kontos nicht startet, ohne jedes Token startet oder die Quelle eines Kontos ohne Vault liest.
