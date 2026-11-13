---
title: Freigaben auswerten
description: "Mit a38 tui und a38 audit stats Freigaben und Auslieferungen auswerten: Start, Ansichten, Tasten, Kennzahlen und was du daraus machst."
render_macros: true
tags:
  - Thema/Betrieb
---
# Freigaben auswerten

Diese Anleitung zeigt, wie du mit `a38 tui` auswertest, welche Agents welche Zugangsdaten erhalten haben ({{ ref("UC6") }}).
Die Ansicht liest das Audit-Log nur lesend.
Sie schreibt nie und spricht nicht mit dem Daemon.
Der Daemon muss nicht laufen.
Das Audit-Log enthält keinen Secret-Wert, also zeigt auch die Ansicht keinen.

`a38 tui` gibt es nur unter macOS, denn das Audit-Log liegt beim Daemon auf dem Mac ({{ ref("D57") }}).
Für Skripte und unter Linux gibt `a38 audit stats` dieselben Zahlen aus.

!!! note "Entwurf"
    Die Oberfläche, die Kennzahlen und ihre Definitionen sind ein Vorschlag aus der Entwicklung.
    Stefan nimmt sie ab ({{ ref("D57") }}, {{ ref("G12") }}).

## Starten

Starte die Ansicht im Terminal.

```console
$ a38 tui
```

Die Ansicht liest die Audit-Datei aus `paths.audit_file` der Konfiguration.
`--config <datei>` wählt eine andere Konfiguration, `--file <audit-datei>` eine andere Datei.
`--range 30d` startet mit 30 Tagen statt 7.
`--refresh 30s` liest das Audit-Log alle 30 Sekunden neu.
Ohne `--refresh` liest die Ansicht nur mit der Taste `R` neu.

Zum Ausprobieren ohne echtes Audit-Log schreibt ein Werkzeug der Entwicklung ein synthetisches Log mit 30 Tagen.
Es schreibt nur eine neue Datei und nie in eine bestehende.

```console
$ go run ./internal/auditdemo/gen -out /tmp/a38-tui-demo/audit.db
$ a38 tui --file /tmp/a38-tui-demo/audit.db --range 30d
```

## Ansichten

Die Ansicht hat drei Teile.
Die Ziffern `1`, `2` und `3` wechseln zwischen ihnen.

| Ansicht | Inhalt |
|---|---|
| 1 Übersicht | Kennzahlen des Zeitraums, Popups je Tag als Balken, die aktivsten Sitzungen und Projekte, die am häufigsten angefragten Zugangsdaten (nur Referenzen). |
| 2 Ereignisse | Die Einträge des Audit-Logs, die neuesten zuerst. Die Eingabetaste öffnet einen Eintrag mit allen Feldern und seiner Stelle in der Hash-Kette. |
| 3 Sitzungen | Jede Sitzung mit Anfragen, Popups, Grants, Auslieferungen und Ablehnungen. Die Eingabetaste zeigt die Grants der Sitzung und ihre Zeitleiste. |

Alle Ansichten teilen denselben Rahmen:

- **Kopfzeile:** links `a38 tui` in einem Farbverlauf, in der Mitte gedämpft die Zeit des letzten Lesens (`updated 17:30:00`), rechts die Ansichten als Tabs. Die aktive Ansicht steht in eckigen Klammern auf der Akzentfarbe (`[2 events]`). Ein offenes Detail hängt hinter einem Pfeil an: `[2 events] › entry 1040`.
- **Trennlinien** über und unter dem Inhalt.
- **Kontextzeile:** die erste Zeile des Inhalts. Sie nennt die Audit-Datei, die Zahl der Einträge, den Zustand der Hash-Kette, den Zeitraum, den Filter und die Suche. Bei der Eingabe von Suche oder Filter steht hier die Eingabezeile, nach einer Taste wie `c` eine Meldung.
- **Fußzeile:** die wichtigsten Tasten der Ansicht, dann die globalen Tasten (`enter detail • / search • f filter • t range • 1/2/3 view • ? help • q quit`). Passt ein Hinweis nicht mehr, steht dort `…`.
- **Hilfe:** `?` legt eine Tafel mit allen Tasten der Ansicht und den globalen Tasten in einem abgerundeten Rahmen über den Inhalt. `esc` oder `?` schließt sie.
- **Auswahl:** Die gewählte Zeile einer Liste trägt die Marke `▸` und ist hinterlegt.

Beim Start prüft die Ansicht die Hash-Kette im Hintergrund.
Ist die Kette gebrochen, nennt die Kontextzeile den ersten fehlerhaften Eintrag, und die Übersicht warnt.
Die Taste `R` prüft die Kette erneut.

## Tasten

| Taste | Wirkung |
|---|---|
| `1` `2` `3` | Ansicht wechseln; schließt ein offenes Detail |
| `j` `k`, Pfeil ab und auf | bewegen oder blättern |
| `pgdown` oder Leertaste, `pgup` oder `b` | seitenweise blättern |
| `g` oder `home`, `G` oder `end` | Anfang und Ende |
| Eingabetaste | Detail eines Ereignisses, Zeitleiste einer Sitzung |
| `esc` | zurück |
| `/` | Text in den Ereignissen suchen, ohne Rücksicht auf Groß- und Kleinschreibung |
| `f` | Filter setzen |
| `c` | Filter und Suche löschen |
| `t` | Zeitraum wechseln: heute, 7 Tage, 30 Tage, alles |
| `R` | Audit-Log neu lesen und Kette prüfen |
| `?` | Hilfe |
| `q` | beenden; in der Eingabezeile ist `q` Text |
| `ctrl+c` | beenden, immer |

Eine Maus ist nicht nötig.
Die Ansicht passt in 80 mal 24 Zeichen und nutzt ein größeres Fenster.
### Farben

Die Ansicht folgt dem Hintergrund des Terminals.
Beim Start fragt sie das Terminal nach seiner Hintergrundfarbe.
Auf hellem Hintergrund dunkelt sie die Farben ab.
Bis die Antwort kommt, gilt das dunkle Theme.

| Rolle | Farbe | Wo |
|---|---|---|
| Akzent | Blau | Programmname (Verlauf von Blau über Türkis nach Violett), aktiver Tab, Überschriften, Spaltenköpfe, Tasten der Fußzeile, Rahmen der Hilfe, Marke `▸`, Balken |
| Zahl | Türkis | Hauptzahlen der Übersicht |
| Gut | Grün | Auslieferung, Freigabe, intakte Kette |
| Vorsicht | Orange | Zeitablauf, Abweichung |
| Fehler | Rot | Ablehnung, Fehler, gebrochene Kette |
| Auswahl | Hinterlegung | gewählte Zeile |

Farben sind nur eine Hilfe: Jeder Zustand steht auch als Wort da, der aktive Tab in Klammern, die Auswahl mit `▸`.
`NO_COLOR=1` schaltet die Farben ab.

### Filter

Der Filter ist eine Zeile mit Paaren `schlüssel=wert`, getrennt durch Leerzeichen.

```text
project=beispielprojekt session=beispiel-koordinator type=denial,expiry code=denied ref=datenbank-dev
```

| Schlüssel | Wählt | Wirkt auf |
|---|---|---|
| `project` | Projekt (geprüfte Tatsache), genau | Kennzahlen, Ereignisse, Sitzungen |
| `session` | Name der Sitzung (Angabe des Agents), genau | Kennzahlen, Ereignisse, Sitzungen |
| `type` | Ereignisarten, durch Komma getrennt | Ereignisse |
| `code` | Fehlercode, genau | Ereignisse |
| `ref` | Teil einer Referenz, zum Beispiel `datenbank-dev` | Ereignisse |

Die Suche mit `/` wirkt nur auf die Ereignisse.
Der Zeitraum mit `t` wirkt auf alle drei Ansichten.

## Was die Kennzahlen bedeuten

Das Audit-Log hat kein Feld für ein Popup oder eine Abweichung.
Das Paket `internal/auditstats` leitet beides aus den Ereignissen ab ({{ ref("D57") }}).
Ein Tag ist ein Kalendertag in der lokalen Zeit.

| Kennzahl | Berechnung aus den Ereignissen |
|---|---|
| Popups | Ereignisse „bundle“ mit einer Kennung der Freigabeanfrage, denen „decision“, „denial“ mit `denied`, „expiry“ mit `timeout` oder „error“ mit derselben Kennung folgt. Ein Bündel ohne ein solches Ereignis hatte kein Popup: Ein Grant deckte jeden Eintrag ab. |
| je Tag | Popups geteilt durch die Zahl der Tage des Zeitraums. Der Balken zeigt die Popups jedes Tages. |
| approved | Popups mit „decision“, auch die Teilfreigaben. |
| partial | Popups mit „decision“ und „denial“: Der Mensch hat einen Teil abgehakt. |
| denied | Popups mit „denial“ ohne „decision“. |
| Anteil der Ablehnungen | denied geteilt durch Popups. |
| timed out | Popups mit „expiry“ und `timeout`. Daneben stehen Fehler der Freigabe-App (`ui_failed`) und Bündel, deren Sitzung vor der Entscheidung endete; diese zählen nicht als Popup. |
| deviations | Popups einer Sitzung, die zur Zeit des Bündels einen lebenden Grant hatte. Der Agent wollte mehr, als der Grant abdeckt ({{ ref("D8") }}). Ein Grant lebt von „grant“ bis zum Ende seiner Laufzeit oder bis „expiry“ oder „revocation“ mit seiner Kennung. |
| size | Zahl der Einträge, die der Mensch im Popup sah: Referenzen der Ereignisse des Ausgangs. Die Übersicht zeigt den Median und das Maximum ({{ ref("G18") }}). |
| deliveries | Ereignisse „delivery“. Eine Auslieferung zählt als „nach einem Popup“, wenn ihr Grant nach der letzten Anfrage der Sitzung entstand. Jede andere lief unter einem Grant ohne Popup. |
| denied early | Ereignisse „denial“ ohne Kennung einer Freigabeanfrage: Die Policy, ein unbekanntes Item oder ein volles Bündel lehnte ab, bevor ein Popup entstand. |
| revocations, errors, locks | Ereignisse „revocation“; Ereignisse „error“ ohne die Marken der Sperre, mit ihrem Code; Beginn und Ende der globalen Sperre (`fault_begin`, `fault_end`). |
| Rangfolgen | Anfragen, Popups und Auslieferungen je Name der Sitzung, je Projekt und je Referenz. |

## Was du mit den Zahlen machst

Die Zahlen helfen, das Ausgangsproblem zu erkennen: Der Mensch bestätigt ohne zu lesen ({{ ref("RI3") }}).
Ab welchem Wert eine Zahl ein Warnsignal ist, ist offen ({{ ref("G12") }}).
Die folgenden Schritte sind Vorschläge.

- **Viele Popups je Tag:** Prüfe in der Übersicht die aktivsten Sitzungen und Zugangsdaten. Ein Zugangsdatum, das jede Sitzung braucht, spricht für eine längere Laufzeit des Grants (`grant.lifetime`) oder ein größeres Bündelfenster (`bundle.window`).
- **Anteil der Ablehnungen nahe null:** Das kann heißen, dass du ohne Lesen bestätigst. Öffne einige Popups der letzten Tage in der Ansicht „Ereignisse“ (Filter `type=decision`) und prüfe, ob du jede Freigabe wieder so geben würdest.
- **Viele Ablehnungen:** Ein Agent fragt Zugangsdaten an, die er nicht braucht. Filtere nach der Sitzung, lies Aufgabe und Begründung im Detail und passe die Anleitung des Agents oder die Vaults des Projekts an.
- **Viele Abweichungen:** Der erste Grant einer Sitzung ist zu klein. Der Agent fordert nach und nach weitere Einträge an. Prüfe, ob die Anleitung des Agents alle nötigen Zugangsdaten auf einmal anfordert.
- **Große Bündel:** Ein langes Popup liest du schlechter ({{ ref("G18") }}, {{ ref("RI12") }}). Prüfe die Obergrenze `bundle.max_entries`.
- **Viele frühe Ablehnungen:** Ein Agent fragt außerhalb der Policy an. Der Filter `code=not_allowed` zeigt die Referenzen. Ergänze die Policy oder korrigiere den Agent.
- **Fehler und Sperren:** Ein Eintrag `fault_begin` zeigt eine Störung des Daemons. Die Anleitung [Störung finden und beheben](24-stoerung-finden-und-beheben.md) hilft.
- **Gebrochene Kette:** Jemand oder etwas hat die Datei geändert ({{ ref("D37") }}). Das Detail des ersten fehlerhaften Eintrags zeigt den Befund. `a38 audit verify` zeigt dasselbe.

## Zahlen für Skripte

`a38 audit stats` gibt dieselben Kennzahlen aus, als Text oder mit `--json` als ein Objekt.

```console
$ a38 audit stats --range 30d --json
```

Die Schalter stehen in der [Referenz der Befehle](../reference/80-befehle-fuer-den-menschen.md#a38-audit).
