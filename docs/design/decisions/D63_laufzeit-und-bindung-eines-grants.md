---
render_macros: true
id: 'D63'
titel: 'Laufzeit und Bindung eines Grants folgen der Umgebung des Vaults'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 11
headless: true
status: gueltig
use_cases: [UC15]
beleg:
  wer: Stefan
  datum: '2026-11-15'
  quelle: ['#94']
---
# D63 — Laufzeit und Bindung eines Grants folgen der Umgebung des Vaults

{{ design_kopf(page) }}

**Entscheidung:** Ein Grant gilt für eine Sitzung und für genau die Zugangsdaten, die der Mensch angehakt hat. Laufzeit und Bindung folgen der Umgebung der Vaults. Die Entscheidung ersetzt {{ ref("D32") }}. Sie übernimmt daraus den Umfang des Grants und die Regel für die Laufzeit aus der Freigabe. {{ ref("D33") }} gilt weiter.

**Laufzeit je Umgebung:**

| Umgebung | Standard | Bedeutung |
|---|---|---|
| dev | `4h` | wie bisher |
| test | `30m` | kurz; Rückmeldung aus dem Praxistest |
| prod | `0s` | kein Grant über das Kommando hinaus: Jedes Kommando braucht eine Freigabe |

Die Konfiguration setzt die Werte je Umgebung ({{ ref("D66") }}). Der Mensch kann in der Freigabeanfrage eine kürzere Laufzeit wählen, nie eine längere. Liefert die Freigabe eine längere Laufzeit als die Konfiguration, vergibt der Daemon keinen Grant und meldet `ui_failed`. Eine gemischte Anfrage nimmt die kürzeste Laufzeit ihrer Umgebungen.

**Laufzeit 0:** Der Daemon vergibt einen Grant für genau eine Auslieferung. Der Grant endet mit dieser Auslieferung. Das Audit-Log schreibt dafür die Ereignisse Grant und Ablauf. `grant_status` zeigt einen solchen Grant nicht.

**Bindung je Umgebung:** Die Bindung legt fest, für welche Kommandos ein Grant gilt.

| Bindung | Ein Grant gilt für | Standard für |
|---|---|---|
| `none` | die Referenzen, mit jedem Weg und jedem Kommando | dev |
| `program` | die Referenzen mit demselben Pfad des Programms und denselben Namen der Variablen; die Argumente sind frei | test |
| `command` | die Referenzen mit demselben Pfad, denselben Argumenten, demselben Arbeitsverzeichnis und denselben Namen der Variablen | prod |

**Programm ohne Aussage:** Bei einer Shell, einem Interpreter oder einem Starter sagt der Pfad nichts über das Kommando. Für diese Programme gilt `program` wie `command`. Die Liste ist fest im Code: `sh`, `bash`, `zsh`, `dash`, `fish`, `env`, `sudo`, `xargs`, `nohup`, `time`, `timeout`, `nice`, `python`, `python3`, `node`, `deno`, `bun`, `ruby`, `perl`, `osascript`. Der Vergleich nutzt den letzten Teil des Pfads.

**Grant aus dem anderen Weg:** In dev gilt ein Grant aus `request_credentials` auch für `run_with_credentials` und umgekehrt (Bindung `none`). In test und prod entsteht ein Grant nur über den exec-Weg.

**Laufzeit je Vault (Wahl von Stefan):** Die Rückmeldung aus dem Praxistest fragt nach einer Laufzeit je Vault. Empfehlung: in 0.3.0 nur je Umgebung. Die Umgebung trägt die Schutzstufe; drei Werte sind leicht zu prüfen. Eine Angabe je Vault, die nur kürzen darf, ist später ohne Bruch möglich.

**Begründung:**

- Rückmeldung aus dem Praxistest: Ein Grant von 4 Stunden ist für echte Secrets lang ([Issue 83](https://github.com/seletz/passierschein/issues/83)). Eine globale Laufzeit passt nicht zugleich zu dev und prod.
- Die Bindung an das Programm in test erlaubt eine Folge von Abfragen mit demselben Werkzeug, etwa mehrere Aufrufe von `psql`, ohne ein Popup je Aufruf ({{ ref("RI3") }}). Ein Wechsel des Programms braucht eine neue Freigabe.
- In prod sieht der Mensch jedes Kommando vor der Ausführung. Das entspricht der Lage „Agenten haben im Cluster sehr beschränkte Rechte“: Jeder Zugriff auf prod ist eine bewusste Entscheidung.
- Der kleinste Umfang ist der sichere Standard ({{ ref("D32") }}).

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91), Frage 4 und 7. Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
