---
render_macros: true
id: 'T45'
titel: 'Das Backend leitet jede Referenz über den Vault an den Client seines Kontos'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 6
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F25, D73]
---
# T45 — Das Backend leitet jede Referenz über den Vault an den Client seines Kontos

{{ design_kopf(page) }}

**Form:** Go-Test mit einer Attrappe der SDK-Schnittstelle je Konto (`TestAccountsRouteByVault` in `internal/backend/onepassword`). Kein Test nutzt ein echtes Konto.

**Schritte:**

1. Zwei Konten `privat` und `firma` mit je einem Vault. Der Test löst je Konto eine Referenz auf und liest sie.
2. Das Konto `firma` hat kein Token.
3. 1Password lehnt das Token des Kontos `privat` ab.
4. Eine Referenz in einem Vault, der nicht in der Konfiguration steht.

**Erwartung:** 1 ruft je Referenz nur die Attrappe des eigenen Kontos. `Vaults` nennt die Vaults beider Konten in der Reihenfolge der Konfiguration. 2 endet für den Vault von `firma` mit `backend_locked`, ohne Aufruf; `privat` liefert. 3 sperrt nur `privat`. 4 endet mit `not_found`, ohne Aufruf.

**Scheitert, wenn:** eine Anfrage den Client eines anderen Kontos nutzt oder ein Konto ohne Token einen Vault eines anderen Kontos sperrt.
