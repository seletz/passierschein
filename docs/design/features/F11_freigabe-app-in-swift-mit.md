---
render_macros: true
id: 'F11'
titel: 'Freigabe-App in Swift mit Touch ID'
bereich: 'Freigabe'
lauf: '2.0'
stand: '2026-11-12'
order: 2
headless: true
status: aktiv
stufe: 2
umsetzung: teilweise
implementiert: [D4, D6, D7, D31, D45]
use_cases: [UC1, UC3, UC4]
---
# F11 — Freigabe-App in Swift mit Touch ID

{{ design_kopf(page) }}

Die App liest eine Freigabeanfrage von stdin, zeigt das Fenster, fragt Touch ID ab und gibt die Entscheidung auf stdout aus. Sie zeigt geprüfte Fakten und Angaben des Agents getrennt. Sie markiert eine Abweichung.

Der Spike liefert einen Vorschlag für das Protokoll: eine Nachricht `approval_request` hinein, eine Zeile `approval_decision` heraus, mit den Ergebnissen `approved`, `denied`, `timeout` und `error`.


Setzt um: {{ ref("D4") }}, {{ ref("D6") }}, {{ ref("D7") }}, {{ ref("D31") }}, {{ ref("D45") }}.
