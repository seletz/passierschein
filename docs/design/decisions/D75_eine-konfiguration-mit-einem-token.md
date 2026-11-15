---
render_macros: true
id: 'D75'
titel: 'Eine Konfiguration mit einem Token lädt nicht mehr, und die Meldung nennt den Umbau'
bereich: 'Betrieb und Release'
lauf: '2.0'
stand: '2026-11-15'
order: 8
headless: true
status: gueltig
use_cases: [UC11]
beleg:
  notiz: Vorschlag aus der Entwicklung (Issue 88), zu bestätigen durch Stefan
  annahme: true
---
# D75 — Eine Konfiguration mit einem Token lädt nicht mehr, und die Meldung nennt den Umbau

{{ design_kopf(page) }}

**Entscheidung:** Es gibt keinen stillen Übergang. Eine Konfiguration im Format mit einem Token lädt nicht. Die Meldung nennt den Befund und den nächsten Schritt.

**Was nicht mehr lädt:**

- ein Schlüssel `backend.onepassword.token_source` oder `backend.onepassword.token_file` direkt unter `onepassword`,
- ein Vault ohne `account` ({{ ref("D70") }}).

**Die Meldungen** (Code und Meldungen auf Englisch):

```text
backend.onepassword.token_source: this key is from the configuration with one token and is no longer valid; each account names its own token source. Next step: move the key into an account under backend.onepassword.accounts (for example "accounts: [{name: privat, token_source: keychain}]"), add "account: privat" to each vault, store the token with "a38 config onepassword-token set --account privat" and restart the daemon (how-to: Mehrere 1Password-Konten einrichten)
vault "beispiel.app.k8s-dev": the account is missing; each vault names its 1Password account (account: NAME), and NAME must be in backend.onepassword.accounts
vault "beispiel.app.k8s-dev": the account "privat" is not in backend.onepassword.accounts; add it there or correct the name
```

**Was der Mensch ändert:**

1. Er legt unter `backend.onepassword.accounts` ein Konto an, zum Beispiel `privat`. Eine bisherige Quelle `file` und ihre Datei zieht er in dieses Konto um.
2. Er ergänzt an jedem Vault `account: privat`.
3. Er prüft die Datei mit `a38 config validate`.
4. Er legt das Token neu ab: `a38 config onepassword-token set --account privat`. Der alte Eintrag im Schlüsselbund wird nicht übernommen ({{ ref("D72") }}).
5. Er startet den Daemon neu und prüft mit `a38 doctor`.
6. Er löscht den alten Eintrag: `a38 config onepassword-token delete --legacy`. `a38 doctor` meldet ihn bis dahin als Befund.

**Begründung:** Ein automatischer Übergang bräuchte ein Standardkonto. Das schließt {{ ref("D70") }} aus. Die Konfiguration hat genau einen Menschen als Leser; der Umbau ist kurz. Eine klare Meldung beim Start ist sicherer als eine Datei, die halb im alten und halb im neuen Format gilt.

**Quelle:** [Issue 88](https://github.com/seletz/passierschein/issues/88), Frage „Übergang für eine bestehende Konfiguration“. Vorschlag aus der Entwicklung, zu bestätigen durch Stefan.
