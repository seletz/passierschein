---
title: Erste Schritte mit a38
description: "Daemon mit Fake-Backend starten, ein Zugangsdatum anfordern, Grants und Audit-Log ansehen, den Grant widerrufen."
tags:
  - Thema/Einstieg
---
# Erste Schritte mit a38

Dieses Tutorial führt dich einmal durch den ganzen Ablauf von Stufe 1.
Du baust `a38`, startest den Daemon, forderst ein Zugangsdatum an und widerrufst den Grant.
Am Ende hast du jeden Teil von Stufe 1 gesehen.

!!! warning "Stufe 1 hat keinen echten Passwortmanager und kein Freigabe-Fenster"
    In diesem Tutorial liest der Daemon die Werte aus einer Datei mit erfundenen Werten (Fake-Backend).
    Das Tutorial nutzt kein 1Password-Backend.
    Es gibt kein Popup und kein Touch ID.
    Der Schalter `--ui auto-approve` gibt jede Anfrage ohne Mensch frei.
    Nutze diese Kombination nur zum Lernen und zum Testen.
    Der Daemon startet mit einem echten Backend nur mit der echten Freigabe-App.

Das Tutorial zeigt nie einen Wert.
Jede Ausgabe, die einen Wert enthält, steht hier mit `[redacted]`.
Maskiere Werte auch in deinen Ausgaben.

## Was du brauchst

- einen Mac mit dem Klon des Repos `seletz/passierschein`
- [mise](https://mise.jdx.dev/) mit `mise install`
- zwei Terminal-Fenster

Das Tutorial nennt sie Terminal 1 und Terminal 2.

## 1. a38 bauen

Baue das Binary in Terminal 2 im Klon des Repos:

```sh
mise run build
```

Die Ausgabe zeigt den Build-Befehl.
Das Binary liegt danach unter `bin/a38`.
Setze den Pfad, damit beide Terminals `a38` finden.
Ersetze `<klon>` durch den Pfad deines Klons:

```sh
export PATH="<klon>/bin:$PATH"
a38 version
```

```text
a38 9a0b7a1 (commit 9a0b7a1, built 2026-11-11T22:59:14Z)
```

Deine Versionsnummer weicht ab.
Setze den `PATH` auch in Terminal 1.

## 2. Arbeitsverzeichnis und Konfiguration anlegen

Lege ein Arbeitsverzeichnis mit einem Beispielprojekt an.
Auf macOS ist `/tmp` ein Link auf `/private/tmp`.
Das stört nicht: Der Daemon löst die Pfade der Konfiguration beim Laden auf.
Ein Pfad, den es nicht gibt, meldet `a38 doctor`.

```sh
mkdir -p /tmp/a38-tutorial/beispielprojekt
cd /tmp/a38-tutorial
```

Schreibe die Konfiguration.
Sie nennt einen Vault der Umgebung `dev` und ein Projekt, das ihn nutzen darf:

```sh
cat > config.yaml <<'EOF'
version: 1
vaults:
  - name: beispiel.app.k8s-dev
    environment: dev
projects:
  - name: beispielprojekt
    paths:
      - /tmp/a38-tutorial/beispielprojekt
    vaults:
      - beispiel.app.k8s-dev
paths:
  socket_dir: /tmp/a38-tutorial/run
  audit_file: /tmp/a38-tutorial/audit.db
EOF
```

Alle anderen Werte haben Standardwerte.
Eine Freigabe gilt vier Stunden.
Das Bündelfenster dauert drei Sekunden.

Schreibe die Datei für das Fake-Backend mit erfundenen Werten:

```sh
cat > fake.yaml <<'EOF'
vaults:
  - name: beispiel.app.k8s-dev
    items:
      - name: datenbank-dev
        fields:
          - name: username
            value: beispiel-wert-1
          - name: password
            value: beispiel-wert-2
EOF
```

Das Format steht in der [Referenz zum Fake-Backend](../reference/70-fake-backend.md).

## 3. Den Daemon starten

Starte den Daemon in Terminal 1.
Wechsle zuerst in das Arbeitsverzeichnis:

```sh
cd /tmp/a38-tutorial
a38 serve --config config.yaml --backend fake --fake-file fake.yaml --ui auto-approve
```

Der Daemon bleibt im Vordergrund und schreibt sein Log:

```text
time=2026-11-12T00:15:59.922+01:00 level=INFO msg="daemon started" config=config.yaml backend=fake ui=auto-approve agent_socket=/tmp/a38-tutorial/run/agent.sock admin_socket=/tmp/a38-tutorial/run/admin.sock
```

Der Daemon legt zwei Sockets an: den Agent-Socket für die Agents und den Admin-Socket für dich.

## 4. Mit a38 doctor prüfen

Wechsle zu Terminal 2 und prüfe den Zustand:

```sh
cd /tmp/a38-tutorial
a38 doctor --config config.yaml
```

```text
OK       configuration      loaded, 1 projects, 1 vaults
OK       socket modes       directory 0700, sockets 0600
OK       daemon             version a9d5f0f-dirty, 0 sessions, 0 grants
OK       delivery lock      delivery is open
OK       audit file         /tmp/a38-tutorial/audit.db exists and is writable
OK       audit space        321790 MiB free (least: 100 MiB)
OK       audit chain        intact, 0 entries
OK       backend            ok
OK       approval UI        ok

No findings.
```

Der Exit-Code ist 0.
Bei einem Befund nennt `doctor` die Ursache und den nächsten Schritt.
Die Befehle stehen in der [Referenz](../reference/80-befehle-fuer-den-menschen.md).

## 5. Ein Zugangsdatum anfordern

Ein Agent fordert Zugangsdaten über den MCP-Server `a38 mcp` an.
Es gibt zwei Wege.

### Weg A: mit einem Agent-Werkzeug

Melde `a38 mcp` in deinem Agent-Werkzeug an.
Die Anleitung [a38 als MCP-Server anmelden](../how-to/22-mcp-server-anmelden.md) zeigt das für Claude Code.
Starte das Werkzeug im Verzeichnis `/tmp/a38-tutorial/beispielprojekt`.
Der Daemon ordnet die Sitzung über das Arbeitsverzeichnis dem Projekt zu.
Bitte den Agent dann, den Wert `op://beispiel.app.k8s-dev/datenbank-dev/password` anzufordern.

### Weg B: ohne Agent-Werkzeug, von Hand

`a38` hat keinen eigenen Befehl, der ein MCP-Werkzeug aufruft.
Der MCP-Server `a38 mcp` liest aber JSON-Zeilen von stdin.
Du schickst ihm darum die drei Nachrichten eines MCP-Clients selbst.
Das ist ein Behelf für das Lernen.

Schreibe die Nachrichten in eine Datei.
Die erste Zeile meldet den Client an.
Die zweite Zeile bestätigt die Anmeldung.
Die dritte Zeile ruft das Werkzeug `request_credentials` auf:

```sh
cd /tmp/a38-tutorial
cat > anfrage.jsonl <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"handaufruf","version":"1"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"request_credentials","arguments":{"references":["op://beispiel.app.k8s-dev/datenbank-dev/password"],"task":"Tutorial","reason":"Erste Schritte"}}}
EOF
```

Starte `a38 mcp` im Projektverzeichnis.
Das Verzeichnis bestimmt das Projekt.
Die Sitzung muss offen bleiben, solange du den Grant ansiehst.
Darum hält das Kommando stdin 120 Sekunden offen.
Es läuft im Hintergrund:

```sh
cd /tmp/a38-tutorial/beispielprojekt
( (cat ../anfrage.jsonl; sleep 120) | a38 mcp --config ../config.yaml \
    --session-name beispiel-koordinator > ../antwort.jsonl 2>/dev/null & )
sleep 6
cd ..
```

Das Bündelfenster dauert drei Sekunden.
Darum wartet der Befehl `sleep 6`.
Zeige die Antwort an und maskiere den Wert:

```sh
sed -E 's/(value[^:]*:[^"]*")[^"\\]*/\1[redacted]/g' antwort.jsonl
```

```text
{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"logging":{},"tools":{"listChanged":true}},"instructions":"a38 asks a human before it gives credentials. Use the skill of a38.","protocolVersion":"2025-06-18","serverInfo":{"name":"a38","version":"9a0b7a1"}}}
{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"entries\":[{\"reference\":\"op://beispiel.app.k8s-dev/datenbank-dev/password\",\"value\":\"[redacted]\"}]}"}],"structuredContent":{"entries":[{"reference":"op://beispiel.app.k8s-dev/datenbank-dev/password","value":"[redacted]"}]}}}
```

Die zweite Zeile ist die Antwort auf die Anfrage.
Sie enthält ein Feld `value`.
Der Daemon hat die Anfrage mit `--ui auto-approve` ohne Mensch freigegeben und den Wert aus dem Fake-Backend geliefert.
Zeige den Wert nie in Docs, Tickets oder Logs.

Endet die Antwort mit `not_allowed`, liegt das Projektverzeichnis nicht im Pfad der Konfiguration.
Prüfe, ob du in `beispielprojekt` gestartet hast. Prüfe auch, ob `a38 doctor` einen Pfad meldet, den es nicht gibt.

## 6. Grants ansehen

Der Daemon hat nach der Freigabe einen Grant vergeben.
Der Grant gilt für diese Sitzung und dieses Zugangsdatum:

```sh
a38 grants list --config config.yaml
```

```text
GRANT                             SESSION   NAME                  PROJECT          CREDENTIALS                                       REMAINING
grant-vvxtdzfet7icpv7kxt7pbjox74  s-4d5af6  beispiel-koordinator  beispielprojekt  op://beispiel.app.k8s-dev/datenbank-dev/password  3h59m56s
```

Die Spalte `CREDENTIALS` zeigt die Referenz und nie den Wert.
Eine zweite Anfrage für dieses Zugangsdatum braucht in den nächsten vier Stunden keine neue Freigabe.

## 7. Das Audit-Log ansehen und prüfen

Der Daemon schreibt jedes Ereignis in das Audit-Log:

```sh
a38 audit list --config config.yaml
```

```text
SEQ  TIME                  TYPE      PROJECT          SESSION               REFERENCES                                        RESULT
1    2026-11-11T23:16:01Z  request   beispielprojekt  beispiel-koordinator  op://beispiel.app.k8s-dev/datenbank-dev/password  ok
2    2026-11-11T23:16:04Z  bundle    beispielprojekt  beispiel-koordinator  op://beispiel.app.k8s-dev/datenbank-dev/password  ok
3    2026-11-11T23:16:04Z  decision  beispielprojekt  beispiel-koordinator  op://beispiel.app.k8s-dev/datenbank-dev/password  ok (approved)
4    2026-11-11T23:16:04Z  grant     beispielprojekt  beispiel-koordinator  op://beispiel.app.k8s-dev/datenbank-dev/password  ok
5    2026-11-11T23:16:04Z  delivery  beispielprojekt  beispiel-koordinator  op://beispiel.app.k8s-dev/datenbank-dev/password  ok
```

Die Reihenfolge ist Anfrage, Bündel, Entscheidung, Grant, Auslieferung.
Die Zeit steht in UTC.
Prüfe die Hash-Kette:

```sh
a38 audit verify --config config.yaml
```

```text
The chain is intact: 5 entries.
```

Der Exit-Code ist 0.
Bei einem Bruch nennt der Befehl den ersten fehlerhaften Eintrag und endet mit 1.

## 8. Den Grant widerrufen

Kopiere die Kennung des Grants aus der Liste von Schritt 6.
Die Option `--config` darf vor und nach der Kennung stehen:

```sh
a38 grants revoke grant-vvxtdzfet7icpv7kxt7pbjox74 --config config.yaml
```

```text
The grant grant-vvxtdzfet7icpv7kxt7pbjox74 ended. The next request needs a new approval.
```

Die Liste ist jetzt leer:

```sh
a38 grants list --config config.yaml
```

```text
No active grants.
```

Der Widerruf steht im Audit-Log:

```sh
a38 audit list --config config.yaml --type revocation
```

```text
SEQ  TIME                  TYPE        PROJECT          SESSION               REFERENCES                                        RESULT
6    2026-11-11T23:16:25Z  revocation  beispielprojekt  beispiel-koordinator  op://beispiel.app.k8s-dev/datenbank-dev/password  ok (the human revoked the grant)
```

Die Kette ist weiter intakt:

```sh
a38 audit verify --config config.yaml
```

```text
The chain is intact: 6 entries.
```

## 9. Den Daemon stoppen

Beende zuerst die offene Sitzung.
Sie endet von selbst nach 120 Sekunden.
Drücke dann in Terminal 1 die Tasten `Ctrl-C`.
Das Log zeigt das Ende:

```text
time=2026-11-12T00:16:01.295+01:00 level=INFO msg="session started" session_id=s-4d5af6f40fc479e91d4d210bfdae5626 project=beispielprojekt peer_pid=57288
time=2026-11-12T00:16:29.615+01:00 level=INFO msg="session ended" session_id=s-4d5af6f40fc479e91d4d210bfdae5626
time=2026-11-12T00:16:30.261+01:00 level=INFO msg="daemon stopped"
```

Der Daemon entfernt seine Sockets.
Ein Grant, der beim Stopp noch lebt, endet mit dem Grund `daemon_stopped` im Audit-Log.
Das Audit-Log bleibt.
Du prüfst es auch ohne Daemon:

```sh
a38 audit verify --config config.yaml
```

```text
The chain is intact: 6 entries.
```

`a38 doctor` meldet jetzt einen Befund, weil der Daemon nicht antwortet:

```text
FINDING  daemon             the daemon does not answer
```

Lösche das Arbeitsverzeichnis, wenn du es nicht mehr brauchst:

```sh
rm -rf /tmp/a38-tutorial
```

## Wie es weitergeht

- [a38 als MCP-Server anmelden](../how-to/22-mcp-server-anmelden.md): den Server in Claude Code eintragen.
- [Störung finden und beheben](../how-to/24-stoerung-finden-und-beheben.md): `a38 doctor` und die globale Sperre.
- [Befehle für den Menschen](../reference/80-befehle-fuer-den-menschen.md): alle Argumente und Exit-Codes.
- [Testabdeckung](../reference/90-testabdeckung.md): was die Tests belegen.
- [Design](../design/index.md): Entscheidungen, Features und Testfälle.
