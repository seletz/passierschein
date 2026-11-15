---
render_macros: true
id: 'D71'
titel: 'Die Konfiguration nennt die 1Password-Konten mit Name und Token-Quelle'
bereich: 'Backends'
lauf: '2.0'
stand: '2026-11-15'
order: 9
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag aus der Entwicklung (Issue 88), zu bestätigen durch Stefan
  annahme: true
---
# D71 — Die Konfiguration nennt die 1Password-Konten mit Name und Token-Quelle

{{ design_kopf(page) }}

**Entscheidung:** Der Abschnitt `backend.onepassword` der Konfiguration ({{ ref("D49") }}) nennt die Konten in der Liste `accounts`. Jedes Konto hat einen frei gewählten Namen und eine eigene Quelle für das Token seines Service Accounts.

```yaml
version: 1
vaults:
  - name: beispiel.app.k8s-dev
    environment: dev
    account: privat
  - name: datenbank-dev
    environment: dev
    account: firma
backend:
  onepassword:
    accounts:
      - name: privat              # token_source: keychain (Standard)
      - name: firma
        token_source: file
        token_file: ~/.config/a38/onepassword-token-firma
```

**Regeln beim Laden:** Ein Verstoß bricht den Start ab ({{ ref("D49") }}).

| Schlüssel | Regel |
|---|---|
| `backend.onepassword.accounts[].name` | Pflicht. Kleinbuchstaben, Ziffern und Bindestrich, 1 bis 32 Zeichen, am Anfang ein Buchstabe oder eine Ziffer. Jeder Name steht höchstens einmal in der Liste. Der Name `service-account` ist gesperrt ({{ ref("D72") }}). |
| `backend.onepassword.accounts[].token_source` | `keychain` (Standard) oder `file` |
| `backend.onepassword.accounts[].token_file` | Absoluter Pfad oder Pfad mit `~/`. Der Standard ist `$XDG_CONFIG_HOME/a38/onepassword-token-<name>`, ohne die Variable `~/.config/a38/onepassword-token-<name>`. Zwei Konten mit derselben Datei sind ein Fehler. |
| `vaults[].account` | Pflicht ({{ ref("D70") }}). Der Name eines Kontos aus `accounts`. |

Ein Konto ohne Vault ist erlaubt. `a38 doctor` nennt es als Hinweis. Der Daemon liest sein Token nicht.

**Namen der Vaults:** Der Name eines Vaults bleibt in der ganzen Konfiguration eindeutig, über alle Konten ({{ ref("D49") }}). Die Referenz des Agents nennt kein Konto ({{ ref("D27") }}). Der Daemon findet das Konto über den Vault. Zwei Konten mit einem Vault desselben Titels behandelt {{ ref("G41") }}.

**Befehle:** `a38 config account add <name> [--token-source keychain|file] [--token-file <pfad>]`, `a38 config account remove <name>` und `a38 config account list` ändern und zeigen die Liste nach den Regeln aus {{ ref("D55") }}. `remove` lehnt ein Konto ab, das ein Vault nennt. `a38 config vault add` verlangt `--account`.

**Service Account je Konto:** {{ ref("D39") }} gilt für jedes Konto: Jedes Konto hat einen eigenen Service Account, der nur die Vaults der Konfiguration lesen darf.

**Begründung:** Ein Service Account gehört zu genau einem Konto. Mehrere Konten brauchen deshalb mehrere Tokens. Die Quelle je Konto erlaubt den Schlüsselbund für ein Konto und die Datei für ein Konto der Entwicklung. Ein kurzer Name aus wenigen Zeichen passt in den Schlüsselbund, in das Log und in die Freigabeanfrage. Eindeutige Namen der Vaults halten die Referenz und das Protokoll unverändert.

**Folge:** Die Schlüssel `backend.onepassword.token_source` und `backend.onepassword.token_file` aus {{ ref("D51") }} entfallen. Den Übergang regelt {{ ref("D75") }}.

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88), Skizze. Vorschlag aus der Entwicklung, zu bestätigen durch Stefan.
