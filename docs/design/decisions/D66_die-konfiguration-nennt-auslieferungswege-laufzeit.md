---
render_macros: true
id: 'D66'
titel: 'Die Konfiguration nennt Auslieferungswege, Laufzeit und Bindung je Umgebung und die Grenzen des exec-Wegs'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 7
headless: true
status: gueltig
use_cases: [UC16]
beleg:
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D66 — Die Konfiguration nennt Auslieferungswege, Laufzeit und Bindung je Umgebung und die Grenzen des exec-Wegs

{{ design_kopf(page) }}

**Entscheidung:** Die Konfiguration ({{ ref("D49") }}) erhält zwei neue Abschnitte: `environments` und `exec`. Die Versionsnummer bleibt `1`; jeder neue Schlüssel hat einen Standard.

```yaml
version: 1
vaults:
  - name: beispiel.app.k8s-dev
    environment: dev
  - name: beispiel.app.k8s-test
    environment: test
  - name: beispiel.app.k8s-prod
    environment: prod
environments:
  dev:
    delivery: [mcp, exec]   # Standard
    grant_lifetime: 4h      # Standard
    binding: none           # Standard
  test:
    delivery: [exec]        # Standard
    grant_lifetime: 30m     # Standard
    binding: program        # Standard
  prod:
    delivery: []            # Standard: ausgeschaltet
    grant_lifetime: 0s      # fest
    binding: command        # fest
exec:
  default_timeout: 2m
  max_timeout: 10m
  output_limit: 64KiB       # je Strom
```

**Regeln beim Laden:** Ein Verstoß bricht den Start ab ({{ ref("D49") }}).

| Schlüssel | Erlaubte Werte |
|---|---|
| `environments.<env>.delivery` | Teilmenge von `mcp` und `exec`. `mcp` nur bei dev ({{ ref("D62") }}). |
| `environments.<env>.grant_lifetime` | Dauer von `0s` bis `24h`. Bei prod nur `0s`. |
| `environments.<env>.binding` | `none`, `program`, `command`. Bei test nicht `none`. Bei prod nur `command`. |
| `exec.default_timeout` | Dauer größer als 0, höchstens `exec.max_timeout` |
| `exec.max_timeout` | Dauer größer als 0, höchstens `1h` |
| `exec.output_limit` | Größe von `1KiB` bis `1MiB`, mit Einheit `KiB` oder `MiB` |

Eine andere Umgebung als dev, test und prod unter `environments` ist ein unbekannter Schlüssel.

**Übergang von `grant.lifetime`:** Der bisherige Schlüssel `grant.lifetime` ({{ ref("D32") }}) bleibt gültig. Er setzt die Laufzeit von dev. Stehen `grant.lifetime` und `environments.dev.grant_lifetime` zusammen in der Datei, ist das ein Fehler. `a38 config validate` und `a38 doctor` nennen `grant.lifetime` als veraltet und den neuen Schlüssel. `a38 config` ({{ ref("D55") }}) kennt die neuen Schlüssel.

**Anzeige:** `a38 doctor` und `a38 tui` zeigen je Umgebung Weg, Laufzeit und Bindung.

**Begründung:** Die festen Regeln für prod und für `mcp` außerhalb von dev stehen im Code und in der Prüfung der Konfiguration. Eine Konfiguration kann sie nicht lockern. Der Mensch kann prod einschalten (`delivery: [exec]`) und dev und test enger fassen. Der Standard schaltet prod aus: Eine fehlende Angabe darf nie zu einer Auslieferung aus prod führen; das folgt dem Gedanken aus {{ ref("D47") }}.

**Wahl von Stefan:** Ob prod im Standard ausgeschaltet bleibt ({{ ref("D62") }}), und ob prod eine Laufzeit größer als 0 erlauben soll. Empfehlung: ausgeschaltet im Standard, Laufzeit fest 0.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91), Frage 7. Die Konten aus [Issue 88](https://github.com/seletz/passierschein/issues/88) ändern die Liste `vaults`, nicht diese Abschnitte. Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
