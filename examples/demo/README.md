# Demo-Kit von a38

Dieser Ordner enthält das Material der Demo.
Den Ablauf beschreibt [Demo vorführen](../../docs/how-to/50-demo-vorfuehren.md).

- `demo-projekt/`: das Demo-Projekt für den Agent. `demo:setup` kopiert es in das Demo-Verzeichnis.
- `requests/`: die Nachrichten der Akte für den Weg ohne Agent (`mise run demo:request -- <akt>`).
- `fake.yaml`: das Fake-Backend der Probe mit erfundenen Werten.

Die Tasks `demo:*` laufen nur auf macOS und nur von Hand, nie in der CI.
