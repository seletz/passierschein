---
render_macros: true
id: 'D55'
titel: 'a38 config ändert die Konfigurationsdatei nur gezielt: keine Symlinks, Sperrdatei, kein Neuladen'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-12'
order: 6
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag aus der Entwicklung, zu bestätigen durch Stefan
  annahme: true
---
# D55 — a38 config ändert die Konfigurationsdatei nur gezielt: keine Symlinks, Sperrdatei, kein Neuladen

{{ design_kopf(page) }}

**Entscheidung:**

1. **Ort:** `a38 config path` nennt die Datei und den Grund: Schalter `--config`, Variable `A38_CONFIG`, `$XDG_CONFIG_HOME/a38/config.yaml`, `~/.config/a38/config.yaml`. Die Reihenfolge ist die von {{ ref("D49") }}.
2. **Anlegen:** `a38 config init` legt das Verzeichnis mit dem Modus 0700 und die Datei mit dem Modus 0600 an. Die Datei nennt alle Standardwerte und die Quelle des Tokens (`token_source: keychain`) ausdrücklich. Der Befehl überschreibt nie eine Datei. Mit `--force` verschiebt er die alte Datei zuerst nach `config.yaml.bak-<Zeitstempel>`.
3. **Ändern:** `vault add|remove` und `project add|remove` ändern nur den Block der Liste. Kommentare, Reihenfolge und Leerzeilen bleiben Byte für Byte erhalten. Jede Änderung durchläuft vor dem Schreiben dieselben Regeln wie das Laden. Ein Projekt darf nur Vaults der Liste nennen. Ein Pfad ist absolut oder beginnt mit `~/`; ein relativer Pfad ist ein Fehler. Ein Vault, den ein Projekt nutzt, bleibt in der Liste; der Fehler nennt die Projekte. Eine Datei, die nicht lädt, ändert der Befehl nicht.
4. **Schreiben:** Eine temporäre Datei im selben Verzeichnis wird über die Datei umbenannt; der Modus der Datei bleibt. Eine Sperrdatei `config.yaml.lock` (O_EXCL) und ein Vergleich des Inhalts direkt vor dem Umbenennen verhindern zwei gleichzeitige Änderungen.
5. **Kein Symlink:** Ist der Pfad ein symbolischer Link, schreibt a38 nicht durch ihn und meldet das. Das Ziel kann einem anderen Werkzeug gehören.
6. **Kein Neuladen:** Der Daemon liest die Datei nur beim Start ({{ ref("D49") }}). Jeder ändernde Befehl sagt, dass der Daemon neu starten muss.

**Begründung:** Der Mensch soll die Datei nicht von Hand schreiben müssen, aber seine Kommentare behalten. Ein Neuschreiben über den YAML-Baum verliert die Leerzeilen, darum nutzt die Änderung den Baum nur, um die Zeilen zu finden. Die Modi 0700 und 0600 und die Weigerung bei Symlinks folgen dem Fail-Closed-Grundsatz. Ein Neuladen ändert die Entscheidung {{ ref("D49") }} nicht.

**Quelle:** [Issue 63](https://github.com/seletz/passierschein/issues/63), Wunsch von Stefan vom 2026-11-12. Annahme der Entwicklung; Stefan bestätigt sie.
