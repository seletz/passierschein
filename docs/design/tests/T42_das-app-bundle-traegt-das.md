---
render_macros: true
id: 'T42'
titel: 'Das App-Bundle trägt das App-Icon in allen Größen'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F11, F19, D69]
---
# T42 — Das App-Bundle trägt das App-Icon in allen Größen

{{ design_kopf(page) }}

**Form:** Prüfung mit den Skripten `scripts/app-icon.sh` und `scripts/app-icon-check.sh` auf macOS. `mise run app:icon`, `mise run app:build` und `mise run release:app:check` führen sie aus, lokal und im Workflow `macos.yml`.

**Vorbedingung:** macOS mit Xcode und der festgelegten Swift-Version.

**Schritte:**

1. `mise run app:icon` zeichnet die Entwürfe aus `app/Icon/render-icon.swift` und baut `AppIcon.icns`. Das Skript zeichnet den gewählten Entwurf ein zweites Mal und vergleicht beide Dateien.
2. `mise run app:build` legt das Entwicklungs-Bundle an und prüft es mit `scripts/app-icon-check.sh`.
3. `mise run release:app:check` prüft das signierte Release-Bundle mit demselben Skript.

**Erwartung:**

1. Beide Läufe liefern dieselbe Datei `AppIcon.icns`.
2. Die `Info.plist` nennt `CFBundleIconFile` = `AppIcon`.
3. `Contents/Resources/AppIcon.icns` enthält zehn Bilder: 16, 32, 128, 256 und 512 Punkt, je in einfacher und doppelter Auflösung, mit der passenden Pixelgröße.
4. Die Signatur des Bundles bleibt gültig ({{ ref("T30") }} prüft sie im selben Lauf).

**Scheitert, wenn:** zwei Läufe verschiedene Dateien liefern, die `Info.plist` das Icon nicht nennt, die Datei fehlt, oder eine Größe fehlt oder falsch ist.

**Grenze:** Der Test prüft nicht, wie das Icon aussieht. Das Aussehen entscheidet Stefan im Pull Request ({{ ref("D69") }}).
