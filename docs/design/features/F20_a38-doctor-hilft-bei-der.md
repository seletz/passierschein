---
render_macros: true
id: 'F20'
titel: 'a38 doctor hilft bei der Fehlersuche'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-12'
order: 3
headless: true
status: aktiv
stufe: 1
umsetzung: vorhanden
implementiert: [D36, D28, D35]
use_cases: [UC14]
---
# F20 — a38 doctor hilft bei der Fehlersuche

{{ design_kopf(page) }}

`a38 doctor` prüft den Zustand von `a38` und nennt zu jedem Befund die Ursache und den nächsten Schritt. Der Befehl prüft: Läuft der Daemon? Stimmen die Rechte der Sockets? Ist die Audit-Datei schreibbar, und ist Platz frei? Ist die Kette des Audit-Logs unversehrt? Ist die Konfiguration gültig, je Projekt? Ist das Backend erreichbar und entsperrt? Startet die Freigabe-App? Ist die Auslieferung wegen einer Störung gesperrt ({{ ref("D48") }})? Der Befehl gibt keinen Secret-Wert aus. Der Exit-Code ist 0 ohne Befund.


Setzt um: {{ ref("D36") }}, {{ ref("D28") }}, {{ ref("D35") }}.
