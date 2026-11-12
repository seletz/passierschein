# demo-projekt

A small demo project. It has a database and an object storage.
Both are simulated. No script connects to a real system.

## Credentials

This project needs credentials. You do not have them.
Ask the human for them through the MCP server `a38` (tool `request_credentials`).
Follow the skill `a38`.

| Purpose | Reference | Environment variable |
|---|---|---|
| Database user | `op://passierschein.dev/datenbank-dev/username` | `DB_USER` |
| Database password | `op://passierschein.dev/datenbank-dev/password` | `DB_PASSWORD` |
| Object storage access key | `op://passierschein.dev/objektspeicher-dev/access-key` | `OBJEKTSPEICHER_ACCESS_KEY` |

Ask only for the credentials of the current task.

## Tasks

### Run the database migration

Needs `DB_USER` and `DB_PASSWORD`.

```sh
DB_USER=<user> DB_PASSWORD=<password> ./migrate.sh
```

### Upload the build artifact

Needs `OBJEKTSPEICHER_ACCESS_KEY`.

```sh
OBJEKTSPEICHER_ACCESS_KEY=<key> ./upload.sh
```

## Rules

- Never write a credential into a file, a commit or a message.
- Pass a credential only as an environment variable of the one command that needs it.
- If a38 says no, stop and tell the human.
