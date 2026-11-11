---
render_macros: true
id: 'D48'
titel: 'Bei einer Störung des Daemons ist die Auslieferung global gesperrt'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 9
headless: true
status: gueltig
use_cases: [UC14]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: von Stefan im Review am 2026-11-11 festgelegt
---
# D48 — Bei einer Störung des Daemons ist die Auslieferung global gesperrt

{{ design_kopf(page) }}

**Entscheidung:** Erkennt der Daemon eine Störung an sich selbst, sperrt er die Auslieferung global. Solange die Sperre besteht, liefert der Daemon keinen Wert aus und vergibt keinen Grant, für keine Sitzung und kein Projekt. Jede Anfrage endet mit dem Code `daemon_faulted`. Die Sperre endet nie von selbst. Nur der Mensch hebt sie auf. Der Daemon zeigt dem Menschen ein gut sichtbares Popup mit der Störung.

**Geltung:** Die Regel gilt für jede Störung, die der Daemon heute oder später erkennt, auch durch Selbstprüfungen. Heute ist das der Fehler beim Schreiben des Audit-Logs ({{ ref("D36") }}).

**Abgrenzung:** Die fehlerhafte Konfiguration eines einzelnen Projekts ist keine Störung des Daemons. Sie sperrt nur dieses Projekt ({{ ref("D28") }}). Ein gesperrtes oder nicht erreichbares Backend ist ein Fehler der Anfrage und keine Störung ({{ ref("D35") }}).

**Einzelheiten:** Der Daemon zeigt je Störung ein Popup und nicht eines je Anfrage. Widerruf, die Liste der Grants und `a38 doctor` arbeiten während der Sperre weiter. Der Mensch hebt die Sperre mit einem Admin-Befehl über den Admin-Socket auf oder startet den Daemon neu. Der Daemon hebt die Sperre nur auf, wenn die Prüfung der Ursache besteht; sonst bleibt die Sperre, und der Befehl nennt den Befund. Verschwindet die Ursache von selbst, bleibt die Sperre trotzdem bestehen. Der Daemon schreibt Beginn und Ende der Sperre in das Audit-Log, sobald das Schreiben möglich ist.

**Begründung:** Ein Daemon in einem unklaren Zustand darf keine Zugangsdaten ausgeben. Der Mensch muss die Störung sofort sehen. Der Dienst verhält sich vorhersehbar: Er wechselt nicht ohne Zutun zwischen gesperrt und offen (Stefan, 2026-11-11).
