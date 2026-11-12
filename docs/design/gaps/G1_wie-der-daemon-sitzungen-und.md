---
render_macros: true
id: 'G1'
titel: 'Wie der Daemon Sitzungen und VMs erkennt und welcher Transport in die VM führt, ist offen'
bereich: 'Grenzen und Identität'
lauf: '2.0'
stand: '2026-11-12'
order: 1
headless: true
type: Gap
kind: design
status: vertagt
beleg:
  wer: Koordinator
  fuer: Stefan
  datum: '2026-11-12'
  quelle: ['#11', '#58']
  notiz: Stufe 5 (Transport in die VM); wartet auf die Entscheidung über die Empfehlung des Spikes 40 (Tailscale) und den Praxistest; nichts in den Stufen 1 bis 3 hängt davon ab (D23, D24)
register:
  owner: Stefan
---
# G1 — Wie der Daemon Sitzungen und VMs erkennt und welcher Transport in die VM führt, ist offen

{{ design_kopf(page) }}

Über SSH-`RemoteForward` kommt jede Verbindung vom SSH-Prozess auf dem Mac. Die Peer-Daten nennen dann SSH und nicht den Agent. Bei einem Netzwerk-Transport gibt es keine Peer-Daten eines Unix-Sockets. Ohne eine verlässliche Kennung gilt ein Grant für jeden Prozess in der VM. Das betrifft die Stufe 5.

SSH ist nicht zwingend (Stefan, 2026-11-11). Optionen für den Transport in die VM:

- SSH-`RemoteForward` auf den Agent-Socket, wie in {{ ref("D3") }} beschrieben.
- Ein eigener Port-Forward je VM.
- Eine Freigabe des Diensts über Tailscale. Der Daemon kennt dann die Tailscale-Identität der VM.
- Die Host-Anbindung der VM.

Jede Option außer dem Unix-Socket braucht eine Antwort darauf, wer den Endpunkt erreichen darf.

**Spike:** Der [Bericht des Spikes](../../explanation/spikes/40-tailscale-vm-zugang.md) empfiehlt einen eigenen Tailscale-Knoten im Daemon (`tsnet`) mit einem Tag, VMs mit einem zweiten Tag und eine Regel der Policy für genau einen Port. Die VM ist dann die Sitzung. Der Praxistest und die Entscheidung von Stefan stehen aus.

**Bevorzugt:** Tailscale (Stefan, 2026-11-11). Tailscale liefert stabile Identitäten und ist im Testumfeld im Einsatz. Die Policy des Tailnets soll den Zugriff so beschränken, dass nur die eigenen VMs den Endpunkt auf dem Mac erreichen. Ein Spike prüft das und die Folgen für den Betrieb der VMs ([Issue 11](https://github.com/seletz/passierschein/issues/11)).

Offene Fragen:

- Welche Option gibt dem Daemon eine verlässliche Kennung der VM?
- Reicht eine VM je Projekt, sodass die VM die Sitzung ist?
- Kann der Daemon je VM einen eigenen Socket-Pfad anlegen und so die VM erkennen?
- Was zeigt die Freigabeanfrage als geprüfte Fakten?

**Quelle:** kritische Prüfung, Schwachstelle zur Identität; Projektnotiz, Ablauf Schritt 3.
