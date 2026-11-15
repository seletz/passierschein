---
render_macros: true
id: 'T43'
titel: 'Die Konfiguration lädt nur mit einem Konto an jedem Vault und nennt beim alten Format den Umbau'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F25, D70, D71, D75]
---
# T43 — Die Konfiguration lädt nur mit einem Konto an jedem Vault und nennt beim alten Format den Umbau

{{ design_kopf(page) }}

**Form:** Go-Test, tabellengesteuert, über das Laden der Konfiguration (`TestAccounts` in `internal/config`).

**Schritte:** Je Zeile eine Datei:

1. zwei Konten `privat` (Quelle `keychain`) und `firma` (Quelle `file`), jeder Vault mit Konto,
2. ein Vault ohne `account`,
3. ein Vault mit einem Konto, das nicht in `accounts` steht,
4. der alte Schlüssel `backend.onepassword.token_source`,
5. der alte Schlüssel `backend.onepassword.token_file`,
6. derselbe Name eines Kontos zweimal,
7. ein ungültiger Name: Großbuchstabe, Bindestrich am Anfang, 33 Zeichen, `service-account`,
8. zwei Konten mit derselben Token-Datei,
9. ein Konto ohne Vault,
10. eine unbekannte Quelle.

**Erwartung:** 1 lädt; das Konto `privat` hat die Standard-Datei `onepassword-token-privat`. 9 lädt und nennt das Konto als unbenutzt. Alle anderen Dateien laden nicht. Die Meldung nennt den Schlüssel. Die Meldung zu 2 bis 5 nennt den nächsten Schritt aus {{ ref("D75") }}.

**Scheitert, wenn:** eine Datei ohne Konto am Vault lädt, ein Vault ein Standardkonto bekommt oder eine Meldung den nächsten Schritt nicht nennt.
