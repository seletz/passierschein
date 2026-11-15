---
render_macros: true
id: 'D61'
titel: 'a38 mcp startet ein Kommando mit Zugangsdaten in dessen Umgebung über das Werkzeug run_with_credentials'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 7
headless: true
status: gueltig
use_cases: [UC15]
beleg:
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D61 — a38 mcp startet ein Kommando mit Zugangsdaten in dessen Umgebung über das Werkzeug run_with_credentials

{{ design_kopf(page) }}

**Entscheidung:** `a38 mcp` bietet drei Werkzeuge: `request_credentials`, `grant_status` und neu `run_with_credentials`. Das neue Werkzeug ist der exec-Weg. `a38 mcp` holt die Werte beim Daemon, startet das Kommando selbst mit den Werten in dessen Umgebung und gibt dem Agent nur das Ergebnis des Kommandos zurück. Der Wert steht nie in der MCP-Antwort. Die Entscheidung ersetzt {{ ref("D41") }}.

**Wer das Kommando startet:** `a38 mcp` startet das Kommando, nicht der Daemon. Das Kommando läuft so dort, wo der Agent arbeitet: mit seinem Arbeitsverzeichnis, seinem `PATH` und seinen Werkzeugen, lokal wie später in der VM. Der Daemon startet nie ein Programm aus einer Anfrage des Agents. Er bleibt der einzige Prozess mit Zugriff auf 1Password ({{ ref("D1") }}) und enthält keinen Code zum Starten fremder Programme.

**Parameter von `run_with_credentials`:**

| Parameter | Pflicht | Bedeutung |
|---|---|---|
| `command` | ja | Name oder Pfad des Programms, zum Beispiel `psql`. Kein Text für eine Shell. |
| `args` | nein | Liste der Argumente. `a38 mcp` gibt sie unverändert weiter und setzt nie einen Wert ein. |
| `env` | ja | Abbildung von Name der Umgebungsvariable auf Referenz, zum Beispiel `PGPASSWORD` → `op://beispiel.app.k8s-test/datenbank-test/password`. Mindestens ein Eintrag, höchstens `bundle.max_entries` Einträge. |
| `cwd` | nein | Absoluter Pfad des Arbeitsverzeichnisses. Standard: das Arbeitsverzeichnis von `a38 mcp`. |
| `timeout_seconds` | nein | Zeitgrenze des Kommandos. Standard und Höchstwert nennt die Konfiguration ({{ ref("D64") }}, {{ ref("D66") }}). |
| `task` | ja | Aufgabe des Agents, eine Angabe des Agents ({{ ref("D4") }}). |
| `reason` | ja | Grund für die Zugangsdaten, eine Angabe des Agents. |

**Ablauf in `a38 mcp`:**

1. `a38 mcp` prüft die Parameter. Es sucht das Programm über den eigenen `PATH` und bildet den absoluten Pfad. Ein Pfad mit `/` gilt relativ zu `cwd`. Symbolische Links bleiben stehen.
2. `a38 mcp` sendet die neue Nachricht `request_exec` an den Agent-Socket: Referenzen, Namen der Variablen, Pfad des Programms, Argumente, Arbeitsverzeichnis, Zeitgrenze, Aufgabe und Grund.
3. Der Daemon prüft Policy, Auslieferungsweg ({{ ref("D62") }}) und Grant ({{ ref("D63") }}). Er fragt bei Bedarf den Menschen ({{ ref("D65") }}). Er antwortet mit `exec_credentials`: die Werte und die Grenzen für Zeit und Ausgabe.
4. `a38 mcp` startet genau den Pfad aus Schritt 1, ohne neue Suche. Die Umgebung ist die eigene Umgebung von `a38 mcp` plus die Variablen aus `env`. stdin ist leer ({{ ref("G37") }}). Das Kommando läuft in einer eigenen Prozessgruppe.
5. `a38 mcp` sammelt stdout und stderr, maskiert die Werte und kürzt die Ausgabe ({{ ref("D64") }}).
6. `a38 mcp` sendet `exec_result` an den Daemon: Exit-Code, Signal, Zeitablauf, Dauer, Zahl der Bytes je Strom, Kürzung und Zahl der Maskierungen. Nie die Ausgabe und nie einen Wert. Der Daemon schreibt das Ergebnis als Angabe von `a38 mcp` in das Audit-Log ({{ ref("D65") }}).
7. `a38 mcp` gibt dem Agent das Ergebnis zurück und verwirft die Werte.

**Ergebnis für den Agent:** Ein gestartetes Kommando ist immer ein Ergebnis ohne `isError`, auch bei einem Exit-Code ungleich 0 oder bei einem Zeitablauf. Das Ergebnis ist JSON:

```json
{
  "exit_code": 0,
  "signal": "",
  "timed_out": false,
  "duration_ms": 412,
  "stdout": "…",
  "stderr": "…",
  "stdout_truncated": false,
  "stderr_truncated": false,
  "masked": 0
}
```

**Fehler:** Ein Fehler kommt als Ergebnis mit `isError` und einem stabilen Code ({{ ref("D35") }}). Es gelten die Codes aus {{ ref("D35") }}. Zwei Codes kommen hinzu:

| Code | Bedeutung | Regel für den Agent |
|---|---|---|
| `exec_required` | Der Vault liefert nur über den exec-Weg. `request_credentials` gibt dafür keinen Wert ({{ ref("D62") }}). | Dieselbe Aufgabe einmal mit `run_with_credentials` stellen. |
| `exec_failed` | `a38 mcp` hat das Kommando nach der Auslieferung nicht gestartet, zum Beispiel weil die Datei nicht mehr ausführbar ist. | Stoppen und dem Menschen den Code nennen. |

Ein Programm, das es nicht gibt, ein Arbeitsverzeichnis außerhalb des Projekts und eine ungültige Abbildung `env` enden vor jeder Anfrage an den Daemon mit `invalid_request`. Der Daemon prüft das Arbeitsverzeichnis zusätzlich selbst ({{ ref("D64") }}).

**Bündelung:** Eine Anfrage `request_exec` bildet immer ein eigenes Bündel. Sie wartet nicht im Bündelfenster ({{ ref("D29") }}). Eine Freigabeanfrage zeigt so genau ein Kommando.

**Begründung:**

- Der Vorschlag kommt aus dem Praxistest: Ein Agent schlägt einen Modus wie `op run` vor ([Issue 83](https://github.com/seletz/passierschein/issues/83)). `op run` ist Agents und Menschen vertraut.
- Startet `a38 mcp` das Kommando, braucht der Daemon keine Umgebung des Agents und kein Wissen über Arbeitsverzeichnis, `PATH` oder die VM. Der Vertrag bleibt in der VM gleich ({{ ref("D60") }}).
- Ein Programm mit Argumenten statt Text für eine Shell macht das Kommando in der Freigabeanfrage lesbar. Eine Shell bleibt als Programm erlaubt (`sh` mit `-c`), dann greift die strengere Bindung ({{ ref("D63") }}).
- `a38 mcp` gehört zur Zone des Agents ({{ ref("D2") }}). Ein verändertes `a38 mcp` kann den Wert dem Agent geben. Der exec-Weg schützt deshalb nicht gegen einen gekaperten Agent. Er verhindert das routinemäßige Abfließen in Kontext und Transkript ({{ ref("RI1") }}, {{ ref("RI13") }}).
- Ein Ergebnis ohne `isError` bei einem Exit-Code ungleich 0 trennt „das Kommando ist gescheitert“ von „`a38` hat nichts geliefert“. Der Agent soll beim ersten Fall weiterarbeiten dürfen und beim zweiten stoppen.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91); Vorschlag des Koordinators vom 2026-11-15. Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
