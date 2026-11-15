---
render_macros: true
id: 'T30'
titel: 'Das signierte a38 mit Hardened Runtime führt den Kern des 1Password-SDK aus'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F12, F19, D59, UC11]
---
# T30 — Das signierte a38 mit Hardened Runtime führt den Kern des 1Password-SDK aus

{{ design_kopf(page) }}

**Form:** Prüfung der CI mit dem Skript `scripts/sdk-core-check.sh`, dazu Go-Tests. Der Test braucht kein Token und kein Netz.

**Vorbedingung:** a38 ist für macOS gebaut. Die Signatur hat die Hardened Runtime.

**Schritte:**

1. `mise run test:sdk-core` baut a38, signiert es ad hoc mit `--options runtime` und den Entitlements aus `scripts/a38.entitlements` und startet `a38 selftest sdk-core`. Danach signiert er eine Kopie ohne Entitlements und startet sie ebenso.
2. `mise run release:app:check` startet `a38 selftest sdk-core` mit dem a38 im signierten Bundle: ad hoc im macOS-Workflow, mit der Developer ID im Release und im signierten Snapshot.
3. Die Prüfung liest die Entitlements von a38 und der App.

**Erwartung:**

1. Der Kern des 1Password-SDK lehnt das ungültige Token ab. `a38 selftest sdk-core` endet mit Exit-Code 0. Die Kopie ohne Entitlements stirbt mit `SIGKILL` (Exit-Status 137). So zeigt der Test, dass das Entitlement nötig ist.
2. Wie Schritt 1, mit dem a38 des Bundles.
3. a38 trägt genau die Entitlements aus `scripts/a38.entitlements`: `com.apple.security.cs.allow-unsigned-executable-memory` ({{ ref("D59") }}). Die App trägt keine. `mise run release:signatures:check` prüft dasselbe für das einzelne a38 der tar-Archive.

**Scheitert, wenn:** macOS a38 tötet (`SIGKILL`, Exit-Status 137, `OS_REASON_CODESIGNING`), `a38 selftest sdk-core` mit einem anderen Exit-Code endet, a38 keine Hardened Runtime hat, andere Entitlements als in Schritt 3 trägt, oder die Kopie ohne Entitlements nicht stirbt.

Die Go-Tests prüfen den Befehl und die Auswertung der Antwort des Kerns. Sie laufen ohne Hardened Runtime. Den Fehler aus [Issue 89](https://github.com/seletz/passierschein/issues/89) findet nur das signierte Binary.
