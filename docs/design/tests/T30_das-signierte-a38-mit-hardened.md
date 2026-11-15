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

**Form:** Prüfung der CI mit den Skripten `scripts/release-app-check.sh`, `scripts/signatures-check.sh` und `scripts/sdk-core-check.sh`, dazu Go-Tests. Der Test braucht kein Token und kein Netz.

**Vorbedingung:** a38 ist für macOS gebaut. Die Signatur hat die Hardened Runtime.

**Schritte:**

1. **Statisch (die verlässliche Prüfung):** `mise run release:app:check` liest die Entitlements von a38 und der App im signierten Bundle: ad hoc im macOS-Workflow, mit der Developer ID im Release und im signierten Snapshot. `mise run release:signatures:check` liest die Entitlements des einzelnen a38 der tar-Archive (quill).
2. **Laufzeit:** `release:app:check` startet `a38 selftest sdk-core` mit dem a38 des Bundles. `mise run test:sdk-core` baut a38, signiert es ad hoc mit `--options runtime` und den Entitlements aus `scripts/a38.entitlements` und startet den Befehl ebenso.
3. **Gegenprobe:** `mise run test:sdk-core` signiert eine Kopie ohne Entitlements und startet sie.

**Erwartung:**

1. a38 trägt genau die Entitlements aus `scripts/a38.entitlements`: `com.apple.security.cs.allow-unsigned-executable-memory` ({{ ref("D59") }}). Die App trägt keine.
2. Der Kern des 1Password-SDK lehnt das ungültige Token ab. `a38 selftest sdk-core` endet mit Exit-Code 0.
3. Auf einem Mac, der die Hardened Runtime durchsetzt, stirbt die Kopie mit `SIGKILL` (Exit-Status 137). Läuft die Kopie, meldet das Skript „enforcement not active here“ als Warnung und läuft weiter.

**Scheitert, wenn:** a38 keine Hardened Runtime hat, andere Entitlements als in Schritt 1 trägt, die App ein Entitlement trägt, `a38 selftest sdk-core` nicht mit Exit-Code 0 endet, oder die Kopie anders als mit 0 oder 137 endet.

**Grenze:** Die macOS-Runner von GitHub setzen die Hardened Runtime für Code im Speicher nicht durch. Dort läuft auch a38 ohne Entitlement (Lauf 43726064113, 2026-11-15). Schritte 2 und 3 beweisen auf dem Runner deshalb nichts über die Hardened Runtime. Verlässlich ist dort nur Schritt 1. Schritte 2 und 3 beweisen die Wirkung nur auf einem Mac, der die Regel durchsetzt, etwa mit `mise run test:sdk-core` auf dem Mac der Entwicklung. Dort ist die Gegenprobe am 2026-11-15 mit 137 gestorben.

Die Go-Tests prüfen den Befehl und die Auswertung der Antwort des Kerns. Sie laufen ohne Hardened Runtime.
