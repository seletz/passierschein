# Passierschein (CLI `a38`)

Credential broker for coding agents: `a38 serve` (Go daemon on the Mac, only component with 1Password access), a Swift approval app (popup + Touch ID), `a38 mcp` (MCP server for the agents), `a38 tui`, `a38 llm --skill`.

**Read `docs/design/index.md` first.** The design set is the specification. Do not decide again what it decides. If something in it is wrong or missing, say so and add a gap; do not diverge silently.

## Rules

- **The design is frozen.** Design run 1 is accepted and frozen (2026-11-11); never change the body of its artefacts. A design error becomes a new artefact in run 2 with `dg-register` (skill `dg-register`); a replaced decision gets the status `ersetzt`, and that PR stays a draft for Stefan. Never assign a number by hand, never write a bare ID in text (`{{ ref("D1") }}`). A test names its test case (`// T2: ...`).
- **Use cases:** a use case with a complex flow shows a Mermaid sequence diagram (fenced block `mermaid`; Zensical renders it).
- **Spikes** answer open questions. Report in `docs/explanation/spikes/`, throwaway code in `spikes/`.
- **Worktrees only** (`wt switch --create <type>/<issue>-<name>`), never work on `develop`. Never `wt --yes`; hook approvals are Stefan's decision. Merge only through a PR. The coordinator checks each PR (CI, tests, diff) and then merges it (Stefan, 2026-11-11).
- **Tasks:** `mise` for everything, no Makefiles. Workflows call only `mise run <task>`; actions are pinned to a commit hash (`mise run tools:check`). Tool versions are pinned in `mise.toml`.
- **Language:** docs (`docs/`, `README.md`), issues and PRs in German, DIN 8579 (one statement per sentence, short, active, present tense, one term per thing). Code, identifiers, comments, error messages and commit messages in English, ASD-STE100.
- **Test data is neutral:** no real customer, company or person names, no real secrets, no real vault or item names. Use `beispiel.app.k8s-dev`, `datenbank-dev`, `beispielprojekt`.
- **Secrets:** a secret value never appears in a log, an error message or test output. gitleaks runs before each commit (`mise run hooks:install`) and in CI.
- **Do not use 1Password** (`op`, the 1Password SSH agent, the SDK) without Stefan's explicit approval. Commits are unsigned (`git -c commit.gpgsign=false commit`); push over HTTPS with the gh credentials.
- **Pull requests:** use `.github/pull_request_template.md`. Each PR names the features, the use cases and the implemented test cases (T IDs with the test function), and shows a screenshot (`mise run pr:screenshot`, `mise run pr:screenshots:upload`). A PR with a choice or an acceptance for Stefan (each UI decision) stays a draft and is assigned to him (`--add-assignee seletz`); set it to ready only after his approval in the discussion.
- **Docs review:** on request, show the built site in the tailnet (`tailscale serve --bg --https=<port> http://localhost:<port>`), tell Stefan the URL with the port, and remove the share after the review.
- `dg-register` is the globally installed instance (global mise configuration). Do not pin it in this repo and do not change the global configuration; tell Stefan if a newer version is necessary.
- Go: static build, `CGO_ENABLED=0`. TDD: failing test first. Table-driven tests. Go doc comments on exported identifiers.
- Conventional Commits (`feat(serve): …`); the release notes come from them. SemVer.
- Never mention Claude or AI in commits, PRs or issues.

## Tasks

`mise run check | build | test | lint | vuln | secrets | docs:serve | docs:build | design:check | release:snapshot`
