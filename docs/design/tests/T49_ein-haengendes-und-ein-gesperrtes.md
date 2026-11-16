---
render_macros: true
id: 'T49'
titel: 'Ein hängendes und ein gesperrtes Konto stören den Status eines gesunden Kontos nicht'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-16'
order: 8
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F25, D78]
---
# T49 — Ein hängendes und ein gesperrtes Konto stören den Status eines gesunden Kontos nicht

{{ design_kopf(page) }}

**Form:** Go-Tests mit einem gefälschten SDK je Konto und mit gefälschten Zuständen der Konten. Kein echtes Token, kein Schlüsselbund, kein echtes Konto.

| Test | Paket |
|---|---|
| `TestAccountsCheckEachAccount`, `TestAccountsVaultsSideBySide` | `internal/backend/onepassword` |
| `TestStatusEachAccount` | `internal/broker` |
| `TestRoundTrip`, `TestEncodeRejects` (Feld `accounts`) | `internal/protocol` |
| `TestBackendCheckEachAccount`, `TestBackendCheckNoAccountAnswers` | `internal/doctor` |
| `TestLogStartCheck`, `TestTUIAccountStates` | `cmd/a38` |
| `TestAccountStates` | `internal/tui` |

**Schritte:**

1. Ein hängendes Konto `firma` vor einem gesunden Konto `privat`.
2. Ein gesperrtes Konto `labor` (kein Token) neben `privat`.
3. Zwei hängende Konten, ein gesperrtes und ein gesundes Konto.
4. 1Password lehnt das Token von `firma` ab.
5. Status, `a38 doctor` und `a38 tui` mit diesen Zuständen.

**Erwartung:** `privat` ist in jedem Schritt `ok` und sieht seine Vaults. Das hängende Konto ist `unreachable`, und der Grund nennt die Frist. Das gesperrte Konto und das abgelehnte Token sind `locked`. Zwei hängende Konten kosten eine Frist, nicht zwei. Das Backend ist `ok`, solange ein Konto antwortet. `a38 doctor` zeigt je Konto eine Zeile mit dem nächsten Schritt. `a38 tui` zeigt den Zustand jedes Kontos. Die Reihenfolge ist die der Konfiguration. Kein Text enthält ein Token.

**Scheitert, wenn:** ein langsames Konto die Frist eines anderen Kontos verbraucht, der Status nur einen Zustand für alle Konten nennt oder ein gesperrtes Konto ein gesundes Konto als gestört meldet.
