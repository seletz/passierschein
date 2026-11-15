---
render_macros: true
id: 'D68'
titel: 'Der Skill nennt den exec-Weg als Regel für test und prod und als bevorzugten Weg in dev'
bereich: 'Oberflächen'
lauf: '2.0'
stand: '2026-11-15'
order: 9
headless: true
status: gueltig
use_cases: [UC15, UC16]
beleg:
  notiz: Vorschlag aus Issue 91, zu bestätigen durch Stefan
  annahme: true
---
# D68 — Der Skill nennt den exec-Weg als Regel für test und prod und als bevorzugten Weg in dev

{{ design_kopf(page) }}

**Entscheidung:** Der Skill aus `a38 llm --skill` ({{ ref("D22") }}) erhält die folgenden Teile. Der Skill bleibt Englisch nach ASD-STE100. Die übrigen Teile bleiben, wie sie sind.

**Neue Regeln** (ersetzen die Regel „In this version a38 delivers only credentials of dev vaults“ und die Regel „Pass a value only to the program that needs it“):

```markdown
- Use `run_with_credentials` when a command needs a credential. You do not see the value.
- For a test or prod vault, `run_with_credentials` is the only way. `request_credentials` returns `exec_required`.
- For a dev vault, prefer `run_with_credentials`. Use `request_credentials` only if no single command can use the value.
- Never put a value into the arguments of a command. Give it through `env` only.
- Never try to print, encode or copy a value out of a command. The output is masked, but only the exact value.
```

**Neuer Abschnitt für das Werkzeug:**

```markdown
## Tool `run_with_credentials`

a38 starts one command for you. The command gets the credentials as environment variables.
You get the exit code and the output of the command. You never get the value.

Parameters:

- `command`: the program, for example `psql`. Required. Not a shell text.
- `args`: the arguments, a list of strings.
- `env`: a map from the name of an environment variable to a reference. Required.
  Example: `{"PGPASSWORD": "op://beispiel.app.k8s-test/datenbank-test/password"}`.
- `cwd`: the working directory. It must be in your project. Default: your working directory.
- `timeout_seconds`: the time limit. Default 120.
- `task` and `reason`: as for `request_credentials`. Give them.

The human sees the full command. Use a short and clear command.
If you need a shell, use `sh` with `-c` and refer to the variable, for example `"$PGPASSWORD"`.
Then each command needs its own approval.

The result has `exit_code`, `timed_out`, `stdout`, `stderr`, the flags `stdout_truncated` and `stderr_truncated` and `masked`.
A non-zero exit code is a result, not an a38 error. Read the output and continue your work.
`[masked:NAME]` in the output replaces the value of the variable NAME.

stdin is empty. Interactive commands do not work.
The output keeps the last 64 KiB of each stream.
```

**Neuer Abschnitt zu den Grants** (ersetzt „It is valid for a short time“):

```markdown
A grant lasts as long as the human allows. In dev it can last some hours. In test it is short and is valid only for the same program.
In prod there is no grant: each command needs a new approval.
```

**Neue Codes:**

```markdown
- `exec_required`: this vault gives values only through `run_with_credentials`. Send the same task once with `run_with_credentials`. Do not use another way.
- `exec_failed`: a38 could not start the command after delivery. Stop and tell the human.
```

**Begründung:** Der Skill steuert, welchen Weg ein Agent wählt. Ohne die Regel „prefer `run_with_credentials`“ nimmt ein Agent in dev weiter die MCP-Antwort, und der Wert steht im Transkript ({{ ref("RI1") }}). Der Satz „It is valid for a short time“ war nach dem Praxistest falsch: Der Grant lief 4 Stunden ([Issue 83](https://github.com/seletz/passierschein/issues/83)). `exec_required` ist der einzige Code, nach dem der Agent einen anderen Weg nehmen soll; der Weg ist genau benannt ({{ ref("RI8") }}).

**Quelle:** [Issue 91](https://github.com/seletz/passierschein/issues/91); [Issue 83](https://github.com/seletz/passierschein/issues/83), Rückmeldung des Agents Punkt 2. Vorschlag des Designlaufs 2; Stefan bestätigt ihn.
