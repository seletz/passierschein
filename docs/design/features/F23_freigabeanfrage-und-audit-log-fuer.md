---
render_macros: true
id: 'F23'
titel: 'Freigabeanfrage und Audit-Log für den exec-Weg'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-15'
order: 3
headless: true
status: aktiv
stufe: 4
umsetzung: zu-bauen
implementiert: [D65]
use_cases: [UC15]
---
# F23 — Freigabeanfrage und Audit-Log für den exec-Weg

{{ design_kopf(page) }}

Die Freigabeanfrage zeigt beim exec-Weg Umgebung, Programm, Argumente, Arbeitsverzeichnis, Variablen und die Art der Freigabe. In prod gibt es keine Wahl der Laufzeit. Das Audit-Log nennt Weg und Kommando und hat die neue Ereignisart „Ausführung“. `a38 audit` und `a38 tui` zeigen die neuen Felder.

Setzt auf: Freigabe-App ({{ ref("F11") }}), Freigabe-Schnittstelle ({{ ref("F4") }}), Audit-Log ({{ ref("F7") }}).

Setzt um: {{ ref("D65") }}.
