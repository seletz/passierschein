---
render_macros: true
id: 'RI1'
titel: 'Ein ausgeliefertes dev-Secret steht im Kontext des Modells und im Transkript'
bereich: 'Grants und Auslieferung'
lauf: '1.0'
stand: '2026-11-11'
order: 1
headless: true
status: akzeptiert
wahrscheinlichkeit: hoch
auswirkung: niedrig
behandlung: [mindern, akzeptieren]
beleg:
  wer: Stefan
  datum: '2026-11-11'
  notiz: 'Projektnotiz, Secrets-Modell: Leck in dev akzeptiert, weil rotierbar'
register:
  owner: Stefan
---
# RI1 — Ein ausgeliefertes dev-Secret steht im Kontext des Modells und im Transkript

{{ design_kopf(page) }}

**Beschreibung:** Der Wert einer MCP-Antwort geht an den Modell-Anbieter und steht im Transkript, in Zusammenfassungen und in Sitzungsarchiven. Ein gekaperter Agent kann den Wert nach außen senden.

**Bewertung:** Die Wahrscheinlichkeit ist hoch, weil es bei jeder Auslieferung geschieht. Die Auswirkung ist niedrig, weil dev-Vaults nur Zugangsdaten der Entwicklungsumgebung enthalten.

**Maßnahme:** Nur dev-Vaults ({{ ref("D24") }}). Rotation bei Verdacht ({{ ref("G13") }}). Außerhalb von dev nie als MCP-Antwort ({{ ref("D10") }}).

**Revisit:** wenn ein dev-Vault Zugangsdaten enthält, die mehr als dev erreichen.

**Quelle:** kritische Prüfung, kritische Schwachstelle zum Secret im Kontext.
