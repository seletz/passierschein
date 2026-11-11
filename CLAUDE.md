# Passierschein (CLI `a38`)

Credential broker for coding agents: `a38 serve` (Go daemon on the Mac, only component with 1Password access), a Swift approval app (popup + Touch ID), `a38 mcp` (MCP server for the agents), `a38 tui`, `a38 llm --skill`.

**Read `docs/design/index.md` first.** The design set is the specification. Do not decide again what it decides. If something in it is wrong or missing, say so and add a gap; do not diverge silently.

## Rules

- **Design before code.** No product code before Stefan has reviewed the design. Use cases, features, decisions, tests, gaps and risks only with `dg-register` (skill `dg-register`); never assign a number by hand, never write a bare ID in text (`{{ ref("D1") }}`).
- **Spikes** answer open questions. Report in `docs/explanation/spikes/`, throwaway code in `spikes/`.
- **Worktrees only** (`wt switch --create <type>/<issue>-<name>`), never work on `develop`. Never `wt --yes`; hook approvals are Stefan's decision. Merge only through a PR and only with Stefan's approval.
- **Tasks:** `mise` for everything, no Makefiles. Workflows call only `mise run <task>`; actions are pinned to a commit hash (`mise run tools:check`). Tool versions are pinned in `mise.toml`.
- **Language:** docs (`docs/`, `README.md`), issues and PRs in German, DIN 8579 (one statement per sentence, short, active, present tense, one term per thing). Code, identifiers, comments, error messages and commit messages in English, ASD-STE100.
- **Test data is neutral:** no real customer, company or person names, no real secrets, no real vault or item names. Use `beispiel.app.k8s-dev`, `datenbank-dev`, `beispielprojekt`.
- **Secrets:** a secret value never appears in a log, an error message or test output. gitleaks runs before each commit (`mise run hooks:install`) and in CI.
- **Do not use 1Password** (`op`, the 1Password SSH agent, the SDK) without Stefan's explicit approval. Commits are unsigned (`git -c commit.gpgsign=false commit`); push over HTTPS with the gh credentials.
- Go: static build, `CGO_ENABLED=0`. TDD: failing test first. Table-driven tests. Go doc comments on exported identifiers.
- Conventional Commits (`feat(serve): …`); the release notes come from them. SemVer.
- Never mention Claude or AI in commits, PRs or issues.

## Tasks

`mise run check | build | test | lint | vuln | secrets | docs:serve | docs:build | design:check | release:snapshot`
