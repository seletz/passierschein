---
title: Lokaler Einsatz und VM
description: "Was a38 lokal leistet und was erst die VM leistet, und wo der exec-Weg endet."
render_macros: true
---
# Lokaler Einsatz und VM

Diese Seite erklärt die zwei Einsatzfälle von `a38` und die Grenzen des
exec-Wegs. Die verbindlichen Aussagen stehen im
[Design-Set](../design/index.md), vor allem in {{ ref("D60") }},
{{ ref("D61") }} und {{ ref("D62") }}.

!!! note "Stand"
    Die Seite beschreibt den Entwurf für `0.3.0`. Stefan hat die
    Entscheidungen noch nicht bestätigt.

## Zwei Einsatzfälle

| | lokal | VM |
|---|---|---|
| Wo der Agent läuft | auf dem Mac, unter demselben Benutzer wie der Daemon | in einer VM ohne `op`, ohne SDK und ohne Token |
| Weg zu einem Secret | `a38`, aber auch `op`, der Schlüsselbund, die Token-Datei und jeder Prozess des Benutzers | nur der Daemon über den Agent-Socket |
| Technische Grenze | keine | die VM |
| Maßstab | der direkte Aufruf von `op` | ein Secret erreicht die VM nur über `a38` |
| Meilenstein | `0.3.0` | Stufe 5 (M5) |

Lokal kann `a38` einen Agent nicht aufhalten. Ein Agent, der `a38`
umgehen will, umgeht es ({{ ref("RI5") }}). Lokal zählt deshalb ein
anderer Vergleich: Was tut der Agent heute ohne `a38`?

## Der lokale Maßstab: `op`

Agenten rufen heute `op` auf, für dev, test und prod. `op` gibt den Wert
direkt aus. Der Wert steht dann im Kontext des Modells und im Transkript.
Niemand gibt den Zugriff frei. Kein Log nennt ihn.

`a38` verbessert drei Dinge, auch ohne technische Grenze:

1. **Freigabe:** Der Mensch gibt jeden Zugriff frei und sieht, wofür der
   Agent den Wert braucht.
2. **Nachweis:** Das Audit-Log nennt jeden Zugriff und jedes Kommando.
3. **Kein Wert im Kontext:** Mit dem exec-Weg sieht der Agent den Wert im
   Normalfall nicht.

Diese Vorteile wirken nur, wenn die Agenten `a38` statt `op` nutzen. Dafür
muss `a38` alles abdecken, was die Agenten heute mit `op` tun. Deckt
`a38` nur dev ab, bleibt `op` für test und prod im Einsatz. Dann lässt
sich der Hook gegen `op` nicht für alle Projekte einschalten. Das war das
Ergebnis des Praxistests von `0.2.0`
([Issue 83](https://github.com/seletz/passierschein/issues/83)).

## Der exec-Weg

Beim exec-Weg fragt der Agent nicht nach einem Wert. Er nennt ein
Kommando und die Zugangsdaten, die das Kommando braucht. `a38 mcp` holt
die Werte beim Daemon, startet das Kommando mit den Werten in dessen
Umgebung und gibt dem Agent nur Exit-Code und Ausgabe zurück. Das
MCP-Werkzeug heißt `run_with_credentials` ({{ ref("D61") }}). Der Ablauf
steht in {{ ref("UC15") }}.

Ein Beispiel: Der Agent will in test die Zahl der Aufträge prüfen.

```json
{
  "command": "psql",
  "args": ["-h", "datenbank-test.beispiel.internal", "-U", "beispiel",
           "-c", "select count(*) from auftrag"],
  "env": {"PGPASSWORD": "op://beispiel.app.k8s-test/datenbank-test/password"},
  "task": "Zahl der Aufträge in test prüfen",
  "reason": "Migration 42 verifizieren"
}
```

Der Mensch sieht in der Freigabeanfrage das ganze Kommando. Der Agent
erhält die Ausgabe von `psql`. Das Passwort sieht er nicht.

## Weg je Umgebung

| Umgebung | MCP-Antwort | exec-Weg | Freigabe |
|---|---|---|---|
| dev | erlaubt | erlaubt, im Skill bevorzugt | Grant, Standard 4 Stunden |
| test | nie | erlaubt | Grant, Standard 30 Minuten, nur für dasselbe Programm |
| prod | nie | nur nach Einschalten | je Kommando, ohne Grant |

Die Regel „außerhalb von dev nie als MCP-Antwort“ steht im Code. Die
Konfiguration kann sie nicht lockern ({{ ref("D62") }},
{{ ref("D66") }}). Laufzeit und Bindung des Grants stehen in
{{ ref("D63") }}.

## Grenzen des exec-Wegs

Der exec-Weg schützt nicht gegen einen gekaperten Agent. Die Gründe:

- **Der Agent wählt das Kommando.** `printenv` oder
  `echo "$PGPASSWORD" | base64` bringt den Wert in die Ausgabe. Der Mensch
  sieht das Kommando in der Freigabeanfrage. Bei einem passenden Grant
  sieht er es nicht ({{ ref("RI13") }}).
- **Die Maskierung erkennt nur den exakten Wert.** `a38 mcp` ersetzt jedes
  exakte Vorkommen in der Ausgabe durch `[masked:NAME]`. Ein kodierter
  oder veränderter Wert bleibt stehen ({{ ref("D64") }}).
- **Die Umgebung eines Prozesses ist lesbar.** Lokal liest jeder Prozess
  desselben Benutzers die Umgebung des Kommandos.
- **`a38 mcp` liegt in der Zone des Agents.** Ein verändertes `a38 mcp`
  gibt den Wert direkt zurück ({{ ref("D2") }}).
- **Argumente sind öffentlich.** Ein Wert in den Argumenten steht in der
  Prozessliste. Der Daemon lehnt ein Kommando ab, das einen der Werte in
  den Argumenten enthält. Eine Expansion in einer Shell erkennt er nicht
  ({{ ref("RI14") }}).

Der exec-Weg leistet etwas anderes: Er verhindert, dass der Wert im
Normalfall in den Kontext des Modells und in das Transkript fließt. Das
ist die Ursache von {{ ref("RI1") }}. Ein ehrlicher Agent, der eine
Aufgabe erledigt, bringt den Wert so nicht mehr zum Modell-Anbieter.

## Warum keine Datei auf einem tmpfs

Das erste Design sah für Werte außerhalb von dev eine Datei mit Laufzeit
vor ({{ ref("D10") }}). Der exec-Weg ersetzt diese Datei:

- Eine Datei gibt dem Agent einen Pfad. Ein Agent liest Dateien
  routinemäßig. Der Wert landet dann doch im Kontext.
- Eine Datei braucht Löschen, einen geschützten Ort und Regeln für einen
  Absturz. macOS hat im Standard kein tmpfs.
- Lokal schützt eine Datei nicht besser als die Umgebung eines Prozesses.

## Was die VM ändert

In der VM gibt es kein `op` und kein Token. Der Daemon ist der einzige
Weg zu einem Secret. Der Vertrag des exec-Wegs bleibt gleich, weil
`a38 mcp` das Kommando dort startet, wo der Agent arbeitet.

Offen ist, ob ein Wert aus test oder prod in die VM gelangen darf
({{ ref("G35") }}). In der VM liegt die Umgebung des Kommandos in der Zone
des Agents. Eine Antwort kann eine Ausführung außerhalb dieser Zone sein.
Die lokale Lösung verbaut keine dieser Antworten.

## Der Hook gegen `op`

Der Hook `a38 hook op-guard` blockiert den Aufruf von `op` und nennt den
Weg über `a38` ({{ ref("D58") }}). Er ist ein Hinweis und kein Schutz
({{ ref("D15") }}). Mit dem exec-Weg hat jeder Aufruf von `op` einen
Ersatz. Erst dann ist der Hook für alle Projekte sinnvoll
({{ ref("D67") }}, {{ ref("UC16") }}).
