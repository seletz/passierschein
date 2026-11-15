---
render_macros: true
id: 'T41'
titel: 'Der Skill nennt run_with_credentials, exec_required und die Grenzen des exec-Wegs'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 4
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F24, D68]
---
# T41 — Der Skill nennt run_with_credentials, exec_required und die Grenzen des exec-Wegs

{{ design_kopf(page) }}

**Form:** Go-Test über die Ausgabe von `a38 llm --skill`, wie {{ ref("T20") }}.

**Erwartung:** Der Skill nennt `run_with_credentials` mit allen Parametern, `exec_required` und `exec_failed` mit ihrer Regel, die Regel „never put a value into the arguments“, die Grenze der Maskierung und die Regel für Grants je Umgebung. Der Satz „delivers only credentials of dev vaults“ fehlt. Jeder Code aus `protocol.Codes()` steht im Skill.

**Scheitert, wenn:** einer dieser Teile fehlt oder ein Code ohne Regel bleibt.
