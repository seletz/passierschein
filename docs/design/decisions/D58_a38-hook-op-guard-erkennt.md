---
render_macros: true
id: 'D58'
titel: 'a38 hook op-guard erkennt op im Bash-Befehl mit einem Shell-Parser und lässt im Zweifel durch'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-12'
order: 6
headless: true
status: gueltig
use_cases: [UC12]
beleg:
  notiz: Vorschlag aus der Entwicklung, zu bestätigen durch Stefan
  annahme: true
---
# D58 — a38 hook op-guard erkennt op im Bash-Befehl mit einem Shell-Parser und lässt im Zweifel durch

{{ design_kopf(page) }}

**Entscheidung:** Der Hook aus {{ ref("D15") }} ist der Unterbefehl `a38 hook op-guard`.

1. **Name und Form:** `a38 hook <name>` sammelt die Hooks für Agent-Werkzeuge. `op-guard` ist der erste. Er liest die Eingabe des Hooks als JSON von stdin. Er prüft nur das Werkzeug `Bash`; jedes andere Werkzeug passiert. Er antwortet mit dem Exit-Code: 0 lässt durch und schreibt nichts, 2 blockiert und schreibt die Meldung für den Agent auf stderr. Diesen Vertrag dokumentiert Claude Code für `PreToolUse`, und andere Werkzeuge mit einem Hook vor einem Befehl nutzen ihn ebenfalls. Ein Fehler im Aufruf endet mit 1, nicht mit 2, damit ein falscher Eintrag in den Einstellungen nicht jeden Befehl blockiert.
2. **Einrichtung:** `a38 hook op-guard --print-config` gibt den Block für die `settings.json` von Claude Code aus. `--scope project` nennt den Namen `a38`, `--scope user` den absoluten Pfad des laufenden Binarys. Kein Befehl ändert die Einstellungen des Menschen.
3. **Erkennung:** Ein Shell-Parser (`mvdan.cc/sh/v3/syntax`, reines Go) zerlegt den Befehl. Der Hook blockiert das Kommandowort `op`, auch als Pfad, in Anführungszeichen oder mit Backslash, hinter `sudo`, `env`, `command`, `exec`, `nohup`, `time`, `nice`, `timeout`, `xargs` und `find -exec`, in Pipelines, Listen, Subshells, Blöcken, Funktionen, Befehlsersetzung, Backticks und Prozessersetzung. Er prüft die Zeichenkette von `bash -c`, `sh -c`, `zsh -c`, `eval` und `env -S` bis zur Tiefe 3. `which op`, `command -v op`, `type op` und `brew install 1password-cli` lesen kein Secret und passieren. Ein Wort, das nur „op“ enthält, ein Argument, ein Kommentar und ein Here-Document passieren.
4. **Zwei weitere Regeln:** Der Hook blockiert auch das Lesen der Token-Datei von a38 (ein Pfad auf `/onepassword-token` als Argument oder Umleitung, außer bei `ls`, `stat`, `test`, `echo`, `printf` und `a38`) und `security find-generic-password` mit `-w` oder `-g` für den Eintrag `a38.onepassword` ({{ ref("D51") }}). Beide lesen das Token, mit dem ein Agent 1Password ohne a38 erreicht.
5. **Im Zweifel durchlassen:** Eine Eingabe, die kein JSON ist, die größer als 1 MiB ist, die keinen Befehl hat oder deren Befehl kein Parser versteht, lässt der Hook durch (Exit-Code 0). Ein Wort, das erst zur Laufzeit entsteht (`$TOOL`, `bash -c "$CMD"`), lässt er ebenfalls durch. Der Hook schreibt dabei nur den Grund, nie einen Teil der Eingabe.

**Begründung:** Der Hook ist ein Hinweis und kein Schutz ({{ ref("D15") }}). Ein Agent unter demselben Benutzer erreicht ein Secret auf vielen anderen Wegen ({{ ref("RI5") }}). Der Rest von a38 schließt im Zweifel. Der Hook öffnet im Zweifel, weil er keine Grenze ist: Ein Fehler im Hook darf die normale Arbeit des Agents nicht anhalten. Ein Parser erkennt Pipelines, Anführungszeichen und Befehlsersetzung zuverlässiger als ein Suchmuster. Er meldet weniger falsche Treffer, etwa bei `git commit -m "fix op"`. Die Meldung nennt nur den Weg über `request_credentials` und keinen Umweg ({{ ref("D35") }}, {{ ref("RI8") }}). Der Vertrag über Exit-Codes passt zu mehr als einem Agent-Werkzeug.

**Quelle:** [Issue 78](https://github.com/seletz/passierschein/issues/78); [Hooks reference](https://code.claude.com/docs/en/hooks) und [Hooks guide](https://code.claude.com/docs/en/hooks-guide) von Claude Code, gelesen am 2026-11-12. Annahme der Entwicklung; Stefan bestätigt sie im Praxistest.
