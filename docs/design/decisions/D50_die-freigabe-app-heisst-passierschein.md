---
render_macros: true
id: 'D50'
titel: 'Die Freigabe-App heißt Passierschein, das Kommandozeilenwerkzeug a38'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-12'
order: 5
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  wer: Stefan
  datum: '2026-11-12'
  notiz: im Review von PR 43 festgelegt
---
# D50 — Die Freigabe-App heißt Passierschein, das Kommandozeilenwerkzeug a38

{{ design_kopf(page) }}

**Entscheidung:**

1. Die Freigabe-App heißt „Passierschein“. Das Bundle heißt `Passierschein.app`. Die Hauptdatei ist `Passierschein.app/Contents/MacOS/Passierschein`. Ein Entwicklungsbau ist eine einfache ausführbare Datei `Passierschein`.
2. Das Kommandozeilenwerkzeug heißt weiter `a38`. Das Go-Binary darf im Bundle liegen, als `Contents/MacOS/a38`.
3. `a38 serve` sucht die App in dieser Reihenfolge: Schalter `--ui-app`, Schlüssel `paths.ui_app`, dann (nach Auflösung der symbolischen Links von `a38`) das Geschwister `Passierschein` im eigenen Bundle, dann `Passierschein.app` oder `Passierschein` neben `a38`, dann `/Applications/Passierschein.app` und `~/Applications/Passierschein.app`. Der erste vorhandene Treffer gilt. Die Prüfung der Datei gilt für jeden Treffer.

**Begründung:** Der Mensch sieht einen Namen im Dock, im Dialog von Touch ID und im Finder. Der Name „Passierschein“ passt zum Produkt. `a38` bleibt kurz für die Kommandozeile und für die Agents.

**Folge:** Das Release kann die Kommandozeile im Bundle ausliefern und per symbolischem Link in den `PATH` legen ({{ ref("D42") }}, {{ ref("D46") }}). Die Freigabe-App aus {{ ref("D45") }} trägt den neuen Namen. Das Linux-Binary bleibt das einzelne Binary `a38` ohne App. Die Signatur der App bleibt offen ({{ ref("G5") }}).

**Quelle:** Entscheidung von Stefan im Review von PR 43 am 2026-11-12.
