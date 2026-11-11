"""Render the output of a command as a PNG for a pull request.

The script runs a command, takes its combined output and renders it as a
terminal window with a headless browser. Use it for pull requests that show
a command line result.

Usage::

    mise run pr:screenshot -- out.png -- bin/a38 version

The first argument is the PNG file. The arguments after ``--`` are the
command. The exit code of the command does not stop the script; the picture
shows it.
"""

from __future__ import annotations

import html
import subprocess
import sys
from pathlib import Path

from playwright.sync_api import sync_playwright

#: Maximum number of output lines in the picture.
MAX_LINES = 120

_PAGE = """<!doctype html><meta charset="utf-8">
<style>
body {{ margin: 0; background: #1e1e2e; }}
.win {{ display: inline-block; min-width: 720px; padding: 16px 20px;
  font: 13px/1.45 ui-monospace, Menlo, monospace; color: #cdd6f4; }}
.cmd {{ color: #a6e3a1; }} .rc {{ color: #9399b2; }}
pre {{ margin: 8px 0 0; white-space: pre-wrap; word-break: break-word;
  max-width: 1100px; }}
</style>
<div class="win"><div class="cmd">$ {cmd}</div><pre>{out}</pre>
<div class="rc">exit code {rc}</div></div>"""


def render(command: list[str], target: Path) -> int:
    """Run a command and write a picture of its output.

    :param command: command and arguments.
    :param target: path of the PNG file.
    :returns: the exit code of the command.
    """
    proc = subprocess.run(command, capture_output=True, text=True, check=False)
    lines = (proc.stdout + proc.stderr).rstrip("\n").splitlines()
    if len(lines) > MAX_LINES:
        lines = lines[:MAX_LINES] + [f"... ({len(lines) - MAX_LINES} more lines)"]
    page_html = _PAGE.format(
        cmd=html.escape(" ".join(command)),
        out=html.escape("\n".join(lines)),
        rc=proc.returncode,
    )
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()
        page = browser.new_page(device_scale_factor=2)
        page.set_content(page_html)
        page.locator(".win").screenshot(path=str(target))
        browser.close()
    return proc.returncode


def main(argv: list[str]) -> int:
    """Parse the arguments and render the picture.

    :param argv: arguments without the program name.
    :returns: 0 if the picture exists, 2 for wrong arguments.
    """
    if len(argv) < 3 or argv[1] != "--":
        print("usage: term_screenshot.py OUT.png -- COMMAND [ARG...]", file=sys.stderr)
        return 2
    target = Path(argv[0])
    code = render(argv[2:], target)
    print(f"{target}: exit code of the command {code}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
