---
render_macros: true
id: 'D64'
titel: 'a38 mcp maskiert den exakten Wert in der Ausgabe und begrenzt Zeit, Ausgabe und Umgebung des Kommandos'
bereich: 'Grants und Auslieferung'
lauf: '2.0'
stand: '2026-11-15'
order: 12
headless: true
status: gueltig
use_cases: [UC15]
beleg:
  wer: Stefan
  datum: '2026-11-15'
  quelle: ['#94']
---
# D64 — a38 mcp maskiert den exakten Wert in der Ausgabe und begrenzt Zeit, Ausgabe und Umgebung des Kommandos

{{ design_kopf(page) }}

**Entscheidung:** `a38 mcp` maskiert die Werte in der Ausgabe des Kommandos und begrenzt das Kommando. Die Werte der Grenzen setzt die Konfiguration des Daemons ({{ ref("D66") }}). Der Daemon sendet sie mit der Antwort `exec_credentials`. `a38 mcp` liest die Konfiguration nicht ({{ ref("D49") }}).

**Maskierung:**

1. `a38 mcp` ersetzt jedes exakte Vorkommen jedes ausgelieferten Werts in stdout und stderr durch `[masked:<NAME>]`. `<NAME>` ist der Name der Umgebungsvariable.
2. Die Maskierung vergleicht Bytes. Sie erkennt nur den exakten Wert. Sie erkennt keinen kodierten Wert (Base64, URL-Kodierung, Hex), keinen Teil eines Werts in der Mitte und keinen veränderten Wert. Die Doku und der Skill sagen das klar ({{ ref("D68") }}).
3. `a38 mcp` maskiert die ganze gesammelte Ausgabe und kürzt erst danach. An einer Schnittkante maskiert es zusätzlich jeden Anfang und jedes Ende eines Werts mit mindestens 4 Bytes.
4. Bei mehreren Werten maskiert `a38 mcp` den längsten Wert zuerst.
5. Das Ergebnis nennt die Zahl der Maskierungen (`masked`). Der Wert selbst erscheint nie.

**Grenzen:**

| Grenze | Standard | Regel |
|---|---|---|
| Zeit | `2m`, höchstens `10m` | Ein größerer Wert in `timeout_seconds` endet mit `invalid_request`. Am Ende der Zeit sendet `a38 mcp` SIGTERM an die Prozessgruppe, nach 5 Sekunden SIGKILL. Das Ergebnis hat `timed_out: true`. |
| Ausgabe | `64KiB` je Strom | `a38 mcp` behält das Ende der Ausgabe und setzt `stdout_truncated` oder `stderr_truncated`. `a38 mcp` hält höchstens 4 MiB je Strom im Speicher; den Rest zählt es nur. |
| Einträge in `env` | `bundle.max_entries` | wie ein Bündel ({{ ref("D29") }}) |
| Länge des Kommandos | 256 Argumente, 16 KiB zusammen | Mehr endet mit `invalid_request`. |

Ungültiges UTF-8 in der Ausgabe ersetzt `a38 mcp` durch U+FFFD.

**Umgebung:**

- Ein Name in `env` folgt dem Muster `[A-Z_][A-Z0-9_]*`.
- Verboten sind `PATH`, `HOME`, `USER`, `LOGNAME`, `SHELL`, `PWD`, `TMPDIR`, `IFS`, jeder Name mit dem Anfang `DYLD_`, `LD_` oder `A38_`. Ein verbotener Name endet mit `invalid_request`.
- Ein Name aus `env` ersetzt eine gleichnamige Variable der Umgebung von `a38 mcp`.

**Arbeitsverzeichnis:** Das Arbeitsverzeichnis muss im Projekt der Sitzung liegen (Regel aus {{ ref("D49") }}, mit aufgelösten symbolischen Links). `a38 mcp` prüft das zuerst. Der Daemon prüft es selbst noch einmal und lehnt sonst mit `not_allowed` ab.

**Werte im Speicher:** `a38 mcp` hält die Werte nur von der Antwort des Daemons bis zum Ende des Kommandos. Danach überschreibt es die Puffer, so weit Go das erlaubt. Kein Wert steht in einem Log, einer Fehlermeldung oder in `exec_result` ({{ ref("D34") }}).

**Begründung:**

- Eine exakte Maskierung fängt den häufigen Fall: Ein Kommando gibt seine Konfiguration oder eine Fehlermeldung mit dem Wert aus. Mehr verspricht die Maskierung nicht. Eine Erkennung kodierter Werte wäre unvollständig und würde Sicherheit vortäuschen ({{ ref("RI13") }}).
- Das Ende der Ausgabe enthält meist die Fehlermeldung. Deshalb bleibt das Ende.
- Die Zeitgrenze und die Prozessgruppe verhindern Prozesse, die mit dem Wert in der Umgebung weiterlaufen.
- Die verbotenen Namen verhindern, dass ein Wert die Suche nach Programmen oder das Laden von Bibliotheken verändert.

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91). Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
