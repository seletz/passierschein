---
render_macros: true
id: 'D65'
titel: 'Freigabeanfrage und Audit-Log zeigen beim exec-Weg das Kommando, nie einen Wert und nie die Ausgabe'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-15'
order: 8
headless: true
status: gueltig
use_cases: [UC15]
beleg:
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D65 — Freigabeanfrage und Audit-Log zeigen beim exec-Weg das Kommando, nie einen Wert und nie die Ausgabe

{{ design_kopf(page) }}

**Entscheidung:** Beim exec-Weg zeigt die Freigabeanfrage das Kommando, und das Audit-Log nennt es. Ein Wert und die Ausgabe des Kommandos stehen nie in der Freigabeanfrage und nie im Audit-Log ({{ ref("D34") }}).

**Freigabeanfrage:** Die Freigabeanfrage zeigt zusätzlich zu {{ ref("D27") }} und {{ ref("D4") }}:

- die Umgebung, hervorgehoben bei test und prod,
- den Pfad des Programms, die Argumente und das Arbeitsverzeichnis, ohne Kürzung (lange Argumente umbrechen),
- je Variable den Namen und das Zugangsdatum mit den Namen des Backends,
- die Art der Freigabe: Grant mit Laufzeit und Bindung (dev, test) oder „nur dieses Kommando“ (prod).

In prod gibt es keine Wahl der Laufzeit. Sonst gelten {{ ref("D31") }} und {{ ref("D45") }} unverändert.

Entwurf als Text (das echte Fenster entsteht mit der Umsetzung):

```text
┌─ Passierschein ─────────────────────────────────────────────┐
│  Kommando mit Zugangsdaten ausführen                        │
│                                                             │
│  Projekt     beispielprojekt                    (geprüft)   │
│  Sitzung     beispiel-koordinator · Claude Code (Angabe)    │
│  Umgebung    ▌TEST▐                                         │
│                                                             │
│  Programm    /opt/homebrew/bin/psql                         │
│  Argumente   -h datenbank-test.beispiel.internal            │
│              -U beispiel -c "select count(*) from auftrag"  │
│  Verzeichnis ~/develop/beispielprojekt                      │
│                                                             │
│  Variable    PGPASSWORD                                     │
│   ☐ beispiel.app.k8s-test › datenbank-test › password       │
│                                                             │
│  Aufgabe (Angabe):  Zahl der Aufträge in test prüfen        │
│  Grund   (Angabe):  Migration 42 verifizieren               │
│                                                             │
│  Grant: 30 Minuten  [ 5 m | 15 m | 30 m ]                   │
│  gilt für: dieses Programm, beliebige Argumente             │
│                                                             │
│            [ Ablehnen ]        [ Mit Touch ID freigeben ]   │
└─────────────────────────────────────────────────────────────┘
```

In prod lauten die letzten Zeilen „Freigabe: nur dieses Kommando, einmal“, ohne Wahl der Laufzeit.

**Audit-Log:** Der Daemon schreibt die Ereignisse aus {{ ref("D36") }}. Beim exec-Weg tragen die Ereignisse Anfrage, Grant und Auslieferung zusätzlich:

- den Weg (`mcp` oder `exec`),
- den Pfad des Programms, die Argumente (zusammen höchstens 4 KiB, danach gekürzt mit Vermerk) und das Arbeitsverzeichnis,
- je Variable den Namen und die Referenz,
- die Zeitgrenze und die Bindung des Grants.

**Neue Ereignisart „Ausführung“:** Nach `exec_result` schreibt der Daemon ein Ereignis mit Exit-Code, Signal, Zeitablauf, Dauer, Zahl der Bytes je Strom, Kürzung und Zahl der Maskierungen. Diese Felder sind Angaben von `a38 mcp` und stehen getrennt von den geprüften Fakten ({{ ref("D4") }}). Fehlt `exec_result`, fehlt das Ereignis; die Auslieferung steht trotzdem im Log.

**Wert im Kommando:** Der Daemon prüft nach dem Lesen der Werte und vor dem Eintrag für die Auslieferung: Enthalten Argumente oder Arbeitsverzeichnis einen der Werte, liefert er nicht aus. Der Agent erhält `invalid_request`. Der Eintrag nennt den Grund, nicht den Wert ({{ ref("RI14") }}).

**Begründung:** Der Mensch gibt beim exec-Weg ein Kommando frei, nicht nur ein Zugangsdatum. Er braucht dafür das ganze Kommando. Die Auswertung „welches Kommando lief mit welchem Zugangsdatum“ braucht dieselben Felder im Audit-Log. Die Ausgabe kann Daten aus test und prod enthalten; sie gehört nicht in das Audit-Log.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91). Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
