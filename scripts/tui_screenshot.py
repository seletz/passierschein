"""Take a screenshot of a running terminal program, such as ``a38 tui``.

The script starts the program in a pseudo terminal of a fixed size, sends
keys, reads the screen with the terminal emulator ``pyte`` and renders the
cells with their colours, bold and reverse video as a PNG with a headless
browser. No human and no real terminal window is needed.

Usage::

    mise run pr:screenshot:tui -- out.png --size 80x24 --keys "2 j enter" \\
        -- bin/a38 tui --file /tmp/a38-tui-demo/audit.db

Keys are separated by spaces: a name (enter, esc, tab, up, down, pgup,
pgdown, space), ``text:WORD`` for typed text, ``sleep:SECONDS`` for a pause,
or a single character. ``--theme light`` renders with a light palette.
The script sends ``q`` at the end and stops the program.
"""

from __future__ import annotations

import argparse
import fcntl
import html
import os
import pty
import re
import select
import struct
import subprocess
import sys
import termios
import time
from pathlib import Path

import pyte
from playwright.sync_api import sync_playwright

#: Escape sequences of the named keys.
KEYS = {
    "enter": "\r",
    "esc": "\x1b",
    "tab": "\t",
    "space": " ",
    "up": "\x1b[A",
    "down": "\x1b[B",
    "pgup": "\x1b[5~",
    "pgdown": "\x1b[6~",
}

#: Control sequences that pyte does not know and would print as text: CSI
#: sequences with the private markers "<", ">", "=" or with the intermediate
#: "$" (keyboard enhancements, mode queries of Bubble Tea).
UNKNOWN_CSI = re.compile(rb"\x1b\[[<>=][0-9;]*[a-zA-Z]|\x1b\[\??[0-9;]*\$[a-zA-Z]")

#: Block elements: the eighths of a cell from the bottom.
BLOCKS = {"▁": 1, "▂": 2, "▃": 3, "▄": 4, "▅": 5, "▆": 6, "▇": 7, "█": 8}

#: Palettes: the 16 colours of the terminal, foreground and background.
THEMES = {
    "dark": {
        "fg": "#cdd6f4", "bg": "#1e1e2e",
        "black": "#45475a", "red": "#f38ba8", "green": "#a6e3a1", "brown": "#f9e2af",
        "blue": "#89b4fa", "magenta": "#f5c2e7", "cyan": "#94e2d5", "white": "#bac2de",
    },
    "light": {
        "fg": "#24292f", "bg": "#ffffff",
        "black": "#24292f", "red": "#cf222e", "green": "#116329", "brown": "#7d4e00",
        "blue": "#0969da", "magenta": "#8250df", "cyan": "#1b7c83", "white": "#6e7781",
    },
}


def run(command: list[str], cols: int, rows: int, keys: list[str], wait: float) -> pyte.Screen:
    """Run the command in a pseudo terminal, send the keys, return the screen.

    :param command: command and arguments.
    :param cols: columns of the terminal.
    :param rows: rows of the terminal.
    :param keys: keys as described in the module documentation.
    :param wait: seconds to wait after the start and after each key.
    :returns: the screen after the last key.
    """
    screen = pyte.Screen(cols, rows)
    stream = pyte.ByteStream(screen)
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
    env = dict(os.environ, TERM="xterm-256color", COLUMNS=str(cols), LINES=str(rows))
    env.pop("NO_COLOR", None)
    proc = subprocess.Popen(command, stdin=slave, stdout=slave, stderr=slave, env=env, start_new_session=True)
    os.close(slave)

    def pump(seconds: float) -> None:
        end = time.monotonic() + seconds
        while True:
            left = end - time.monotonic()
            if left <= 0:
                return
            ready, _, _ = select.select([master], [], [], left)
            if not ready:
                return
            try:
                data = os.read(master, 65536)
            except OSError:
                return
            if not data:
                return
            stream.feed(UNKNOWN_CSI.sub(b"", data))

    pump(wait)
    for key in keys:
        if key.startswith("sleep:"):
            pump(float(key[6:]))
            continue
        text = key[5:] if key.startswith("text:") else KEYS.get(key, key)
        if key.startswith("text:"):
            for ch in text:
                os.write(master, ch.encode())
                pump(0.05)
        else:
            os.write(master, text.encode())
        pump(wait)
    os.write(master, b"q")
    pump(0.5)
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait()
    os.close(master)
    return screen


def colour(name: str, palette: dict[str, str], default: str) -> str:
    """Return the CSS colour of a pyte colour name.

    :param name: "default", a colour name or a hex value without "#".
    :param palette: the palette of the theme.
    :param default: the colour for "default".
    :returns: a CSS colour.
    """
    if name == "default":
        return default
    if name.startswith("bright"):
        name = name[6:]
    if name in palette:
        return palette[name]
    if len(name) == 6:
        return "#" + name
    return default


def to_html(screen: pyte.Screen, theme: str, title: str) -> str:
    """Render the cells of the screen as HTML.

    :param screen: the screen of the emulator.
    :param theme: "dark" or "light".
    :param title: the command line above the screen.
    :returns: an HTML page.
    """
    pal = THEMES[theme]
    rows = []
    for y in range(screen.lines):
        line = screen.buffer[y]
        spans = []
        for x in range(screen.columns):
            ch = line[x]
            fg = colour(ch.fg, pal, pal["fg"])
            bg = colour(ch.bg, pal, pal["bg"])
            if ch.reverse:
                fg, bg = bg, fg
            style = f"color:{fg};background:{bg}"
            if ch.bold:
                style += ";font-weight:700"
            text = html.escape(ch.data or " ")
            if ch.data in BLOCKS:
                # Draw the block as a filled part of the cell, so that the
                # bars have no gap between the lines, as in a terminal.
                pct = 100 * BLOCKS[ch.data] // 8
                style = (f"color:transparent;background:linear-gradient(to top,{fg} {pct}%,{bg} {pct}%)")
                text = " "
            spans.append(f'<span style="{style}">{text}</span>')
        rows.append("".join(spans))
    return f"""<!doctype html><meta charset="utf-8">
<style>
body {{ margin: 0; background: {pal['bg']}; }}
.win {{ display: inline-block; padding: 14px 18px; background: {pal['bg']}; }}
.cmd {{ font: 12px/1.4 ui-monospace, Menlo, monospace; color: {pal['white']}; margin-bottom: 8px; }}
.scr {{ font: 13px/16px Menlo, ui-monospace, monospace; white-space: pre; color: {pal['fg']}; }}
.scr div {{ height: 16px; }}
.scr span {{ display: inline-block; width: 1ch; height: 16px; vertical-align: top; overflow: hidden; }}
</style>
<div class="win"><div class="cmd">$ {html.escape(title)} ({screen.columns}x{screen.lines})</div>
<div class="scr">{''.join(f'<div>{r}</div>' for r in rows)}</div></div>"""


def main(argv: list[str]) -> int:
    """Parse the arguments, run the program and write the picture.

    :param argv: arguments without the program name.
    :returns: 0 if the picture exists, 2 for wrong arguments.
    """
    if "--" not in argv:
        print("usage: tui_screenshot.py OUT.png [--size 80x24] [--keys KEYS] [--wait S] "
              "[--theme dark|light] [--title TEXT] -- COMMAND [ARG...]", file=sys.stderr)
        return 2
    i = argv.index("--")
    parser = argparse.ArgumentParser(prog="tui_screenshot.py")
    parser.add_argument("out")
    parser.add_argument("--size", default="80x24")
    parser.add_argument("--keys", default="")
    parser.add_argument("--wait", type=float, default=0.6)
    parser.add_argument("--theme", choices=sorted(THEMES), default="dark")
    parser.add_argument("--title", default="")
    args = parser.parse_args(argv[:i])
    command = argv[i + 1:]
    cols, rows = (int(v) for v in args.size.split("x"))
    screen = run(command, cols, rows, args.keys.split(), args.wait)
    page_html = to_html(screen, args.theme, args.title or " ".join(command))
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()
        page = browser.new_page(device_scale_factor=2)
        page.set_content(page_html)
        page.locator(".win").screenshot(path=args.out)
        browser.close()
    print(f"{args.out}: {cols}x{rows}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
