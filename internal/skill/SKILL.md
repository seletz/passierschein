---
name: a38
description: Ask the human for credentials (passwords, tokens, keys) through a38. Use it when a task needs a credential that you do not have. Never read credentials any other way.
---

# a38: credentials for coding agents

## What a38 is

a38 is a credential broker. It sits between you and the password manager.
You do not get access to the password manager.
You ask a38 for a credential. A human then decides.
If the human agrees, you get the value.

You use two MCP tools of the server `a38`: `request_credentials` and `grant_status`.

## Rules

- Ask only for what the task needs.
- Ask for all credentials of the task in one request. Do not send one request for each credential.
- Never run `op` or any other password manager tool yourself.
- Never write a value into a file, a commit, a log or an output. Never print it, never repeat it in a message.
- Pass a value only to the program that needs it, for example through an environment variable of one command.
- Do not look for another way to get a credential. See "Errors".
- In this version a38 delivers only credentials of dev vaults. A request for any other vault ends with `not_allowed`.

## Tool `request_credentials`

Parameters:

- `references`: a list of references. Required. One reference for each credential.
- `task`: what you are doing. Short sentence. Give it.
- `reason`: why you need these credentials for the task. Short sentence. Give it.

A reference has the form `op://vault/item/field`.
Example: `op://beispiel.app.k8s-dev/datenbank-dev/password`.
Use the exact names of the vault, the item and the field.

The human sees your task and your reason. They are your claims. Be honest and exact.

The answer has one entry for each reference, in the same order.
An entry has a value or an error.
A request can end with some values and some errors.

## Tool `grant_status`

It has no parameters.
It lists the grants of your session.
It never returns a value.

## Grants

A grant is a permission from the human. It is valid for a short time.
It belongs to your session and to the references that the human approved.
While a grant is valid, a new request for the same references needs no new approval.
After it expires, ask again.
Call `grant_status` to see which grants are valid and how long they last.

## Errors

A failed request returns an error with a stable `code`.
Decide on the code, not on the message text.

For every code the rule is the same.
Stop the work that needs the credential.
Tell the human the code and the message.
Do not try another way. Do not guess names. Do not retry in a loop.
Do not use `op`, a file with a secret, an environment variable of the human or any other source.

The codes:

- `denied`: the human said no or chose nothing. Stop and tell the human. Do not ask again unless the human tells you to.
- `timeout`: the human did not answer in time. Stop and tell the human. Ask again only if the human tells you to.
- `not_allowed`: the policy does not allow this request. The human did not see it. Stop and tell the human. In this version it also means that the vault is not a dev vault.
- `bundle_too_large`: the bundle of your session is full. Stop and tell the human. Do not split the request to get around the limit.
- `daemon_unavailable`: `a38 mcp` cannot reach the daemon. Stop and tell the human to start `a38 serve`.
- `backend_locked`: the password manager is locked. Stop and tell the human to unlock it.
- `backend_unavailable`: the daemon cannot reach the password manager. Stop and tell the human.
- `ui_failed`: the approval window failed. Stop and tell the human.
- `audit_failed`: the daemon cannot write its audit log. Stop and tell the human.
- `daemon_faulted`: the daemon found a fault in itself. It delivers nothing to any session until the human clears the lock. Stop and tell the human.
- `config_invalid`: the configuration of your project is invalid. Stop and tell the human to fix it.
- `session_name_conflict`: two sessions have the same name. Stop and tell the human.
- `not_found`: an item or a field does not exist. Check the reference for typing errors only if the human gave you the exact names. Otherwise stop and tell the human.
- `invalid_request`: the request is not valid, for example a wrong reference. Fix the request once if the error is clear. Otherwise stop and tell the human.
