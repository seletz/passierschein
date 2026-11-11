---
render_macros: true
id: 'UC9'
titel: 'Ein Agent lernt die Nutzung über den Skill'
bereich: 'Oberflächen'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: aktiv
stufe: 1
---
# UC9 — Ein Agent lernt die Nutzung über den Skill

{{ design_kopf(page) }}

**Akteure:** Agent, Mensch.

**Auslöser:** Ein Agent soll `a38` nutzen.

**Ablauf:**

1. Der Mensch installiert den Skill mit `a38 llm --skill`.
2. Der Agent liest den Skill.
3. Der Skill erklärt: Zugangsdaten gebündelt anfragen, Referenzen nennen, nach einer Ablehnung stoppen, einen Wert nie in eine Datei, einen Commit oder eine Ausgabe schreiben.

**Ergebnis:** Der Agent fragt gebündelt an. Der Skill ist eine Hilfe. Der Daemon erzwingt die Bündelung auch ohne den Skill ({{ ref("D5") }}).
