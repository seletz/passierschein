---
render_macros: true
id: 'D67'
titel: 'Mit dem exec-Weg gilt der Hook op-guard für alle Projekte, und der Schalter --strict blockiert auch im Zweifel'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 8
headless: true
status: gueltig
use_cases: [UC16]
beleg:
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D67 — Mit dem exec-Weg gilt der Hook op-guard für alle Projekte, und der Schalter --strict blockiert auch im Zweifel

{{ design_kopf(page) }}

**Entscheidung:** Mit dem exec-Weg deckt `a38` die Aufrufe von `op` für dev, test und prod ab ({{ ref("D60") }}). Der Hook `a38 hook op-guard` ({{ ref("D58") }}) ist damit für alle Projekte gedacht. {{ ref("D15") }} und {{ ref("D58") }} gelten weiter; diese Entscheidung ergänzt sie.

1. **Einrichtung für alle Projekte:** Die Anleitung empfiehlt `a38 hook op-guard --print-config --scope user`. Kein Befehl ändert die Einstellungen des Menschen ({{ ref("D58") }}).
2. **Meldung:** Die Meldung beim Blockieren nennt beide Wege von `a38` und keinen Umweg:

    ```text
    Blocked by a38: do not run the 1Password CLI "op".
    To run a command with a credential, use the MCP tool
    run_with_credentials of the server a38. For a dev credential
    value, you can also use request_credentials.
    ```

3. **Schalter `--strict`:** Ohne den Schalter lässt der Hook im Zweifel durch ({{ ref("D58") }}, Punkt 5). Mit `--strict` blockiert er in drei Fällen des Zweifels, wenn der rohe Befehlstext `op` als eigenes Wort enthält (Muster `(^|[^A-Za-z0-9_.-])op([^A-Za-z0-9_.-]|$)`):
    - der Parser versteht den Befehl nicht,
    - ein Kommandowort entsteht erst zur Laufzeit (`$TOOL`, `bash -c "$CMD"`),
    - die Schachtelung von `bash -c` und `eval` ist tiefer als 3.

    Eine Eingabe, die kein JSON ist, die größer als 1 MiB ist oder die keinen Befehl hat, lässt auch `--strict` durch. Das ist ein Fehler im Aufruf, kein Zweifel am Befehl. `--print-config --strict` gibt den Block mit dem Schalter aus.

4. **Kein Hook für das MCP-Werkzeug:** `run_with_credentials` ist kein Bash-Befehl. Der Hook prüft es nicht.

**Begründung:** Stefan, 2026-11-15: „Version 0.2.0 kann ich aber so nicht einsetzen, vor allem nicht mit hook.“ Der Hook blockiert `op`. Ohne exec-Weg hatten die Agenten danach keinen Weg für test und prod. Mit dem exec-Weg hat jeder Aufruf von `op` einen Ersatz. Erst dann ist ein Hook für alle Projekte sinnvoll. `--strict` hilft, wenn ein Agent `op` in einer Variable versteckt, etwa nach einer Blockade. Der Hook bleibt ein Hinweis und kein Schutz ({{ ref("D15") }}): Ein Agent unter demselben Benutzer umgeht ihn leicht ({{ ref("RI5") }}). Der Standard bleibt „im Zweifel durchlassen“, weil ein falscher Treffer die Arbeit des Agents anhält.

**Wahl von Stefan:** Ob `--strict` der Standard wird. Empfehlung: nein; `--strict` als Schalter, nach einem Praxistest mit echten Projekten neu bewerten.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91), Frage 6; [Issue 83](https://github.com/seletz/passierschein/issues/83). Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
