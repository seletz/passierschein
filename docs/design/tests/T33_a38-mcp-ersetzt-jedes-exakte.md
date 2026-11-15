---
render_macros: true
id: 'T33'
titel: 'a38 mcp ersetzt jedes exakte Vorkommen eines Werts in der Ausgabe, auch an der Schnittkante'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 9
headless: true
status: aktiv
umsetzung: zu-bauen
beweist: [F21, D64]
---
# T33 — a38 mcp ersetzt jedes exakte Vorkommen eines Werts in der Ausgabe, auch an der Schnittkante

{{ design_kopf(page) }}

**Form:** Go-Test der Maskierung in `a38 mcp`, ohne Daemon.

**Schritte:**

1. Ausgabe, die den Wert zweimal enthält, auch in stderr.
2. Zwei Werte, von denen einer im anderen steckt.
3. Ausgabe über der Grenze, bei der die Schnittkante mitten in einem Wert liegt.
4. Ausgabe mit dem Wert in Base64.

**Erwartung:**

1. Jedes Vorkommen ist `[masked:DB_PASSWORD]`; `masked` ist 3.
2. Der längere Wert ist zuerst maskiert; kein Rest eines Werts bleibt.
3. Kein Anfang und kein Ende des Werts mit mindestens 4 Bytes bleibt an der Kante.
4. Die Base64-Form bleibt stehen. Der Test hält damit die Grenze der Maskierung fest.

**Scheitert, wenn:** ein exaktes Vorkommen oder ein Teil an der Schnittkante sichtbar bleibt.
