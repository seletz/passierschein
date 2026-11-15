---
render_macros: true
id: 'D72'
titel: 'Jedes Konto hat sein Token in einem eigenen Eintrag des Schlüsselbunds'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 10
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag aus der Entwicklung (Issue 88), zu bestätigen durch Stefan
  annahme: true
---
# D72 — Jedes Konto hat sein Token in einem eigenen Eintrag des Schlüsselbunds

{{ design_kopf(page) }}

**Entscheidung:** Die Entscheidung ersetzt {{ ref("D51") }}. Die Regeln aus {{ ref("D51") }} gelten weiter, jetzt je Konto. Neu ist nur der Bezug auf das Konto.

1. **Schlüsselbund:** Jedes Konto mit der Quelle `keychain` hat einen eigenen Eintrag im Anmelde-Schlüsselbund. Der Dienst ist für alle Konten `a38.onepassword`. Das Konto des Eintrags ist der Name des Kontos aus der Konfiguration ({{ ref("D71") }}). `a38` legt den Eintrag selbst über Security.framework an, nie über `/usr/bin/security`.
2. **Alter Eintrag:** Der Eintrag `a38.onepassword` mit dem Konto `service-account` stammt aus {{ ref("D51") }}. Der Daemon liest ihn nicht mehr. Deshalb ist der Name `service-account` für ein Konto gesperrt.
3. **Befehle:** `a38 config onepassword-token set --account <name> [--replace]`, `a38 config onepassword-token status [--account <name>]` und `a38 config onepassword-token delete --account <name>`.
    - `set` verlangt `--account`. Das Konto muss in der Konfiguration stehen. Fehlt die Konfiguration oder das Konto, liest der Befehl nichts und endet mit einem Fehler. Ein Tippfehler legt so keinen verwaisten Eintrag an.
    - `set` liest das Token nur vom Terminal, ohne Echo. Ist die Standardeingabe kein Terminal, liest der Befehl nichts und endet mit dem Exit-Code 2. Das Token ist nie ein Argument, nie eine Umgebungsvariable und kommt nie aus einer Datei oder einer Pipe. Ein vorhandener Eintrag wird nur mit `--replace` ersetzt.
    - `status` ohne `--account` zeigt jedes Konto der Konfiguration und den alten Eintrag. Mit `--account` zeigt es ein Konto. `status` liest nie ein Token.
    - `delete --account <name>` löscht den Eintrag des Kontos. Das Konto muss nicht in der Konfiguration stehen; so entfernt der Mensch den Eintrag eines Kontos, das er aus der Konfiguration genommen hat. `delete --legacy` löscht den alten Eintrag aus Punkt 2.
4. **Nur Metadaten:** Kein Befehl zeigt ein Token oder einen daraus abgeleiteten Wert. `status` und `a38 doctor` zeigen je Konto nur Existenz, Datum der Anlage und der Änderung, vertraute Programme und Partitionsliste; dazu die Signatur von `a38`. Eine Token-Datei prüfen sie nur über Besitzer und Rechte.
5. **Datei:** Die Token-Datei bleibt der Ausweichweg für Entwicklung und CI, je Konto mit den Regeln aus {{ ref("D51") }} (reguläre Datei des Benutzers, Modus `0600`).
6. Es gibt keine Umgebungsvariable für ein Token. Der Daemon hält jedes Token nur im Arbeitsspeicher, im Typ mit der Ausgabe `[redacted]`.

**Folge für den Hook:** `a38 hook op-guard` ({{ ref("D58") }}) erkennt auch die Token-Datei eines Kontos (`…/onepassword-token-<name>`) und den Eintrag eines Kontos über Dienst oder Bezeichnung.

**Begründung:** Ein Eintrag je Konto unter einem Dienst hält die Einträge in der Schlüsselbundverwaltung zusammen. Das Konto des Eintrags ist der natürliche Platz für den Namen. Der Bezug auf die Konfiguration bei `set` verhindert einen Eintrag, den der Daemon nie liest. Der alte Eintrag lässt sich nicht still übernehmen: Ein neuer Build löst beim Lesen einen Dialog aus, und `a38` liest ein Token nur vom Terminal.

**Grenze:** Die Grenze bis zur Signatur aus {{ ref("D51") }} gilt für jeden Eintrag ({{ ref("G5") }}). Mit mehreren Konten zeigt ein neuer Build mehrere Dialoge.

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88). Vorschlag aus der Entwicklung, zu bestätigen durch Stefan.
