---
render_macros: true
id: 'D69'
titel: 'Das App-Icon zeigt eine stilisierte Ausweiskarte'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 10
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  notiz: Vorschlag aus Issue 104, Wahl des Entwurfs durch Stefan
  annahme: true
---
# D69 — Das App-Icon zeigt eine stilisierte Ausweiskarte

{{ design_kopf(page) }}

**Entscheidung:** `Passierschein.app` trägt ein eigenes App-Icon. Das Icon zeigt eine stilisierte Ausweiskarte: eine weiße Karte mit dunkelblauem Kopfband, einem Fotofeld mit neutraler Figur und Textzeilen, auf einem blauen abgerundeten Quadrat (Entwurf 1). Die Quelle ist ein Programm im Repo (`app/Icon/render-icon.swift`). Ein Task erzeugt daraus `AppIcon.icns` in allen Größen, ohne Schritt von Hand. Die `Info.plist` nennt das Icon mit `CFBundleIconFile`.

**Varianten:**

| Entwurf | Motiv | Stärke | Schwäche |
|---|---|---|---|
| 1 | Ausweiskarte mit Fotofeld | Bei 16 Pixel noch als Karte lesbar. Das Motiv passt zu „Ausweis für Zugangsdaten“. | Ähnelt Icons von Kontakt-Apps. |
| 2 | Reisepass mit Siegel aus Ring und Schlüsselloch | Eigenständig, das Schlüsselloch verweist auf Zugangsdaten. | Bei 16 Pixel nur ein dunkles Rechteck. |
| 3 | Passierschein mit Stempel und Haken | Zeigt die Freigabe. | Ähnelt einem Dokument-Icon. Der Stempel verschwimmt bei 16 Pixel. |

Kein Entwurf nutzt ein fremdes Markenzeichen, eine fremde Figur, ein Staatswappen oder einen echten Ausweis.

**Begründung:** Das Design nennt bisher kein App-Icon. Finder und der Dialog von Touch ID zeigen deshalb das Standard-Icon. Das Motiv stammt von Stefan ([Issue 104](https://github.com/seletz/passierschein/issues/104)). Entwurf 1 ist der Vorschlag, weil er in der kleinsten Größe lesbar bleibt. Ein Programm als Quelle macht das Icon reproduzierbar und prüfbar ({{ ref("T42") }}). Ein Asset-Katalog braucht Xcode-Projekte; die App baut aber mit SwiftPM und Skripten.

**Quelle:** [Issue 104](https://github.com/seletz/passierschein/issues/104). Die Wahl des Entwurfs trifft Stefan im Pull Request.
