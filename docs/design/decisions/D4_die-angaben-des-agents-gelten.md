---
render_macros: true
id: 'D4'
titel: 'Die Angaben des Agents gelten als unbestätigt und stehen getrennt von den geprüften Fakten'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: gueltig
use_cases: [UC1, UC3]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: Projektnotiz vom 2026-11-11
---
# D4 — Die Angaben des Agents gelten als unbestätigt und stehen getrennt von den geprüften Fakten

{{ design_kopf(page) }}

**Entscheidung:** Name, Aufgabe und Begründung des Agents gelten als unbestätigt. Die Freigabeanfrage zeigt oben die Fakten, die der Daemon geprüft hat: Herkunft der Verbindung, Vault und Items. Sie zeigt unten, sichtbar getrennt, die Angaben des Agents.

**Begründung:** Ein gekaperter Agent liefert eine plausible Begründung. Der Mensch muss erkennen, was geprüft ist ({{ ref("RI2") }}).

**Quelle:** Projektnotiz, Ablauf; kritische Prüfung, Schwachstelle zur selbst behaupteten Identität.
