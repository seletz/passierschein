---
render_macros: true
id: 'F22'
titel: 'Auslieferungsweg, Laufzeit und Bindung des Grants je Umgebung im Daemon'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 5
headless: true
status: aktiv
stufe: 4
umsetzung: zu-bauen
implementiert: [D60, D62, D63, D66]
use_cases: [UC15, UC16]
---
# F22 — Auslieferungsweg, Laufzeit und Bindung des Grants je Umgebung im Daemon

{{ design_kopf(page) }}

Der Daemon wendet je Umgebung den Weg der Auslieferung, die Laufzeit und die Bindung des Grants an. Er lädt die Abschnitte `environments` und `exec` der Konfiguration mit ihren festen Regeln. Er antwortet auf `request_credentials` für test und prod mit `exec_required`, vergibt Grants mit Bindung und Grants für genau eine Auslieferung, prüft das Arbeitsverzeichnis und lehnt ein Kommando ab, das einen Wert enthält. `a38 doctor` und `a38 tui` zeigen die Werte je Umgebung.

Setzt auf: Policy ({{ ref("F3") }}), Grants ({{ ref("F5") }}), Konfiguration ({{ ref("F15") }}).

Setzt um: {{ ref("D60") }}, {{ ref("D62") }}, {{ ref("D63") }}, {{ ref("D66") }}.
