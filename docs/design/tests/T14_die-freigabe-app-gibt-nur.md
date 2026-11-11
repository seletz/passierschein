---
render_macros: true
id: 'T14'
titel: 'Die Freigabe-App gibt nur nach Touch ID frei'
bereich: 'Freigabe'
lauf: '1.0'
stand: '2026-11-11'
order: 4
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F11, D31]
---
# T14 — Die Freigabe-App gibt nur nach Touch ID frei

{{ design_kopf(page) }}

**Form:** Teil 1 automatisch mit Testschaltern der App. Teil 2 manuell nach einer Liste.

**Teil 1:** Die App erhält je eine Beispielanfrage für die Fälle erste Anfrage, großes Bündel, Abweichung und ungültiges JSON. Erwartung: Ohne Bestätigung endet jeder Fall mit `timeout`, das ungültige JSON mit `error`. Nur Exit-Code 0 mit `approved` gilt als Freigabe.

**Teil 2:** Ein Mensch prüft: Die Eingabetaste gibt nichts frei. Die Bestätigung ist erst nach der Verzögerung aktiv. Kein Häkchen ist vorab gesetzt. Touch ID nennt die App. Das Fenster schwebt unter OmniWM.

**Offen:** Die Testschalter dürfen im ausgelieferten Bundle nicht wirken.


Belegt: {{ ref("F11") }}, {{ ref("D31") }}.
