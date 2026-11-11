---
render_macros: true
id: 'T27'
titel: 'a38 doctor nennt zu jedem Fehlerzustand die Ursache und den nächsten Schritt'
bereich: 'Betrieb und Release'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F20, D36]
---
# T27 — a38 doctor nennt zu jedem Fehlerzustand die Ursache und den nächsten Schritt

{{ design_kopf(page) }}

**Form:** Tabellengetriebener Go-Test mit `--backend fake`.

**Fälle:** alles in Ordnung; Daemon läuft nicht; Audit-Datei nicht schreibbar; Kette des Audit-Logs beschädigt; ein Projekt mit fehlerhafter Konfiguration; Backend gesperrt; Socket-Verzeichnis mit falschen Rechten.

**Erwartung:** Der Fall ohne Fehler endet mit Exit-Code 0. Jeder andere Fall endet mit einem Exit-Code ungleich 0 und nennt den Befund, die Ursache und den nächsten Schritt. Die Ausgabe enthält keinen Secret-Wert.


Belegt: {{ ref("F20") }}, {{ ref("D36") }}.
