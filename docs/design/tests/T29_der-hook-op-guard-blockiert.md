---
render_macros: true
id: 'T29'
titel: 'Der Hook op-guard blockiert den Aufruf von op und lässt alles andere durch'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-13'
order: 2
headless: true
status: aktiv
umsetzung: vorhanden
beweist: [F16, D58, UC12]
---
# T29 — Der Hook op-guard blockiert den Aufruf von op und lässt alles andere durch

{{ design_kopf(page) }}

**Form:** Go-Test. Der Test gibt dem Hook Eingaben in der Form von Claude Code auf stdin. Er führt keinen geprüften Befehl aus.

**Vorbedingung:** Der Hook `a38 hook op-guard` ist als `PreToolUse`-Hook für das Werkzeug `Bash` eingetragen.

**Schritte:**

1. Eingabe mit einem Bash-Befehl, der `op` startet: `op read op://…`, auch in einer Befehlsersetzung `$( … )`, hinter `sudo` und in `bash -c`.
2. Eingabe, die `op` nicht startet: `git commit -m "fix op"`, `which op`. Eingabe für ein anderes Werkzeug als `Bash`.
3. Eingabe, die kein JSON ist.

**Erwartung:**

1. Exit-Code 2. Die Meldung auf stderr nennt `request_credentials` und keinen anderen Weg. Der Befehl läuft nicht.
2. Exit-Code 0 ohne Ausgabe auf stdout.
3. Exit-Code 0 ohne Ausgabe auf stdout. Stderr nennt nur den Grund und keinen Teil der Eingabe.

**Scheitert, wenn:** ein `op`-Aufruf aus Schritt 1 durchkommt, eine Eingabe aus Schritt 2 oder 3 blockiert wird oder die Meldung einen Umweg nennt.

Der Praxistest in Claude Code gehört nicht zu diesem Testfall.


Belegt: {{ ref("F16") }}, {{ ref("UC12") }}, {{ ref("D58") }}.
