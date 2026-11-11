---
render_macros: true
id: 'T19'
titel: 'Ein Bündel über der Obergrenze wird gekürzt, und Bündel warten nacheinander'
bereich: 'Anfrage und Bündelung'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [D29, D30]
---
# T19 — Ein Bündel über der Obergrenze wird gekürzt, und Bündel warten nacheinander

{{ design_kopf(page) }}

**Form:** Go-Test mit `--ui script` und einer gefälschten Uhr. Die Obergrenze kommt aus der Konfiguration; der Test läuft mit dem Standard zehn und mit dem Wert drei.

**Schritte:** Eine Sitzung sendet zwölf Referenzen im Bündelfenster. Zwei Sitzungen senden zur gleichen Zeit je eine Anfrage.

**Erwartung:** Die Freigabeanfrage enthält zehn Einträge; zwei Einträge enden mit `bundle_too_large`. Die zwei Sitzungen erzeugen zwei Freigabeanfragen nacheinander, nie zur gleichen Zeit. Genau die Obergrenze ist erlaubt. Die ersten Einträge in der Reihenfolge des Eingangs bleiben. Ein Bündelfenster von 0 oder eine Obergrenze von 0 in der Konfiguration: Der Daemon startet nicht.


Belegt: {{ ref("D29") }}, {{ ref("D30") }}.
