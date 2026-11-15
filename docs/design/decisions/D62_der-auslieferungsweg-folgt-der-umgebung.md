---
render_macros: true
id: 'D62'
titel: 'Der Auslieferungsweg folgt der Umgebung des Vaults: außerhalb von dev nur über den exec-Weg, nie als MCP-Antwort'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 10
headless: true
status: gueltig
use_cases: [UC15]
beleg:
  wer: Stefan
  datum: '2026-11-15'
  quelle: ['#94']
---
# D62 — Der Auslieferungsweg folgt der Umgebung des Vaults: außerhalb von dev nur über den exec-Weg, nie als MCP-Antwort

{{ design_kopf(page) }}

**Entscheidung:** Die Umgebung des Vaults ({{ ref("D47") }}) bestimmt den Weg der Auslieferung. Die Entscheidung gilt für den lokalen Einsatz ({{ ref("D60") }}). Sie ersetzt {{ ref("D10") }} und {{ ref("D24") }}.

| Umgebung | MCP-Antwort (`request_credentials`) | exec-Weg (`run_with_credentials`) | Freigabe |
|---|---|---|---|
| dev | erlaubt | erlaubt | Grant mit Laufzeit, wie heute |
| test | nie | erlaubt | Grant mit kurzer Laufzeit, gebunden an das Programm |
| prod | nie | erlaubt, nur nach Einschalten in der Konfiguration | Freigabe je Kommando, keine Wiederverwendung |

Laufzeit und Bindung stehen in {{ ref("D63") }}, die Schlüssel der Konfiguration in {{ ref("D66") }}.

**Feste Regel:** Ein Wert außerhalb von dev steht nie in einer MCP-Antwort. Die Konfiguration kann diese Regel nicht ändern. Fragt ein Agent mit `request_credentials` nach einem Vault aus test oder prod, erhält er den Code `exec_required` ({{ ref("D61") }}). Die Freigabeanfrage erscheint dann nicht. Ist der Weg für die Umgebung ausgeschaltet, erhält er `not_allowed`.

**Gemischte Anfrage:** Nennt eine Anfrage Vaults aus mehreren Umgebungen, gilt für die ganze Anfrage die strengste Umgebung: prod vor test vor dev.

**prod (Vorschlag, Wahl von Stefan):** Der exec-Weg für prod ist im Standard ausgeschaltet. Der Mensch schaltet ihn in der Konfiguration ein. Danach braucht jedes Kommando eine eigene Freigabe. Die Alternative ist „prod gar nicht“: `a38` lehnt prod immer mit `not_allowed` ab. Empfehlung: einschaltbar mit Freigabe je Kommando. Begründung: Agenten rufen `op` heute auch für prod auf. Ohne Weg für prod bleibt `op` dafür im Einsatz, ohne Freigabe und ohne Eintrag ({{ ref("D60") }}).

**Verhältnis zur Datei auf einem tmpfs:** Der exec-Weg ersetzt die Datei aus {{ ref("D10") }}. `a38` schreibt keinen Wert in eine Datei. Die Lücke {{ ref("G11") }} ist damit geschlossen. Gründe:

- Eine Datei gibt dem Agent einen Pfad. Ein Agent liest eine Datei routinemäßig mit `cat` oder einem Lese-Werkzeug. Der Wert steht dann im Kontext. Beim exec-Weg gibt es nichts zu lesen.
- Eine Datei braucht Löschen nach der Laufzeit, einen Ort ohne Sicherung und ohne Index (macOS hat kein tmpfs im Standard) und Regeln für Absturz und Neustart. Der exec-Weg braucht nichts davon: Der Wert lebt so lange wie das Kommando.
- Lokal schützt eine Datei den Wert nicht besser als die Umgebung eines Prozesses. Beide liest jeder Prozess desselben Benutzers. Das war der Grund für {{ ref("D24") }}.
- Den Weg in der VM entscheidet {{ ref("G35") }}. Braucht die VM eine Datei, entsteht dafür eine neue Entscheidung.

**Begründung:** {{ ref("D24") }} lehnte test und prod ab, weil lokal kein Weg den Wert schützt. Der Maßstab ist aber der direkte Aufruf von `op` ({{ ref("D60") }}). `op` liefert test und prod heute ohne Freigabe, ohne Eintrag und mit dem Wert in der Ausgabe. Der exec-Weg ist für test und prod besser als `op`: Freigabe, Eintrag im Audit-Log und kein Wert im Kontext im Normalfall. Eine Ablehnung verdrängt `op` nicht, sie hält es im Einsatz. Für dev bleibt die MCP-Antwort erlaubt ({{ ref("D9") }}); der Skill empfiehlt dort trotzdem den exec-Weg ({{ ref("D68") }}).

**Grenze:** Der exec-Weg schützt nicht gegen einen gekaperten Agent. Der Agent wählt das Kommando, und die Umgebung eines Prozesses desselben Benutzers ist lesbar ({{ ref("RI13") }}, {{ ref("RI5") }}). Der exec-Weg verhindert, dass der Wert im Normalfall in den Kontext des Modells und in das Transkript fließt. Das ist die Ursache von {{ ref("RI1") }}.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91), Frage 2, 3 und 5; Stefan am 2026-11-15: „Stoße 0.3.0 an. Fokus auf exec weg wie du vorgeschlagen hast, lokale anwendung.“ Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
