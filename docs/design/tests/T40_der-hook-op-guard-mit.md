---
render_macros: true
id: 'T40'
titel: 'Der Hook op-guard mit --strict blockiert auch im Zweifel, wenn der Befehl op enthält'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F24, D67, UC16]
---
# T40 — Der Hook op-guard mit --strict blockiert auch im Zweifel, wenn der Befehl op enthält

{{ design_kopf(page) }}

**Form:** Go-Test wie {{ ref("T29") }}, tabellengesteuert, mit und ohne `--strict`.

**Schritte:**

1. `op read …`.
2. Ein Befehl, den der Parser nicht versteht und der `op` als Wort enthält.
3. `TOOL=op; $TOOL read …`.
4. `X=1; $X --version` (kein `op`).
5. Eine Eingabe, die kein JSON ist.

**Erwartung:**

1. Mit und ohne Schalter Exit-Code 2. Die Meldung nennt `run_with_credentials` und `request_credentials` und keinen anderen Weg.
2. und 3. Ohne Schalter Exit-Code 0, mit `--strict` Exit-Code 2.
4. und 5. Mit und ohne Schalter Exit-Code 0.

**Scheitert, wenn:** `--strict` einen Zweifelsfall mit `op` durchlässt, einen Befehl ohne `op` blockiert oder die Meldung einen Umweg nennt.
