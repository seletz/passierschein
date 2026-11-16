---
render_macros: true
id: 'D74'
titel: 'Freigabeanfrage, Audit-Log, a38 doctor und a38 tui nennen das Konto neben dem Vault'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-16'
order: 10
headless: true
status: gueltig
use_cases: [UC1, UC6, UC11]
beleg:
  wer: Stefan
  datum: '2026-11-16'
---
# D74 — Freigabeanfrage, Audit-Log, a38 doctor und a38 tui nennen das Konto neben dem Vault

{{ design_kopf(page) }}

**Entscheidung:** Jede Ausgabe, die einen Vault nennt, nennt auch sein Konto. Der Daemon nimmt das Konto aus der Konfiguration ({{ ref("D70") }}), nie aus der Anfrage des Agents.

| Ausgabe | Was sie zeigt |
|---|---|
| Freigabeanfrage | Je Eintrag das Konto vor dem Vault, zum Beispiel `privat · beispiel.app.k8s-dev`. Ein Bündel mit Vaults aus zwei Konten zeigt beide Konten. |
| Audit-Log | Jedes Ereignis mit einem Vault speichert den Namen des Kontos. Das Feld gehört zur Hash-Kette ({{ ref("D37") }}). Ein Ereignis aus der Zeit vor den Konten hat kein Konto. |
| `a38 tui` | Eine Spalte „Konto“ neben dem Vault, in den Listen der Freigaben und der Ereignisse. |
| `a38 doctor` | Je Konto die Quelle des Tokens, den Eintrag im Schlüsselbund oder die Datei (nur Metadaten), die Vaults des Kontos und einen Befund mit dem nächsten Schritt. |
| `a38 config vault list` | Je Vault die Umgebung und das Konto. |

**Begründung:** Der Mensch gibt einen Vault frei, nicht einen Namen. Zwei Konten können ähnliche Namen haben. Das Konto in der Freigabeanfrage zeigt ihm, aus welchem Konto der Wert kommt. Das Audit-Log muss die Herkunft auch nach einer Änderung der Konfiguration zeigen. Deshalb speichert es das Konto und leitet es nicht später aus der Konfiguration ab.

**Umsetzung:** `a38 doctor` und `a38 config vault list` setzt [Issue 88](https://github.com/seletz/passierschein/issues/88) um. Freigabeanfrage, Audit-Log und `a38 tui` brauchen ein neues Feld in der Nachricht an die Freigabe-App, in der Swift-App und im Schema des Audit-Logs. Sie folgen in einem eigenen Issue ({{ ref("F26") }}).

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88), Skizze. Vorschlag aus der Entwicklung; von Stefan am 2026-11-16 in PR 114 bestätigt.
