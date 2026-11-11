---
title: "Spike: Zugang der VMs zum Daemon über Tailscale"
description: "Der Spike klärt, ob der Daemon den Agent-Endpunkt für VMs über Tailscale bereitstellt, welche Identität Tailscale liefert und wie die Policy den Zugriff beschränkt."
tags:
  - Spike
  - Sicherheit
---
# Spike: Zugang der VMs zum Daemon über Tailscale

Stand: 2026-11-11. Bezug: Issue seletz/passierschein#11.

Der Spike ist eine Untersuchung aus der Dokumentation. Der Spike hat nichts am Tailnet und nichts an der Policy geändert. Der Spike hat keine VM angelegt und keinen Listener gestartet.

## Frage

Der Daemon `a38 serve` läuft auf dem Mac. Der Daemon stellt zwei Unix-Sockets bereit:

- den Agent-Socket für `a38 mcp`,
- den Admin-Socket für die Verwaltung.

In einer späteren Stufe laufen die Coding Agents in Linux-VMs. Die VM ist die Zone des Agents. Die VM gilt als nicht vertraut.

Der Spike beantwortet diese Fragen:

1. Kann der Daemon den Agent-Endpunkt für VMs über Tailscale bereitstellen?
2. Welche Identität einer VM liefert Tailscale, und ist diese Identität ein geprüftes Faktum?
3. Wie stellt die Policy sicher, dass nur die eigenen VMs den Agent-Endpunkt erreichen und dass die VMs nichts anderes erreichen?
4. Was folgt daraus für den Betrieb der VMs?

Das Issue nennt die betroffenen Artefakte des Design-Sets. Der Bericht nennt keine Nummern, weil der Zweig des Spikes diese Artefakte nicht enthält.

## Ergebnis

Ja, der Weg trägt. Der Spike empfiehlt diese Form:

- Der Daemon bettet mit der Go-Bibliothek `tsnet` einen eigenen Knoten in das Tailnet ein. Der Knoten trägt den Tag `tag:a38-host`.
- Jede VM ist ein eigener Knoten mit dem Tag `tag:agent-vm`.
- Die Policy erlaubt genau eine Verbindung: von `tag:agent-vm` zu `tag:a38-host` auf einem TCP-Port.
- Der Daemon fragt für jede Verbindung `WhoIs` ab. Der Daemon prüft den Tag selbst noch einmal und führt eine eigene Liste der erlaubten Knoten.

Die wichtigsten Befunde:

- Tailscale liefert die Identität des Knotens, nicht die Identität eines Prozesses. Für einen Knoten mit Tag liefert Tailscale die Tags und die Knoten-ID, aber keinen Benutzer.
- Die Identitäts-Header von `tailscale serve` sind für Knoten mit Tag leer. `tailscale serve` scheidet damit als Quelle der Identität aus.
- Ein eigener Knoten mit Tag trennt den Agent-Endpunkt vom Mac. Die VMs erreichen keinen Port des Mac. Die bestehende Regel für iPhone und iPad gilt für den eigenen Knoten nicht.
- Ein Listener von `tsnet` ist kein Listener des Betriebssystems. Ein lokaler Prozess auf dem Mac erreicht den Listener nur über das Tailnet und nur, wenn die Policy das erlaubt.
- Die VM ist die Sitzungsgrenze. Tailscale unterscheidet mehrere Agents in einer VM nicht. Jede weitere Kennung in der VM ist eine Angabe des Agents.
- Die Identität der VM ist so stark wie die Kontrolle über den Auth-Key und über den Knotenschlüssel in der VM. Ein Agent mit Root-Recht in der VM kann den Knotenschlüssel kopieren. Die Policy begrenzt, was dieser Schlüssel erreicht.

Ein praktischer Test steht aus. Der Abschnitt [Testplan](#testplan) nennt die Schritte.

## Befunde

Jeder Befund trägt einen Status:

- **Doku:** Die Dokumentation von Tailscale oder eine andere Primärquelle belegt die Aussage. Der Link steht beim Befund.
- **Quelltext:** Der Quelltext von Tailscale im Stand v1.104.6 belegt die Aussage. Der Link steht beim Befund.
- **Lokal gelesen:** Ein lesender Befehl auf dem Mac belegt die Aussage. Der Spike hat nur `tailscale version` und Dateilisten gelesen.
- **Annahme:** Die Aussage ist eine Ableitung. Sie ist nicht belegt. Der praktische Test muss sie prüfen.

### Identität

#### Was Tailscale über eine Verbindung weiß

- **Doku:** Jeder Knoten hat ein Schlüsselpaar. WireGuard authentifiziert die beiden Knoten einer Verbindung über die Knotenschlüssel. Der private Schlüssel verlässt das Gerät nicht ([Tailscale identity](https://tailscale.com/docs/concepts/tailscale-identity)).
- **Doku:** Der Zielknoten sucht die Quelladresse einer Verbindung in der Netzkarte, die der Knoten vom Koordinationsserver erhält. So ordnet der Zielknoten eine Quelladresse einem Knoten zu (gleiche Seite).
- **Doku:** Ein Gerät mit Tag hat keine Benutzeridentität: „Applying a tag to a device removes any user-based authentication.“ Die Identität eines Geräts mit Tag ist die Menge seiner Tags ([Tags](https://tailscale.com/kb/1068/tags), [Tailscale identity](https://tailscale.com/docs/concepts/tailscale-identity)).
- **Doku:** Die Adresse eines Knotens bleibt gleich, solange der Knoten registriert ist und kein Admin sie ändert ([IP-Adressen](https://tailscale.com/kb/1033/ip-and-dns-addresses)).

#### WhoIs

- **Doku:** `WhoIs` liefert den Besitzer einer Adresse: „WhoIs returns the owner of the remoteAddr, which must be an IP or IP:port.“ Kennt der Knoten die Adresse nicht, liefert der Aufruf den Fehler `ErrPeerNotFound` ([Paket `client/local`](https://pkg.go.dev/tailscale.com/client/local)).
- **Doku:** Die Antwort hat den Typ `WhoIsResponse` mit den Feldern `Node`, `UserProfile` und `CapMap`. „In successful whois responses, Node and UserProfile are never nil“ ([Paket `apitype`](https://pkg.go.dev/tailscale.com/client/tailscale/apitype)).
- **Quelltext:** `Node.Tags` enthält die Tags des Knotens. `Node.IsTagged()` meldet, ob der Knoten Tags hat. `Node.User` ist der Benutzer, der den Knoten angelegt hat: „If ACL tags are in use for the node then it doesn't reflect the ACL identity that the node is running as“ ([tailcfg.go](https://github.com/tailscale/tailscale/blob/v1.104.6/tailcfg/tailcfg.go)).
- **Quelltext:** `Node.StableID` ist die Knoten-ID als Zeichenkette. `Node.Key` ist der öffentliche Knotenschlüssel. `Node.Sharer` ist gesetzt, wenn ein fremdes Tailnet den Knoten geteilt hat (gleiche Datei).
- **Quelltext:** Die lokale API verlangt für `WhoIs` nur das Leserecht. Die Antwort entsteht aus der Netzkarte des Knotens ([localapi.go](https://github.com/tailscale/tailscale/blob/v1.104.6/ipn/localapi/localapi.go), [local.go](https://github.com/tailscale/tailscale/blob/v1.104.6/ipn/ipnlocal/local.go)).
- **Annahme:** Für einen Knoten mit Tag enthält `UserProfile` einen Platzhalter und keinen echten Benutzer. Der Daemon wertet `UserProfile` für einen Knoten mit Tag nicht aus.
- **Annahme:** `Node.StableID` ändert sich, wenn eine VM dem Tailnet neu beitritt. `Node.StableID` bleibt gleich, wenn die VM nur neu startet.
- **Annahme:** Der Name des Knotens und die Angaben in `Node.Hostinfo` stammen vom Knoten selbst. Ein Agent mit Root-Recht ändert den Namen mit `tailscale set --hostname`. Die Dokumentation belegt nur, dass der Name dem Hostnamen des Betriebssystems folgt ([Machine names](https://tailscale.com/kb/1098/machine-names)).

#### Ist die Identität ein geprüftes Faktum?

Die Antwort von `WhoIs` ist ein geprüftes Faktum unter drei Bedingungen:

1. Die Verbindung kommt wirklich aus dem Tailnet. Bei einem Listener von `tsnet` gilt das für jede Verbindung. Bei einem TCP-Listener des Betriebssystems gilt das nicht für lokale Verbindungen.
2. Der Daemon fragt den Knoten, der die Verbindung angenommen hat. Bei `tsnet` ist das der Knoten im eigenen Prozess.
3. Der Koordinationsserver von Tailscale verteilt die richtigen Schlüssel. Tailnet Lock sichert diesen Punkt zusätzlich ab: Ein Knoten tritt nur bei, wenn ein vertrauter Knoten ihn signiert ([Tailnet Lock](https://tailscale.com/kb/1226/tailnet-lock)).

Geprüft sind damit diese Angaben:

- die Knoten-ID (`Node.StableID`),
- die Tags (`Node.Tags`),
- die Tailscale-Adresse der Verbindung.

Nicht geprüft sind diese Angaben:

- der Name des Knotens,
- das Betriebssystem und alle weiteren Angaben in `Node.Hostinfo`,
- jede Angabe über den Prozess oder den Agent in der VM.

Die Identität sagt: „Diese Verbindung kommt von dem Knoten, der diesen Knotenschlüssel hält.“ Die Identität sagt nicht: „Diese Verbindung kommt von der VM.“ Wer den Knotenschlüssel aus der VM kopiert, tritt als dieser Knoten auf.

#### Was das Popup zeigt

Der Spike schlägt diese geprüften Fakten für das Popup vor:

- den Namen der VM aus der Liste des Daemons, zum Beispiel `agent-vm-1`,
- den Weg der Anfrage: „über Tailscale“,
- den Tag `tag:agent-vm`.

Der Daemon führt eine Liste, die jeder Knoten-ID einen Namen zuordnet. Das Popup zeigt den Namen aus dieser Liste und nicht den Namen, den der Knoten selbst meldet. Die Knoten-ID steht im Audit-Log.

#### VM oder Sitzung

- **Doku:** Tailscale kennt den Knoten. Tailscale kennt keinen Prozess und keinen Benutzer innerhalb des Knotens ([Tailscale identity](https://tailscale.com/docs/concepts/tailscale-identity)).
- **Folge:** Alle Agents einer VM haben dieselbe Tailscale-Adresse. Der Daemon unterscheidet nur einzelne TCP-Verbindungen.
- **Folge:** Eine Freigabe für eine Verbindung schützt nicht vor einem zweiten Prozess in derselben VM. Ein Prozess mit Root-Recht in der VM liest den Speicher eines anderen Prozesses.

Der Spike schlägt diese Regel vor:

- Die VM ist die Sitzungsgrenze für die Sicherheit. Eine Freigabe gilt höchstens für die VM.
- Der Daemon vergibt je Verbindung von `a38 mcp` eine Sitzungsnummer. Die Sitzungsnummer dient der Bündelung und dem Audit-Log. Die Sitzungsnummer ist keine Sicherheitsgrenze.
- Der Name des Agents und die Aufgabe bleiben Angaben des Agents. Das Popup zeigt sie im unteren Teil.
- Brauchen zwei Aufgaben getrennte Freigaben, laufen sie in zwei VMs.

### Zugriffsbeschränkung

#### Was die Policy kann

- **Doku:** Die Policy verbietet alles, was keine Regel erlaubt: „Tailscale access rules deny access by default“ ([Syntax der Policy](https://tailscale.com/docs/reference/syntax/policy-file)).
- **Doku:** Ein Grant nennt Quelle (`src`), Ziel (`dst`) und Rechte auf Netzebene (`ip`). `"ip": ["tcp:443"]` erlaubt genau einen TCP-Port. Mehrere Grants ergeben die Vereinigung der Rechte ([Syntax der Grants](https://tailscale.com/docs/reference/syntax/grants)).
- **Doku:** Grants und die älteren ACLs dürfen in derselben Policy stehen ([Grants](https://tailscale.com/kb/1324/grants)).
- **Doku:** Als Ziel sind unter anderem Tags, Benutzer, `autogroup:self`, Adressen und Namen aus dem Abschnitt `hosts` erlaubt ([Syntax der Grants](https://tailscale.com/docs/reference/syntax/grants)).
- **Doku:** `autogroup:self` gilt nur für Geräte eines Benutzers: „It does not apply to tagged devices“ ([Syntax der Policy](https://tailscale.com/docs/reference/syntax/policy-file)).
- **Doku:** Eine Regel wirkt in eine Richtung. Das Ziel darf die Quelle nicht von sich aus erreichen ([Device visibility](https://tailscale.com/docs/concepts/device-visibility)).
- **Doku:** Ein Knoten sieht nur die Knoten, die er erreichen darf, und die Knoten, die ihn erreichen dürfen. Ohne Regel zwischen zwei Knoten kennt keiner den anderen (gleiche Seite).
- **Doku:** Der Abschnitt `tests` prüft die Policy beim Speichern. Schlägt ein Test fehl, lehnt Tailscale die Policy ab ([Syntax der Policy](https://tailscale.com/docs/reference/syntax/policy-file)).
- **Doku:** Nur die Besitzer eines Tags und Admins vergeben den Tag. Der Abschnitt `tagOwners` nennt die Besitzer ([Tags](https://tailscale.com/kb/1068/tags)).

#### Folgen für die bestehende Regel

- **Annahme:** Die bestehende Regel für die eigenen Geräte nutzt `autogroup:member` als Quelle und `autogroup:self` als Ziel. Der Spike hat die Policy nicht gelesen.
- **Folge:** Trifft die Annahme zu, erfasst die bestehende Regel weder die VMs noch den Knoten des Daemons. Beide tragen einen Tag.
- **Folge:** Der Mac selbst gehört einem Benutzer. Ein Grant von `tag:agent-vm` zum Mac braucht den Mac als Ziel. Dafür gibt es drei Wege:
    - Der Mac erhält einen Tag. Der Mac verliert damit die Benutzeridentität. Die bestehende Regel für iPhone und iPad gilt dann nicht mehr für den Mac. Der Spike rät davon ab.
    - Die Policy nennt die Adresse des Mac im Abschnitt `hosts`. Die VMs erreichen dann einen Port des Mac. iPhone und iPad erreichen denselben Port über die bestehende Regel.
    - Der Daemon betreibt einen eigenen Knoten mit Tag. Die Policy nennt nur diesen Knoten. Das ist der empfohlene Weg.
- **Folge:** Die Policy darf keine Regel mit der Quelle `*` oder `autogroup:tagged` enthalten. Eine solche Regel öffnet den VMs weitere Ziele.

#### Was der Daemon selbst prüft

Die Policy ist die erste Sperre. Der Daemon ist die zweite Sperre. Der Abschnitt [Prüfungen im Daemon](#prufungen-im-daemon) nennt die Prüfungen. Der Daemon lehnt jede Verbindung ab, die nicht von einem Knoten mit `tag:agent-vm` kommt. Eine zu weite Policy lässt damit zwar die TCP-Verbindung zu, aber der Daemon schließt die Verbindung.

### Endpunkt auf dem Mac

Der Spike vergleicht drei Formen.

#### Form A: `tailscale serve` vor dem Agent-Socket

- **Doku:** `tailscale serve` leitet HTTP-Anfragen an einen lokalen Dienst weiter und setzt die Header `Tailscale-User-Login`, `Tailscale-User-Name` und `Tailscale-User-Profile-Pic`. `tailscale serve` entfernt gleichnamige Header aus der Anfrage ([Tailscale Serve](https://tailscale.com/kb/1312/serve)).
- **Doku:** Die Header fehlen für Knoten mit Tag: „These identity headers are not populated for traffic originating from tagged devices“ (gleiche Seite).
- **Quelltext:** Der Quelltext bestätigt das. Die Funktion `addTailscaleIdentityHeaders` kehrt für einen Knoten mit Tag ohne Header zurück ([serve.go](https://github.com/tailscale/tailscale/blob/v1.104.6/ipn/ipnlocal/serve.go)).
- **Doku:** Der lokale Dienst darf nur auf `localhost` lauschen. Sonst setzt ein Aufrufer die Header selbst (gleiche Seite).
- **Doku:** `tailscale serve --tcp=<port>` leitet rohe TCP-Verbindungen weiter. Die Option `--proxy-protocol` überträgt die Quelladresse an den lokalen Dienst ([Befehl `tailscale serve`](https://tailscale.com/kb/1242/tailscale-serve)).
- **Doku:** Ein Unix-Socket als Ziel verlangt Root-Recht: „Serving a file, directory, or Unix socket requires root.“ Das Bereitstellen von Dateien und Verzeichnissen geht auf macOS nur mit der Open-Source-Variante ([Tailscale Serve](https://tailscale.com/kb/1312/serve)).
- **Annahme:** Die Standalone-Variante auf macOS erreicht einen Unix-Socket im Benutzerverzeichnis nicht, weil die Erweiterung in einer Sandbox läuft.
- **Folge:** Form A liefert für VMs keine Identität über Header. Mit `--proxy-protocol` erhält der Daemon nur die Quelladresse und muss `WhoIs` trotzdem selbst abfragen. Der Daemon muss außerdem darauf vertrauen, dass jede Verbindung am lokalen Ziel von `tailscale serve` stammt. Ein lokaler TCP-Port auf `localhost` ist für jeden lokalen Prozess offen. Form A scheidet aus.

#### Form B: TCP-Listener auf der Tailscale-Adresse des Mac

Der Daemon öffnet einen TCP-Listener auf der Tailscale-Adresse des Mac. Der Daemon fragt `WhoIs` über die lokale API des Tailscale-Clients auf dem Mac ab.

Tailscale gibt es auf macOS in drei Varianten ([macOS-Varianten](https://tailscale.com/kb/1065/macos-variants)):

| Variante | Aufbau | Zugang zur lokalen API |
|---|---|---|
| Mac App Store | Network Extension, Sandbox, läuft als lokaler Benutzer | TCP-Port auf `localhost` mit Token. Ein Prozess desselben Benutzers findet Port und Token über `lsof`. |
| Standalone | System Extension, Sandbox, läuft als Root | TCP-Port auf `localhost` mit Token. Port und Token liegen unter `/Library/Tailscale`. Die Token-Datei gehört `root:admin` mit dem Modus `0640`. |
| Open Source (`tailscaled`) | utun-Schnittstelle, keine Sandbox, kein GUI | Unix-Socket `/var/run/tailscaled.socket`. |

- **Doku:** Tailscale empfiehlt die Standalone-Variante (gleiche Seite).
- **Quelltext:** Die Zeile „Zugang zur lokalen API“ stammt aus dem Quelltext ([safesocket_darwin.go](https://github.com/tailscale/tailscale/blob/v1.104.6/safesocket/safesocket_darwin.go), [paths.go](https://github.com/tailscale/tailscale/blob/v1.104.6/paths/paths.go)).
- **Doku:** Der Go-Client `local.Client` kennt diese Unterschiede. Das Feld `UseSocketOnly` schaltet den Weg über Port und Token ab ([Paket `client/local`](https://pkg.go.dev/tailscale.com/client/local)).
- **Lokal gelesen:** Der Mac im Testumfeld nutzt die Standalone-Variante in Version 1.104.6. Die Token-Datei unter `/Library/Tailscale` gehört `root:admin` mit dem Modus `0640`.
- **Folge:** Bei der Standalone-Variante liest nur ein Mitglied der Gruppe `admin` das Token. Der Daemon läuft dann unter einem Admin-Konto, oder `WhoIs` schlägt fehl.
- **Annahme:** Wer das Token hält, hat bei den Varianten mit Sandbox vollen Zugriff auf die lokale API, nicht nur Leserecht.

Erreichbarkeit eines TCP-Listeners im Vergleich zum Unix-Socket:

| Wer | Unix-Socket (Modus `0600`) | TCP-Listener auf der Tailscale-Adresse des Mac |
|---|---|---|
| Prozess desselben Benutzers | erreicht den Socket | erreicht den Listener |
| Prozess eines anderen Benutzers auf dem Mac | erreicht den Socket nicht | erreicht den Listener |
| Prozess in einer App-Sandbox mit Netzrecht | erreicht den Socket nicht | erreicht den Listener |
| Knoten im Tailnet | erreicht den Socket nicht | erreicht den Listener, wenn die Policy den Port erlaubt |
| iPhone und iPad des Besitzers | erreichen den Socket nicht | erreichen den Listener über die bestehende Regel |
| Gerät im lokalen Netz ohne Tailscale | erreicht den Socket nicht | erreicht den Listener nicht, solange der Listener nur an die Tailscale-Adresse gebunden ist |

- **Annahme:** Die Zeilen der Tabelle für den TCP-Listener folgen aus dem Verhalten von TCP auf macOS. Die Dateirechte eines Unix-Sockets haben bei TCP kein Gegenstück.
- **Annahme:** Eine lokale Verbindung zur eigenen Tailscale-Adresse hat die Tailscale-Adresse des Mac als Quelle. Offen ist, ob `WhoIs` dafür den eigenen Knoten liefert oder einen Fehler. Der Daemon muss beide Fälle ablehnen.
- **Annahme:** Der Listener lässt sich erst binden, wenn Tailscale verbunden ist. Der Daemon muss den Listener nach jedem Neustart von Tailscale neu binden.
- **Folge:** Form B hängt von der Variante des Tailscale-Clients ab, öffnet einen Port des Mac für alle lokalen Prozesse und für die eigenen Geräte und braucht die Adresse des Mac in der Policy.

#### Form C: eigener Knoten mit `tsnet` im Daemon

- **Doku:** `tsnet` bettet einen Knoten in ein Go-Programm ein. `tsnet` nutzt einen Netzstapel im Prozess und braucht weder Root-Recht noch einen Systemdienst ([Paket `tsnet`](https://pkg.go.dev/tailscale.com/tsnet)).
- **Doku:** Mit `tsnet` laufen mehrere Dienste auf einem Gerät mit eigenen Adressen, eigenen Regeln und eigenen Identitäten ([tsnet](https://tailscale.com/kb/1244/tsnet)).
- **Doku:** `Server.Listen` lauscht nur im Tailnet: „Listen announces only on the Tailscale network“ ([Paket `tsnet`](https://pkg.go.dev/tailscale.com/tsnet)).
- **Doku:** `Server.LocalClient` liefert einen Client für den eingebetteten Knoten. Der Daemon ruft darauf `WhoIs` auf. Das Beispiel `tshello` von Tailscale zeigt genau diesen Ablauf ([tshello.go](https://github.com/tailscale/tailscale/blob/v1.104.6/tsnet/example/tshello/tshello.go)).
- **Doku:** `Server.AdvertiseTags` setzt die Tags des Knotens. `Server.Dir` nennt das Verzeichnis für den Zustand. Der Zustand liegt in der Datei `tailscaled.state` in diesem Verzeichnis ([Paket `tsnet`](https://pkg.go.dev/tailscale.com/tsnet)).
- **Doku:** Ohne Auth-Key zeigt der erste Start eine Anmelde-URL. Der Besitzer meldet den Knoten im Browser an ([tsnet](https://tailscale.com/kb/1244/tsnet)).
- **Doku:** Ein Knoten mit Tag hat keinen Schlüsselablauf ([Tags](https://tailscale.com/kb/1068/tags)).
- **Folge:** Der Agent-Endpunkt ist ein eigener Knoten. Die Policy nennt `tag:a38-host` als Ziel. Die VMs erreichen keinen Port des Mac. Die bestehende Regel für die eigenen Geräte erfasst den Knoten nicht.
- **Folge:** Es gibt keinen Listener im Betriebssystem. Ein lokaler Prozess auf dem Mac erreicht den Agent-Endpunkt nur als Knoten im Tailnet über den Tailscale-Client des Mac. Die Policy erlaubt das nicht, solange kein Grant vom Benutzer zu `tag:a38-host` führt.
- **Folge:** Form C hängt nicht von der Variante des Tailscale-Clients auf dem Mac ab. Der Daemon braucht keinen Zugang zur lokalen API des Clients.
- **Folge:** Der Zustand von `tsnet` enthält den privaten Knotenschlüssel. Das Verzeichnis braucht den Modus `0700`. Ein Prozess desselben Benutzers liest die Datei. Dieser Prozess erreicht auch den Agent-Socket, daher entsteht kein neues Recht.
- **Annahme:** `tsnet` vergrößert das Binary und die Liste der Abhängigkeiten deutlich. `tsnet` baut mit `CGO_ENABLED=0`.
- **Annahme:** Ein Knoten von `tsnet` und der Tailscale-Client laufen ohne Störung auf demselben Mac.
- **Annahme:** Die Verbindung von einer VM auf demselben Mac zum Knoten des Daemons läuft direkt und nicht über einen Relay-Server.

Skizze aus der Dokumentation, nicht übersetzt und nicht gegen einen Dienst gestartet:

```go
srv := &tsnet.Server{
	Dir:           stateDir, // mode 0700, holds the node key
	Hostname:      "a38-host",
	AdvertiseTags: []string{"tag:a38-host"},
}
defer srv.Close()

ln, err := srv.Listen("tcp", ":8438") // tailnet only, no OS listener
if err != nil {
	return err
}
lc, err := srv.LocalClient()
if err != nil {
	return err
}

for {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	who, err := lc.WhoIs(ctx, conn.RemoteAddr().String())
	if err != nil { // includes local.ErrPeerNotFound
		conn.Close()
		continue
	}
	n := who.Node
	if !n.IsTagged() || !slices.Contains(n.Tags, "tag:agent-vm") || n.Sharer != 0 {
		conn.Close()
		continue
	}
	if !allowed(n.StableID) { // list of enrolled VMs in the daemon configuration
		conn.Close()
		continue
	}
	go serveAgent(conn, n.StableID) // agent protocol only, never the admin protocol
}
```

#### Admin-Socket

- **Folge:** Der Admin-Socket bleibt ein Unix-Socket. Der Listener von `tsnet` bedient nur das Agent-Protokoll. Der Code für den Admin-Socket hängt an keinem Netz-Listener.
- **Folge:** Der Daemon startet `tailscale serve` nicht und richtet keinen TCP-Forward ein. Damit führt kein Weg aus dem Tailnet zum Admin-Socket.
- **Annahme:** Ein Fehler im Agent-Protokoll bleibt das größte Risiko für den Admin-Socket. Getrennte Listener und getrennte Handler begrenzen dieses Risiko.

### Betrieb der VMs

#### Beitritt zum Tailnet

- **Doku:** Ein Auth-Key meldet einen Knoten ohne Browser an. Ein Auth-Key ist einmalig oder mehrfach nutzbar, optional kurzlebig (ephemeral), optional vorab genehmigt und optional mit Tags versehen ([Auth keys](https://tailscale.com/kb/1085/auth-keys)).
- **Doku:** Ein Auth-Key läuft nach 1 bis 90 Tagen ab. Ein einmaliger Auth-Key ist nach der ersten Nutzung ungültig. Der Ablauf und der Widerruf eines Auth-Keys entfernen keinen Knoten (gleiche Seite).
- **Doku:** Tailscale warnt vor mehrfach nutzbaren Auth-Keys: „These can be very dangerous if stolen“ (gleiche Seite).
- **Doku:** Ein Knoten mit einem Auth-Key mit Tag übernimmt die Tags des Auth-Keys. Der Knoten kann die Tags nicht selbst entfernen: „You cannot remove tags using the `--advertise-tags` flag if the device uses an auth key“ ([Tags](https://tailscale.com/kb/1068/tags)).
- **Doku:** `tailscale up --auth-key=file:<pfad>` liest den Auth-Key aus einer Datei ([Befehl `tailscale up`](https://tailscale.com/docs/reference/tailscale-cli/up)).
- **Doku:** Ein OAuth-Client mit dem Recht `auth_keys` erzeugt Auth-Keys über die API. Der OAuth-Client gehört dem Tailnet und ist an Tags gebunden ([OAuth clients](https://tailscale.com/kb/1215/oauth-clients)).
- **Doku:** Workload Identity Federation meldet einen Knoten mit einem OIDC-Token an (`tailscale up --client-id --id-token`) ([Workload identity federation](https://tailscale.com/docs/features/workload-identity-federation)). Für lokale VMs fehlt der Aussteller eines solchen Tokens.

Der Spike schlägt diese Regeln vor:

- Jede VM erhält einen eigenen, einmaligen Auth-Key mit `tag:agent-vm`, vorab genehmigt, mit einem Tag Laufzeit.
- Der Besitzer erzeugt den Auth-Key auf dem Mac. Der Auth-Key liegt nie im Abbild einer VM und nie in einem Repository.
- Die Einrichtung der VM nutzt den Auth-Key, bevor ein Agent startet. Die Einrichtung löscht die Datei danach.
- Ein mehrfach nutzbarer Auth-Key und ein OAuth-Secret liegen nie in einer VM. Wer einen solchen Schlüssel hält, legt von jedem Ort aus neue Knoten mit `tag:agent-vm` an.
- Die Liste der erlaubten Knoten im Daemon fängt einen gestohlenen Auth-Key ab: Ein neuer Knoten steht nicht in der Liste.

#### Kurzlebige Knoten

- **Doku:** Tailscale entfernt einen kurzlebigen Knoten nach einer kurzen Zeit ohne Verbindung. Die Dokumentation nennt 30 bis 60 Minuten. `tailscale logout` entfernt den Knoten sofort ([Ephemeral nodes](https://tailscale.com/kb/1111/ephemeral-nodes)).
- **Doku:** Ein neu angelegter kurzlebiger Knoten erhält eine neue Adresse (gleiche Seite).
- **Folge:** Kurzlebige Knoten passen zu VMs, die für eine Aufgabe entstehen und danach verschwinden. Verwaiste Knoten räumt Tailscale selbst auf.
- **Annahme:** Schläft der Mac länger als eine Stunde, entfernt Tailscale den kurzlebigen Knoten einer lokalen VM. Offen ist, ob die VM danach mit dem alten Zustand wieder beitritt oder einen neuen Auth-Key braucht.
- **Folge:** Für eine VM, die Tage lebt, ist ein Knoten ohne die Eigenschaft „kurzlebig“ die sichere Wahl, bis der Test die Annahme klärt.

#### Anlegen, Klonen, Löschen

- **Doku:** Ein Klon mit kopiertem Zustand (`/var/lib/tailscale/tailscaled.state`) nutzt denselben Knotenschlüssel. Beide Maschinen streiten um dieselbe Adresse. Die Lösung ist, den Zustand auf einer der Maschinen zu löschen ([Issue 506 von Tailscale](https://github.com/tailscale/tailscale/issues/506)).
- **Folge:** Ein Klon mit kopiertem Zustand hat für den Daemon dieselbe Identität wie das Original. Der Daemon unterscheidet beide nicht.
- **Folge:** Das Abbild für neue VMs enthält Tailscale, aber keinen Zustand. `tailscale up` läuft erst in der fertigen VM.
- **Doku:** Ein Knoten mit Tag hat keinen Schlüsselablauf ([Key expiry](https://tailscale.com/kb/1028/key-expiry)).
- **Folge:** Der Knotenschlüssel einer VM ohne die Eigenschaft „kurzlebig“ bleibt gültig, bis jemand den Knoten löscht. Ein altes Abbild oder ein Snapshot einer solchen VM ist ein gültiger Zugang.
- **Folge:** Zum Löschen einer VM gehören drei Schritte: `tailscale logout` in der VM oder Löschen des Knotens in der Admin-Konsole, Entfernen der Knoten-ID aus der Liste des Daemons, Löschen des Abbilds und der Snapshots.

#### Was ein Agent mit der Identität der VM erreicht

- **Doku:** Der Knoten erreicht genau das, was die Policy der Quelle `tag:agent-vm` erlaubt ([Syntax der Grants](https://tailscale.com/docs/reference/syntax/grants)).
- **Doku:** Der Knoten sieht nur Knoten, zu denen eine Regel besteht ([Device visibility](https://tailscale.com/docs/concepts/device-visibility)).
- **Folge:** Mit der vorgeschlagenen Policy sieht eine VM nur den Knoten des Daemons. Die VM sieht weder den Mac noch iPhone und iPad noch andere VMs.
- **Folge:** Erlaubt die Policy dem Besitzer den Zugriff auf die VMs, sieht jede VM die Geräte des Besitzers mit Namen und Adresse. Die VM erreicht diese Geräte trotzdem nicht.
- **Annahme:** Ein Agent mit Root-Recht in der VM kann Folgendes:
    - den Knotenschlüssel aus `/var/lib/tailscale/tailscaled.state` kopieren und den Knoten von einem anderen Rechner aus betreiben,
    - den Namen des Knotens ändern,
    - den Schutz gegen eingehende Verbindungen (`--shields-up`) abschalten,
    - `tailscale serve` in der VM starten,
    - direkt mit dem Agent-Endpunkt sprechen und `a38 mcp` umgehen.
- **Doku:** Der Agent kann die Tags der VM nicht ändern ([Tags](https://tailscale.com/kb/1068/tags)).
- **Annahme:** Funnel, Exit-Nodes und Tailscale SSH brauchen eigene Einträge in der Policy. Die vorgeschlagene Policy enthält keinen solchen Eintrag für `tag:agent-vm`.
- **Folge:** Der Agent-Endpunkt behandelt jede Eingabe aus der VM als feindlich. Das gilt auch für den Unix-Socket und ist keine neue Anforderung.

#### Mac schläft, Tailnet nicht erreichbar

- **Annahme:** Schläft der Mac, antwortet der Knoten des Daemons nicht. Eine VM auf demselben Mac schläft mit. Eine VM auf einem anderen Rechner erhält eine Zeitüberschreitung.
- **Doku:** Für macOS gibt es Fehlerberichte zu Verbindungen nach dem Aufwachen, zum Beispiel etwa 15 Sekunden ohne Verkehr zu einem Knoten ([Issue 21650 von Tailscale](https://github.com/tailscale/tailscale/issues/21650)). Die Berichte betreffen den Tailscale-Client des Mac. Offen ist, ob sie auch für `tsnet` gelten.
- **Annahme:** Ohne Verbindung zum Koordinationsserver arbeiten bestehende Knoten für einige Zeit mit der letzten Netzkarte weiter. Ein neuer Knoten tritt in dieser Zeit nicht bei.
- **Folge:** Eine Freigabe braucht Touch ID am Mac. Ein schlafender Mac bedeutet daher keinen zusätzlichen Ausfall: Ohne den Menschen am Mac gibt es ohnehin keine Freigabe.
- **Folge:** `a38 mcp` braucht eine Zeitgrenze für den Verbindungsaufbau und einen erneuten Versuch mit Wartezeit.

#### Folgen für `a38 mcp` in der VM

- Die Adresse des Agent-Endpunkts ist konfigurierbar, zum Beispiel `tcp://a38-host:8438`. Ohne Konfiguration nutzt `a38 mcp` weiter den Unix-Socket.
- **Annahme:** Der Name `a38-host` löst sich über MagicDNS auf, wenn die VM die DNS-Einstellungen des Tailnets annimmt. Sonst nutzt die Konfiguration die Tailscale-Adresse des Knotens.
- **Doku:** Verbindungen zwischen Knoten sind Ende-zu-Ende verschlüsselt ([HTTPS aktivieren](https://tailscale.com/kb/1153/enabling-https)). TLS im Agent-Protokoll ist nicht nötig.
- **Doku:** Ein TLS-Zertifikat von Tailscale veröffentlicht den Namen des Knotens im öffentlichen Certificate-Transparency-Protokoll (gleiche Seite). Der Spike rät von TLS mit `ListenTLS` ab.
- Das Agent-Protokoll bleibt auf beiden Wegen gleich. Nur der Transport unterscheidet sich.
- `a38 mcp` unterscheidet diese Fehler:
    - Der Agent-Endpunkt ist nicht erreichbar (Tailscale in der VM ist aus, der Mac schläft, die Policy sperrt). TCP meldet in allen drei Fällen nur eine Zeitüberschreitung.
    - Der Daemon lehnt den Knoten ab (Tag fehlt, Knoten steht nicht in der Liste). Der Daemon sendet einen Fehler und schließt die Verbindung.
    - Der Daemon lehnt die Anfrage ab (bestehende Fehlercodes).
- Die VM enthält kein Secret von Passierschein. Der einzige Schlüssel in der VM ist der Knotenschlüssel von Tailscale.

## Vorschlag für die Policy

Der Vorschlag nutzt neutrale Namen. Der Port `8438` ist ein Beispiel.

```json
{
  "tagOwners": {
    // Only admins assign these tags.
    "tag:a38-host": ["autogroup:admin"],
    "tag:agent-vm": ["autogroup:admin"],
  },

  "grants": [
    // Existing rule for the own devices (assumed, unchanged).
    { "src": ["autogroup:member"], "dst": ["autogroup:self"], "ip": ["*"] },

    // New: the VMs reach the daemon node on one TCP port, nothing else.
    { "src": ["tag:agent-vm"], "dst": ["tag:a38-host"], "ip": ["tcp:8438"] },
  ],

  "tests": [
    {
      "src": "tag:agent-vm",
      "proto": "tcp",
      "accept": ["tag:a38-host:8438"],
      "deny": ["tag:a38-host:22", "tag:a38-host:443", "tag:agent-vm:22"],
    },
    {
      // The own devices do not reach the agent endpoint.
      "src": "besitzer@example.com",
      "proto": "tcp",
      "deny": ["tag:a38-host:8438"],
    },
  ],
}
```

Regeln zum Vorschlag:

- Die Policy enthält keinen Grant mit `tag:agent-vm` als Ziel. Keine VM erreicht eine andere VM. Kein Gerät erreicht eine VM über Tailscale.
- Die Policy enthält keinen Grant mit `tag:a38-host` als Quelle. Der Knoten des Daemons öffnet keine Verbindung.
- Die Policy enthält keine Regel mit der Quelle `*` oder `autogroup:tagged`.
- Der Abschnitt `ssh`, der Abschnitt `nodeAttrs` und der Abschnitt `autoApprovers` nennen `tag:agent-vm` nicht.
- Braucht der Besitzer SSH in die VMs, kommt ein eigener Grant dazu: `{ "src": ["besitzer@example.com"], "dst": ["tag:agent-vm"], "ip": ["tcp:22"] }`. Die VMs sehen dann die Geräte des Besitzers in ihrer Netzkarte.
- **Annahme:** Der Abschnitt `tests` nimmt einen Tag mit Port als Ziel an. Die Dokumentation zeigt nur Hostnamen und Adressen. Die Admin-Konsole meldet einen Fehler beim Speichern, falls das nicht gilt.

Variante für Form B ohne eigenen Knoten:

```json
{
  "hosts": { "a38-mac": "100.64.0.10" },
  "grants": [
    { "src": ["tag:agent-vm"], "dst": ["a38-mac"], "ip": ["tcp:8438"] },
  ],
}
```

Die Adresse `100.64.0.10` ist ein Beispiel. Bei dieser Variante erreichen iPhone und iPad den Port über die bestehende Regel.

Optionale zweite Sperre in der Policy:

- **Doku:** Ein Grant vergibt mit dem Feld `app` ein Anwendungsrecht. Die Anwendung liest das Recht über die lokale API oder über `tsnet` ([Application capabilities](https://tailscale.com/docs/features/access-control/grants/grants-app-capabilities)). Das Recht steht in `WhoIsResponse.CapMap`.
- **Folge:** Der Daemon kann zusätzlich ein eigenes Anwendungsrecht verlangen, zum Beispiel `example.com/cap/a38-agent`. Eine zu weite Regel auf Netzebene genügt dann nicht.
- **Annahme:** Ein Grant mit einem Tag als Quelle vergibt ein Anwendungsrecht. Die Dokumentation zeigt nur Benutzer und Gruppen als Quelle.

## Prüfungen im Daemon

Der Daemon prüft unabhängig von der Policy:

1. Der Daemon nimmt Verbindungen aus dem Tailnet nur über den Listener von `tsnet` an. Der Daemon öffnet keinen TCP-Listener im Betriebssystem.
2. Der Daemon ruft für jede Verbindung `WhoIs` mit der Quelladresse auf. Bei einem Fehler schließt der Daemon die Verbindung.
3. Der Knoten hat Tags (`Node.IsTagged()`), und `Node.Tags` enthält den konfigurierten Tag. Ein Knoten mit Benutzeridentität ist immer abgelehnt. Das gilt auch für den Mac, das iPhone und das iPad.
4. `Node.Sharer` ist leer. Ein geteilter Knoten aus einem fremden Tailnet ist abgelehnt.
5. `Node.StableID` steht in der Liste der erlaubten Knoten in der Konfiguration des Daemons. Der Besitzer trägt eine neue VM über den Admin-Socket ein. Die Liste ordnet jeder Knoten-ID einen Namen zu.
6. Optional: `CapMap` enthält das Anwendungsrecht.
7. Der Daemon wertet `UserProfile`, den Namen des Knotens und `Node.Hostinfo` nicht für eine Entscheidung aus.
8. Der Listener von `tsnet` bedient nur das Agent-Protokoll. Der Admin-Socket bleibt ein Unix-Socket.
9. Der Daemon begrenzt die Zahl der Verbindungen und der Anfragen je Knoten.
10. Das Audit-Log enthält je Sitzung die Knoten-ID, den Namen aus der Liste, die Tags und die Tailscale-Adresse.
11. Beim Start prüft der Daemon den eigenen Knoten: Der Knoten trägt `tag:a38-host`. Fehlt der Tag, startet der Listener nicht.

Verhalten bei einer fehlerhaften Policy:

| Fehler in der Policy | Folge im Netz | Folge im Daemon |
|---|---|---|
| Eine Regel erlaubt den eigenen Geräten den Port. | iPhone, iPad und Mac öffnen eine TCP-Verbindung. | Prüfung 3 lehnt ab. |
| Eine Regel erlaubt allen Knoten mit Tag den Port. | Fremde Knoten mit Tag öffnen eine TCP-Verbindung. | Prüfung 3 und Prüfung 5 lehnen ab. |
| Eine Regel erlaubt `tag:agent-vm` weitere Ziele. | Die VMs erreichen andere Geräte. | Der Daemon bemerkt das nicht. Nur der Abschnitt `tests` der Policy fängt diesen Fehler ab. |
| Ein Fremder erhält `tag:agent-vm` (gestohlener Auth-Key, falscher Eintrag in `tagOwners`). | Der fremde Knoten erreicht den Port. | Prüfung 5 lehnt ab. |
| Die Regel für die VMs fehlt. | Keine VM erreicht den Port. | Kein Risiko. `a38 mcp` meldet eine Zeitüberschreitung. |

## Folgen für den Betrieb der VMs

- Jede VM ist ein eigener Knoten mit `tag:agent-vm`.
- Jede VM erhält einen eigenen, einmaligen Auth-Key mit Tag und kurzer Laufzeit. Der Besitzer erzeugt den Auth-Key.
- Der Auth-Key liegt nie im Abbild, nie in einem Repository und nach der Einrichtung nicht mehr in der VM.
- Das Abbild enthält keinen Zustand von Tailscale. Ein Klon tritt mit einem eigenen Auth-Key bei.
- Der Besitzer trägt die Knoten-ID jeder neuen VM in die Liste des Daemons ein.
- Die VM startet Tailscale ohne Tailscale SSH und mit `--shields-up`. Die Policy bleibt die eigentliche Sperre, weil ein Agent mit Root-Recht die Einstellung ändert.
- Kurzlebige Knoten nur für VMs, die höchstens für eine Aufgabe leben.
- Ein Knoten mit Tag läuft nicht ab. Der Besitzer löscht den Knoten, die Knoten-ID in der Liste und alle Abbilder, wenn er eine VM löscht.
- Ein Snapshot einer VM enthält den Knotenschlüssel. Ein Snapshot ist wie ein Zugang zu behandeln.
- Der Agent in der VM erreicht mit der Identität der VM nur den Agent-Endpunkt. Der Agent kann den Knotenschlüssel kopieren. Der Daemon erkennt das nicht.
- Schläft der Mac, gibt es keinen Agent-Endpunkt und keine Freigabe. `a38 mcp` wartet und versucht es erneut.

## Vergleich

| Weg | Identität der VM | Zugriffsbeschränkung | Aufwand im Betrieb |
|---|---|---|---|
| Tailscale mit `tsnet` (Form C) | Knoten-ID und Tag, vom Koordinationsserver bestätigt, an den Knotenschlüssel in der VM gebunden. | Policy mit Tag als Quelle und Ziel, ein Port. Zweite Sperre im Daemon. Kein Port auf dem Mac. | Mittel. Auth-Key je VM, Eintrag in der Liste, Pflege der Policy. Kein Dienst, der eine Verbindung hält. Funktioniert auch für VMs auf anderen Rechnern. Abhängig vom Dienst Tailscale. |
| Tailscale mit TCP-Listener (Form B) | Wie Form C, aber abhängig von der Variante des Clients auf dem Mac. | Policy mit der Adresse des Mac als Ziel. Der Port ist für lokale Prozesse und für die eigenen Geräte offen. | Mittel. Zusätzlich: Listener neu binden, Zugang zur lokalen API. |
| SSH-`RemoteForward` | Der Mac baut die Verbindung auf. Ein eigener Socket je VM auf dem Mac macht die VM eindeutig. Keine dritte Partei. | Der Socket erscheint nur in der VM. Nichts lauscht im Netz. Der Socket in der VM gehört einem Benutzer (Modus `0600` nach der Vorgabe `StreamLocalBindMask`). | Hoch. Der Mac hält je VM eine SSH-Verbindung. Neuaufbau nach Schlaf und Netzwechsel. Alter Socket in der VM (`StreamLocalBindUnlink`). SSH-Schlüssel des Mac für jede VM. |
| Eigener Port-Forward | Nur die Quelladresse im Netz der VM. Mehrere VMs im selben Netz fälschen die Adresse. | Bindung an eine Adresse und Paketfilter auf dem Mac. Ein TCP-Port ist für lokale Prozesse offen. | Niedrig am Anfang. Danach je VM-Werkzeug eigene Regeln. Keine zentrale Policy. |
| Host-Anbindung der VM | Hängt vom VM-Werkzeug ab. Ein Kanal des Hypervisors je VM ist eindeutig. Ein gemeinsames Netz liefert nur eine Adresse. | Hängt vom VM-Werkzeug ab. Ein gemeinsames Netz öffnet der VM oft weitere Ports des Mac. | Niedrig bis mittel. Bindet das Projekt an ein VM-Werkzeug. Geht nur für VMs auf dem Mac. |

- **Doku:** `RemoteForward` leitet einen Unix-Socket der entfernten Maschine an einen Unix-Socket der lokalen Maschine weiter. `StreamLocalBindMask` hat die Vorgabe `0177`. `StreamLocalBindUnlink` entfernt einen alten Socket ([ssh_config](https://man.openbsd.org/ssh_config)).
- **Annahme:** Die Zeilen „Eigener Port-Forward“ und „Host-Anbindung der VM“ sind Ableitungen. Der Spike hat kein VM-Werkzeug untersucht.

## Empfehlung

Das Projekt wählt Tailscale mit einem eigenen Knoten im Daemon (Form C).

Gründe:

- Form C liefert eine Identität je VM, die der Daemon selbst abfragt und die nicht vom Agent stammt.
- Form C trennt den Agent-Endpunkt vom Mac. Die VMs erreichen keinen Port des Mac. Die bestehende Regel für die eigenen Geräte bleibt unberührt.
- Form C öffnet keinen Listener im Betriebssystem. Die Frage nach lokalen Prozessen entfällt.
- Form C hängt nicht von der Variante des Tailscale-Clients auf dem Mac ab.
- Form A scheidet aus, weil die Identitäts-Header für Knoten mit Tag leer sind.
- Form B ist die Rückfallebene, falls `tsnet` im praktischen Test scheitert oder das Binary zu stark wächst.
- SSH-`RemoteForward` mit einem Socket je VM ist die Rückfallebene ohne Tailscale. Dieser Weg hat die stärkste Trennung, aber den höchsten Aufwand im Betrieb.

### Einträge für den Design-Lauf

Entscheidungen:

- Der Transport in die VM ist Tailscale. Der Daemon betreibt dafür einen eigenen Knoten mit `tsnet` und dem Tag `tag:a38-host`.
- Der Agent-Endpunkt für VMs ist optional. Ohne Konfiguration startet der Daemon keinen Knoten.
- Die VM ist die Sitzungsgrenze. Geprüfte Fakten sind die Knoten-ID, der Tag und der Name aus der Liste des Daemons.
- Der Daemon prüft Tag und Knoten-ID selbst. Die Policy allein genügt nicht.
- Der Admin-Socket hat keinen Netzzugang.
- Das Agent-Protokoll ist über beide Transporte gleich. TLS entfällt.
- Der Daemon liest kein Secret für Tailscale aus dem Passwortmanager. Der Besitzer meldet den Knoten des Daemons einmal im Browser an.

Lücken:

- Der Befehl, mit dem der Besitzer eine VM in die Liste einträgt, fehlt.
- Offen ist, wie der Besitzer die Knoten-ID einer neuen VM sicher erfährt. Ein Weg: Der Daemon zeigt den ersten Verbindungsversuch eines unbekannten Knoten mit `tag:agent-vm` an, und der Besitzer bestätigt ihn.
- Offen ist, wer den Auth-Key je VM erzeugt: der Besitzer von Hand oder ein Werkzeug mit einem OAuth-Client.
- Offen ist, ob Tailnet Lock im Tailnet aktiv sein soll.
- Die Sicherheitsgrenze zwischen VM und Daemon braucht einen Eintrag für den Knotenschlüssel in der VM.

## Risiken

- Ein Agent mit Root-Recht kopiert den Knotenschlüssel der VM. Der Knoten läuft dann auf einem fremden Rechner weiter. Der Daemon erkennt das nicht. Die Policy begrenzt den Schaden auf den Agent-Endpunkt. Jede Auslieferung braucht weiterhin eine Freigabe.
- Ein Knoten mit Tag läuft nicht ab. Ein vergessener Knoten oder ein alter Snapshot bleibt ein gültiger Zugang zum Agent-Endpunkt.
- Ein gestohlener, mehrfach nutzbarer Auth-Key oder ein OAuth-Secret erlaubt neue Knoten mit `tag:agent-vm`. Die Liste im Daemon fängt das ab, die Policy nicht.
- Eine spätere Änderung der Policy öffnet den VMs unbemerkt weitere Ziele. Nur der Abschnitt `tests` der Policy schützt davor.
- Die Identität hängt am Koordinationsserver von Tailscale. Ohne Tailnet Lock kann der Koordinationsserver einen Knoten einfügen.
- Der Dienst Tailscale fällt aus oder ist nicht erreichbar. Neue VMs treten dann nicht bei.
- `tsnet` bringt viele Abhängigkeiten in das Binary. Das vergrößert die Fläche für Schwachstellen und den Aufwand für `govulncheck`.
- Der Zustand von `tsnet` auf dem Mac enthält einen privaten Schlüssel in einer Datei.
- Der Name `a38-host` ist im Tailnet sichtbar für jeden Knoten, der eine Regel zu diesem Knoten hat.
- Das Verhalten nach dem Schlaf des Mac ist nicht geprüft. Fehlerberichte für den Tailscale-Client auf macOS deuten auf Wartezeiten hin.
- Die Regel „eine VM ist eine Sitzung“ führt zu breiten Freigaben, wenn viele Agents in einer VM laufen.

## Testplan

Der Test findet nur nach Rücksprache mit dem Besitzer statt. Der Test nutzt eine Test-VM, einen Wegwerf-Listener ohne Secrets und neutrale Namen. Der Test ändert die Policy. Der Besitzer führt die Schritte 1, 2 und 17 selbst aus.

1. Der Besitzer sichert die aktuelle Policy und ergänzt `tagOwners`, den Grant und die `tests` aus dem [Vorschlag](#vorschlag-fur-die-policy). Erwartung: Die Admin-Konsole speichert die Policy ohne Fehler. Der Schritt klärt, ob `tests` einen Tag als Ziel annimmt.
2. Der Besitzer erzeugt in der Admin-Konsole einen einmaligen, vorab genehmigten Auth-Key mit `tag:agent-vm` und einem Tag Laufzeit.
3. Ein Wegwerfprogramm unter `spikes/tailscale-whois/` startet einen Knoten mit `tsnet` nach der [Skizze](#form-c-eigener-knoten-mit-tsnet-im-daemon) und gibt je Verbindung die Antwort von `WhoIs` als JSON aus:

    ```sh
    go run ./spikes/tailscale-whois -hostname a38-host-test -tag tag:a38-host -addr :8438
    ```

    Erwartung: Der erste Start zeigt eine Anmelde-URL. Nach der Anmeldung trägt der Knoten `tag:a38-host`.
4. Auf dem Mac prüfen, dass kein Listener im Betriebssystem entsteht:

    ```sh
    lsof -nP -iTCP:8438 -sTCP:LISTEN
    ```

    Erwartung: keine Ausgabe.
5. Die Test-VM tritt bei:

    ```sh
    curl -fsSL https://tailscale.com/install.sh | sh
    sudo tailscale up --auth-key=file:/run/a38-test/authkey --hostname=agent-vm-1 --shields-up
    sudo rm /run/a38-test/authkey
    tailscale status
    ```

    Erwartung: `tailscale status` zeigt nur `a38-host-test`.
6. Verbindung aus der VM:

    ```sh
    nc -vz -w 5 a38-host-test 8438
    ```

    Erwartung: Die Verbindung gelingt. Das Wegwerfprogramm zeigt `Node.Tags` mit `tag:agent-vm`. Festhalten: `Node.StableID`, `Node.Name`, `Node.User`, `Node.Sharer`, Inhalt von `UserProfile`, Inhalt von `CapMap`.
7. Gesperrte Ziele aus der VM:

    ```sh
    nc -vz -w 5 a38-host-test 22
    nc -vz -w 5 a38-host-test 443
    tailscale ping --c 3 a38-host-test
    nc -vz -w 5 <name-des-mac> 22
    ```

    Erwartung: Jede TCP-Verbindung läuft in die Zeitüberschreitung. Der Name des Mac löst sich nicht auf.
8. Verbindung vom Mac als lokaler Prozess:

    ```sh
    nc -vz -w 5 a38-host-test 8438
    ```

    Erwartung: Zeitüberschreitung, weil kein Grant vom Benutzer zu `tag:a38-host` führt.
9. Verbindung von einem eigenen Gerät (iPhone oder iPad) auf `a38-host-test:8438`. Erwartung: Zeitüberschreitung.
10. Gegenprobe zur zweiten Sperre: Der Besitzer ergänzt vorübergehend den Grant `{ "src": ["autogroup:member"], "dst": ["tag:a38-host"], "ip": ["tcp:8438"] }` und entfernt den zugehörigen `deny`-Test. Schritt 8 wiederholen. Erwartung: Die TCP-Verbindung gelingt, `WhoIs` zeigt einen Knoten ohne Tags, das Wegwerfprogramm schließt die Verbindung. Danach den Grant wieder entfernen.
11. Name fälschen in der VM:

    ```sh
    sudo tailscale set --hostname=a38-host
    nc -vz -w 5 a38-host-test 8438
    ```

    Erwartung: `Node.StableID` und `Node.Tags` bleiben gleich. Festhalten, welche Felder sich ändern.
12. Neustart und neuer Beitritt: Die VM neu starten und Schritt 6 wiederholen. Danach `sudo tailscale logout`, mit einem neuen Auth-Key beitreten und Schritt 6 wiederholen. Festhalten, wann sich `Node.StableID` und die Adresse ändern.
13. Klon: Die VM mit dem Zustand von Tailscale klonen und beide starten. Festhalten: Verhalten beider Knoten, Anzeige in der Admin-Konsole, Antwort von `WhoIs`. Danach im Klon den Zustand löschen und mit einem eigenen Auth-Key beitreten:

    ```sh
    sudo systemctl stop tailscaled
    sudo rm /var/lib/tailscale/tailscaled.state
    sudo systemctl start tailscaled
    ```

14. Pfad und Ausfall: In der VM `tailscale ping a38-host-test` aufrufen und festhalten, ob die Verbindung direkt oder über einen Relay-Server läuft. Die Internetverbindung des Mac trennen und Schritt 6 wiederholen.
15. Schlaf: Den Mac 10 Minuten schlafen lassen. Nach dem Aufwachen die Zeit bis zur ersten erfolgreichen Verbindung messen:

    ```sh
    time nc -vz -w 30 a38-host-test 8438
    ```

16. Kurzlebiger Knoten: Eine zweite VM mit einem kurzlebigen Auth-Key anlegen, die VM 90 Minuten anhalten und fortsetzen. Festhalten, ob der Knoten wieder beitritt. Optional: den Grant um ein Feld `app` ergänzen und prüfen, ob `CapMap` das Anwendungsrecht für einen Knoten mit Tag enthält. Optional für Form B: `tailscale whois <adresse-der-vm>` auf dem Mac unter einem Konto mit und ohne Admin-Recht aufrufen.
17. Aufräumen: Der Besitzer löscht die Test-Knoten in der Admin-Konsole, widerruft unbenutzte Auth-Keys und stellt die gesicherte Policy wieder her. Danach das Zustandsverzeichnis des Wegwerfprogramms und die Test-VMs löschen.

## Offene Punkte

Diese Punkte klärt nur der praktische Test:

- der Inhalt von `UserProfile` und `Node.User` für einen Knoten mit Tag,
- die Stabilität von `Node.StableID` über Neustart, Schlaf und neuen Beitritt,
- die Felder, die ein Agent mit Root-Recht in der VM ändert,
- die Antwort von `WhoIs` auf eine lokale Verbindung vom Mac,
- die Erreichbarkeit des Knotens von `tsnet` für lokale Prozesse des Mac ohne Grant,
- der Pfad zwischen einer lokalen VM und dem Knoten des Daemons (direkt oder Relay),
- das Verhalten nach dem Schlaf des Mac und ohne Internetverbindung,
- das Verhalten kurzlebiger Knoten nach einer langen Pause,
- die Größe des Binarys und die Zahl der Abhängigkeiten mit `tsnet`,
- ein Anwendungsrecht für einen Knoten mit Tag,
- ein Tag als Ziel im Abschnitt `tests` der Policy.

Diese Punkte hat der Spike nicht geprüft:

- den Inhalt der aktuellen Policy,
- die Wahl des VM-Werkzeugs und dessen Netz,
- den Zustand von Tailnet Lock im Tailnet.

## Quellen

Tailscale, Dokumentation:

- [Tailscale identity](https://tailscale.com/docs/concepts/tailscale-identity)
- [Device visibility](https://tailscale.com/docs/concepts/device-visibility)
- [Tags](https://tailscale.com/kb/1068/tags)
- [Grants](https://tailscale.com/kb/1324/grants)
- [Syntax der Grants](https://tailscale.com/docs/reference/syntax/grants)
- [Syntax der Policy](https://tailscale.com/docs/reference/syntax/policy-file)
- [Application capabilities](https://tailscale.com/docs/features/access-control/grants/grants-app-capabilities)
- [Tailscale Serve](https://tailscale.com/kb/1312/serve)
- [Befehl `tailscale serve`](https://tailscale.com/kb/1242/tailscale-serve)
- [Befehl `tailscale up`](https://tailscale.com/docs/reference/tailscale-cli/up)
- [macOS-Varianten](https://tailscale.com/kb/1065/macos-variants)
- [tsnet](https://tailscale.com/kb/1244/tsnet)
- [Auth keys](https://tailscale.com/kb/1085/auth-keys)
- [Ephemeral nodes](https://tailscale.com/kb/1111/ephemeral-nodes)
- [Key expiry](https://tailscale.com/kb/1028/key-expiry)
- [OAuth clients](https://tailscale.com/kb/1215/oauth-clients)
- [Workload identity federation](https://tailscale.com/docs/features/workload-identity-federation)
- [Tailnet Lock](https://tailscale.com/kb/1226/tailnet-lock)
- [Tailscale Services](https://tailscale.com/kb/1552/tailscale-services)
- [Machine names](https://tailscale.com/kb/1098/machine-names)
- [IP-Adressen](https://tailscale.com/kb/1033/ip-and-dns-addresses)
- [HTTPS aktivieren](https://tailscale.com/kb/1153/enabling-https)
- [Client preferences](https://tailscale.com/kb/1072/client-preferences)

Tailscale, Go-Pakete und Quelltext (v1.104.6):

- [Paket `client/local`](https://pkg.go.dev/tailscale.com/client/local)
- [Paket `apitype`](https://pkg.go.dev/tailscale.com/client/tailscale/apitype)
- [Paket `tsnet`](https://pkg.go.dev/tailscale.com/tsnet)
- [tailcfg.go](https://github.com/tailscale/tailscale/blob/v1.104.6/tailcfg/tailcfg.go)
- [localapi.go](https://github.com/tailscale/tailscale/blob/v1.104.6/ipn/localapi/localapi.go)
- [local.go](https://github.com/tailscale/tailscale/blob/v1.104.6/ipn/ipnlocal/local.go)
- [serve.go](https://github.com/tailscale/tailscale/blob/v1.104.6/ipn/ipnlocal/serve.go)
- [safesocket_darwin.go](https://github.com/tailscale/tailscale/blob/v1.104.6/safesocket/safesocket_darwin.go)
- [paths.go](https://github.com/tailscale/tailscale/blob/v1.104.6/paths/paths.go)
- [tshello.go](https://github.com/tailscale/tailscale/blob/v1.104.6/tsnet/example/tshello/tshello.go)
- [Issue 506: cloned tailscaled.state files](https://github.com/tailscale/tailscale/issues/506)
- [Issue 21650: no traffic after wake from sleep](https://github.com/tailscale/tailscale/issues/21650)

Andere:

- [ssh_config(5)](https://man.openbsd.org/ssh_config)
