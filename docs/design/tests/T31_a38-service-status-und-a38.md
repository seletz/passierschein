---
render_macros: true
id: 'T31'
titel: 'a38 service status und a38 doctor nennen den Abbruch des Daemons wegen der Signatur'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F15, F20, UC14]
---
# T31 — a38 service status und a38 doctor nennen den Abbruch des Daemons wegen der Signatur

{{ design_kopf(page) }}

**Form:** Go-Test mit einem gefälschten `launchctl`. Kein Test fragt das echte launchd.

**Vorbedingung:** Der LaunchAgent ist installiert. Der Daemon läuft nicht.

**Schritte:**

1. `launchctl print` meldet `last exit reason = OS_REASON_CODESIGNING` ohne Exit-Code und ohne Signal. Die Ausgabe stammt aus einem echten Lauf mit eigenem Label (`internal/service/testdata/print-codesigning.txt`).
2. `launchctl print` meldet einen anderen Grund, etwa `OS_REASON_JETSAM`, mit und ohne Exit-Code oder Signal.
3. `a38 service status` und die Prüfung `service` von `a38 doctor` werten die Ausgabe aus.

**Erwartung:**

1. `a38 service status` nennt den Abbruch wegen der Signatur und `a38 selftest sdk-core`. `a38 doctor` meldet einen eigenen Befund. Der nächste Schritt ist `a38 selftest sdk-core` mit dem Programm des LaunchAgent.
2. Die Ausgabe nennt den Grund von launchd. Ein Exit-Code oder Signal behält seine Bedeutung.
3. Keine Ausgabe lautet „no exit known“, wenn launchd einen Grund nennt.

**Scheitert, wenn:** die Ausgabe den Grund `OS_REASON_CODESIGNING` verschweigt oder „no exit known“ meldet.

Belegt: {{ ref("F15") }}, {{ ref("F20") }}, {{ ref("UC14") }}.
