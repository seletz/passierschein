# Approval popup spike

This folder contains a throwaway prototype for issue #4.
It is not production code. Do not use it in the product.

The prototype is a SwiftPM executable.
It reads one approval request as JSON from stdin.
It shows a floating panel (AppKit `NSPanel` with SwiftUI content).
It writes one decision as JSON to stdout and stops.

The report is in `docs/explanation/spikes/20-freigabe-ui.md`.

## Build

```sh
swift build
```

The build needs macOS 15 or later and Swift 6.

## Run

Run without Touch ID. The popup closes after 5 seconds:

```sh
.build/debug/approval-popup --no-auth --auto-close 5 < samples/a-first-request.json
```

Run with Touch ID (default path, calls LocalAuthentication):

```sh
.build/debug/approval-popup < samples/a-first-request.json
```

## Samples

| File | Case |
| --- | --- |
| `samples/a-first-request.json` | First request of a session with two credentials |
| `samples/b-bundle-many.json` | Bundle with twelve credentials in two vaults |
| `samples/c-deviation.json` | Deviation from an active grant, request from a VM |
| `samples/d-outside-dev.json` | Request outside dev |

All samples contain neutral test data only.

## Options

| Option | Function |
| --- | --- |
| `--request <file>` | Read the request from a file, not from stdin. |
| `--auto-close <seconds>` | Close the popup after this time with outcome `timeout`. |
| `--no-auth` | Do not call Touch ID. Use this option only for tests. |
| `--auth-policy <name>` | `biometrics` (default), `biometrics_or_companion`, `owner`. |
| `--take-focus` | Make the popup the key window when it opens. |
| `--closable` | Add a close button. Use this option to test the window manager. |
| `--debug-window` | Write the window attributes to stderr. |

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Outcome `approved` |
| 10 | Outcome `denied` |
| 11 | Outcome `timeout` |
| 12 | Outcome `error`, Touch ID failed or is not available |
| 64 | Usage error |
| 65 | Outcome `error`, the request is not valid |
| 70 | Outcome `error`, internal error |

## Limits

- The prototype is a bare executable. It has no app bundle and no bundle ID.
- The prototype sets the activation policy `accessory`. This gives the same behaviour as `LSUIElement`.
- The Touch ID dialog shows the name of the executable, not a product name.
- The prototype does not sign the decision.
