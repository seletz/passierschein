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

1. `mise run test:sdk-core` baut a38, signiert es ad hoc mit `--options runtime` und ohne Entitlements und startet `a38 selftest sdk-core`.
2. `mise run release:app:check` startet `a38 selftest sdk-core` mit dem a38 im signierten Bundle: ad hoc im macOS-Workflow, mit der Developer ID im Release und im signierten Snapshot.
3. Die Prüfung liest die Entitlements von a38.

**Erwartung:**

1. Der Kern des 1Password-SDK lehnt das ungültige Token ab. `a38 selftest sdk-core` endet mit Exit-Code 0.
2. Wie Schritt 1, mit dem a38 des Bundles.
3. a38 trägt weder `allow-jit` noch `allow-unsigned-executable-memory` noch `disable-executable-page-protection` ({{ ref("D59") }}).

**Scheitert, wenn:** macOS a38 tötet (`SIGKILL`, Exit-Status 137, `OS_REASON_CODESIGNING`), `a38 selftest sdk-core` mit einem anderen Exit-Code endet, a38 keine Hardened Runtime hat oder ein Entitlement aus Schritt 3 trägt.

Die Go-Tests prüfen den Befehl und die Auswertung der Antwort des Kerns. Sie laufen ohne Hardened Runtime. Den Fehler aus [Issue 89](https://github.com/seletz/passierschein/issues/89) findet nur das signierte Binary.
