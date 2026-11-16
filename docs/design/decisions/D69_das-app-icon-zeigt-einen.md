---
render_macros: true
id: 'D69'
titel: 'Das App-Icon zeigt einen stilisierten Reisepass'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 10
headless: true
status: gueltig
use_cases: [UC1]
beleg:
  wer: Stefan
  datum: '2026-11-15'
  quelle: ['#109']
---
# D69 — Das App-Icon zeigt einen stilisierten Reisepass

{{ design_kopf(page) }}

**Entscheidung:** `Passierschein.app` trägt ein eigenes App-Icon. Das Icon zeigt einen stilisierten Reisepass: ein petrolfarbenes Heft mit dunklem Rücken, einem goldenen Siegel aus Ring und Schlüsselloch und zwei goldenen Titelzeilen, auf einem sandfarbenen abgerundeten Quadrat (Entwurf 2). Die Quelle ist ein Programm im Repo (`app/Icon/render-icon.swift`). Ein Task erzeugt daraus `AppIcon.icns` in allen Größen, ohne Schritt von Hand. Die `Info.plist` nennt das Icon mit `CFBundleIconFile`.

**Varianten:**

| Entwurf | Motiv | Stärke | Schwäche |
|---|---|---|---|
| 1 | Ausweiskarte mit Fotofeld | Bei 16 Pixel noch als Karte lesbar. | Ähnelt Icons von Kontakt-Apps. |
| 2 (gewählt) | Reisepass mit Siegel aus Ring und Schlüsselloch | Eigenständig. Das Schlüsselloch verweist auf Zugangsdaten. | Bei 16 Pixel nur ein dunkles Heft mit hellem Rand. |
| 3 | Passierschein mit Stempel und Haken | Zeigt die Freigabe. | Ähnelt einem Dokument-Icon. Der Stempel verschwimmt bei 16 Pixel. |

Kein Entwurf nutzt ein fremdes Markenzeichen, eine fremde Figur, ein Staatswappen oder einen echten Ausweis. Die Entwürfe 1 und 3 bleiben in der Quelle. `APP_ICON_DRAFT` wählt sie für einen Vergleich.

**Begründung:** Das Design nannte bisher kein App-Icon. Finder und der Dialog von Touch ID zeigten deshalb das Standard-Icon. Das Motiv stammt von Stefan ([Issue 104](https://github.com/seletz/passierschein/issues/104)). Stefan hat am 2026-11-15 Entwurf 2 gewählt ([Pull Request 109](https://github.com/seletz/passierschein/pull/109)). Ein Programm als Quelle macht das Icon reproduzierbar und prüfbar ({{ ref("T42") }}). Ein Asset-Katalog braucht ein Xcode-Projekt; die App baut aber mit SwiftPM und Skripten.

**Quelle:** [Issue 104](https://github.com/seletz/passierschein/issues/104), Wahl in [Pull Request 109](https://github.com/seletz/passierschein/pull/109).
