---
render_macros: true
id: 'RI5'
titel: 'In den Stufen 1 bis 4 kann ein Agent a38 umgehen und angreifen'
bereich: 'Grenzen und Identität'
lauf: '1.0'
stand: '2026-11-11'
order: 2
headless: true
status: offen
wahrscheinlichkeit: mittel
auswirkung: mittel
behandlung: [akzeptieren]
beleg:
  notiz: Folge des Stufenplans; Annahme zu bestätigen durch Stefan
  annahme: true
register:
  owner: Stefan
---
# RI5 — In den Stufen 1 bis 4 kann ein Agent a38 umgehen und angreifen

{{ design_kopf(page) }}

**Beschreibung:** Lokal läuft der Agent unter demselben Benutzer wie der Daemon. Er kann die Konfiguration ändern, das Audit-Log ändern, den Admin-Socket nutzen, die Freigabe-App ersetzen, das Token des Service Accounts lesen oder 1Password direkt ansprechen.

**Bewertung:** Ein Agent tut das nicht aus Versehen. Ein gekaperter Agent kann es tun. Der Zustand ist nicht schlechter als heute.

**Maßnahme:** Keine im MVP. Die Stufe 5 trennt die Zonen. Die Doku sagt klar, dass das MVP nicht isoliert.

**Revisit:** mit der Stufe 5.

**Quelle:** Projektnotiz, Stufe 4: lokal kein Schutz.
