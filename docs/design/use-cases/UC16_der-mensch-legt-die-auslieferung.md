---
render_macros: true
id: 'UC16'
titel: 'Der Mensch legt die Auslieferung je Umgebung fest und schaltet den Hook für alle Projekte ein'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 2
headless: true
status: aktiv
stufe: 4
---
# UC16 — Der Mensch legt die Auslieferung je Umgebung fest und schaltet den Hook für alle Projekte ein

{{ design_kopf(page) }}

**Akteure:** Mensch.

**Auslöser:** Der Mensch will `a38` statt `op` für die Agenten aller Projekte einsetzen, auch für test und prod.

**Ablauf:**

1. Der Mensch trägt die Vaults aus test und prod mit ihrer Umgebung in die Konfiguration ein ({{ ref("D47") }}) und erlaubt sie den Projekten ({{ ref("D28") }}).
2. Der Mensch prüft die Werte unter `environments` ({{ ref("D66") }}). Für prod setzt er `delivery: [exec]`, wenn Agenten prod brauchen.
3. Der Mensch prüft die Datei mit `a38 config validate` und startet den Daemon neu ({{ ref("D49") }}).
4. `a38 doctor` zeigt je Umgebung Weg, Laufzeit und Bindung.
5. Der Mensch richtet den Hook im Benutzerbereich ein: `a38 hook op-guard --print-config --scope user`, wahlweise mit `--strict` ({{ ref("D67") }}).
6. Ein Agent ruft `op` auf. Der Hook blockiert und nennt `run_with_credentials`. Der Agent nimmt den exec-Weg ({{ ref("UC15") }}).

**Ergebnis:** Die Agenten erreichen dev, test und prod nur über `a38`, solange sie dem Hook folgen. Der Hook ist ein Hinweis und kein Schutz ({{ ref("D15") }}).
