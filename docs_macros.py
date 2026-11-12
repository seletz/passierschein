"""Build-time macros of the documentation (Zensical, ``zensical.toml``).

The Markdown extension ``zensical.extensions.macros`` registers the macros.
Only pages with ``render_macros: true`` in their front matter run through
Jinja. An error in a macro stops the build.

- :func:`version_banner` shows the version, the build date and the commit
  hash of the documentation.
- ``ref`` links a design artefact by its ID.
- ``design_kopf`` shows the head line of a design artefact from its front
  matter.
- ``design_index`` lists the artefacts of one family, one table per area.

The version comes from the environment variable ``DOCS_VERSION`` (for a
manual build). Without it, the version comes from ``git describe``. After a
release it reads ``0.1.0-3-gabc1234``, and the banner names the last release.
Outside a git checkout, the version is ``dev``.
"""

from __future__ import annotations

import os
import re
import subprocess
from datetime import datetime, timezone
from pathlib import Path

import yaml

#: Folder of the design set, relative to the docs folder.
_DESIGN_DIR = "design"

#: A released version: no commits after the tag, no ``-dirty`` marker.
_RELEASE = re.compile(r"^\d+\.\d+\.\d+$")

#: A version after a release: ``git describe`` gives ``<tag>-<n>-g<hash>``,
#: with ``-dirty`` for a changed checkout.
_AFTER_RELEASE = re.compile(r"^(\d+\.\d+\.\d+)-(\d+)-g[0-9a-f]+(?:-dirty)?$")

_FRONTMATTER = re.compile(r"\A---\s*\n(.*?)\n---\s*\n", re.DOTALL)

#: Display text of each status value.
_STATUS_TEXT = {
    "aktiv": "aktiv",
    "verworfen": "verworfen",
    "gueltig": "gültig",
    "ersetzt": "ersetzt",
    "zurueckgezogen": "zurückgezogen",
    "offen": "offen",
    "vertagt": "vertagt",
    "geschlossen": "geschlossen",
    "akzeptiert": "akzeptiert",
}


def _git(root: Path, *args: str) -> str:
    """Run a git command and return its output.

    :param root: project root (the git checkout).
    :param args: arguments of the git command.
    :returns: the output without surrounding space, or an empty text if the
        command fails.
    """
    try:
        proc = subprocess.run(
            ["git", *args], cwd=root, capture_output=True, text=True, timeout=20, check=True
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    return proc.stdout.strip()


def docs_version(root: Path, environ: dict[str, str] | None = None) -> str:
    """Return the version that the site documents.

    Order: ``DOCS_VERSION``, then ``git describe --tags --match 'v*' --always
    --dirty``, then ``dev``. A leading ``v`` is removed.

    :param root: project root (the git checkout).
    :param environ: environment to read ``DOCS_VERSION`` from; the default is
        ``os.environ``.
    :returns: the version, for example ``0.1.0`` or ``0.1.0-3-gabc1234``.
    """
    env = os.environ if environ is None else environ
    version = (env.get("DOCS_VERSION") or "").strip()
    if not version:
        version = _git(root, "describe", "--tags", "--match", "v*", "--always", "--dirty")
    return version.removeprefix("v") or "dev"


def build_date(environ: dict[str, str] | None = None) -> str:
    """Return the build date of the site in UTC.

    ``SOURCE_DATE_EPOCH`` sets the date for reproducible builds. Without it,
    the date is the current time.

    :param environ: environment to read ``SOURCE_DATE_EPOCH`` from; the
        default is ``os.environ``.
    :returns: the date as ``YYYY-MM-DD HH:MM UTC``.
    """
    env = os.environ if environ is None else environ
    epoch = (env.get("SOURCE_DATE_EPOCH") or "").strip()
    moment = (
        datetime.fromtimestamp(int(epoch), tz=timezone.utc)
        if epoch.isdigit()
        else datetime.now(tz=timezone.utc)
    )
    return moment.strftime("%Y-%m-%d %H:%M UTC")


def version_banner_text(root: Path, repo_url: str) -> str:
    """Return the version line of the start page.

    :param root: project root (the git checkout).
    :param repo_url: URL of the repository, without a slash at the end.
    :returns: one Markdown line with version, build date and commit hash.
    """
    version = docs_version(root)
    commit = _git(root, "rev-parse", "--short", "HEAD") or "unbekannt"
    return f"{version_label(version, repo_url)} · gebaut am {build_date()} · Commit `{commit}`"


def version_label(version: str, repo_url: str) -> str:
    """Return the version part of the banner.

    :param version: result of :func:`docs_version`.
    :param repo_url: URL of the repository, without a slash at the end.
    :returns: a release version links to its release. A version after a
        release (``0.1.0-3-gabc1234``) names the last release and the number
        of commits since. Any other version is a development state.
    """
    if _RELEASE.match(version):
        return f"**Version {version}** ([Release]({repo_url}/releases/tag/v{version}))"
    after = _AFTER_RELEASE.match(version)
    if after:
        tag, count = after.groups()
        commits = "1 Commit" if count == "1" else f"{count} Commits"
        return (
            f"**Version {version}** (Entwicklungsstand, kein Release; "
            f"letztes Release [{tag}]({repo_url}/releases/tag/v{tag}), {commits} danach)"
        )
    return f"**Version {version}** (Entwicklungsstand, kein Release)"


def _frontmatter(md: Path) -> dict:
    """Return the front matter of a Markdown file.

    :param md: path of the file.
    :returns: the front matter, or an empty dict if the file has none.
    """
    match = _FRONTMATTER.match(md.read_text(encoding="utf-8"))
    return (yaml.safe_load(match.group(1)) or {}) if match else {}


def design_artefacts(docs_dir: Path) -> dict[str, dict]:
    """Return the artefacts of the design set by ID.

    The function reads each Markdown file in a family folder of
    ``docs/design`` (not ``index.md``) that has the field ``id`` in its front
    matter.

    :param docs_dir: docs folder of the project.
    :returns: ID mapped to the front matter plus ``pfad``, the path of the
        file.
    """
    artefacts: dict[str, dict] = {}
    for md in sorted((docs_dir / _DESIGN_DIR).glob("*/*.md")):
        if md.name == "index.md":
            continue
        meta = _frontmatter(md)
        if meta.get("id"):
            artefacts[str(meta["id"])] = {**meta, "pfad": md}
    return artefacts


def design_ref(artefacts: dict[str, dict], artefact_id: str, start: Path) -> str:
    """Return a link to an artefact, relative to the folder of the page.

    :param artefacts: result of :func:`design_artefacts`.
    :param artefact_id: ID, for example ``D7``.
    :param start: absolute folder of the page that calls the macro.
    :returns: a Markdown link, for example ``[D7](decisions/D7_….md)``.
    :raises KeyError: if the ID is unknown. The build then stops; it does not
        make a dead link.
    """
    key = artefact_id.strip()
    if key not in artefacts:
        raise KeyError(f"unknown design artefact: {artefact_id!r} (docs/design/)")
    return f"[{key}]({os.path.relpath(artefacts[key]['pfad'], start=start)})"


def _status_text(artefacts: dict[str, dict], meta: dict, start: Path) -> str:
    """Return the status of an artefact as display text.

    :param artefacts: result of :func:`design_artefacts`.
    :param meta: front matter of the artefact.
    :param start: absolute folder of the page that shows the status.
    :returns: the status, with the replacing artefact and the assumption
        marker where they apply.
    """
    status = str(meta.get("status", ""))
    text = _STATUS_TEXT.get(status, status)
    if status == "ersetzt" and meta.get("ersetzt_durch"):
        target = str(meta["ersetzt_durch"])
        text += " durch " + (design_ref(artefacts, target, start) if target in artefacts else target)
    if (meta.get("beleg") or {}).get("annahme"):
        text += " (Annahme)"
    return text


def design_head_text(artefacts: dict[str, dict], meta: dict, start: Path) -> str:
    """Return the head line of an artefact as a block quote.

    :param artefacts: result of :func:`design_artefacts`.
    :param meta: front matter of the artefact.
    :param start: absolute folder of the page.
    :returns: one block-quote line with area, run, date, status, owner and
        evidence.
    """
    parts = [
        f"**Bereich:** {meta.get('bereich', '')}",
        f"**Lauf:** {meta.get('lauf', '')}",
        f"**Stand:** {meta.get('stand', '')}",
    ]
    if meta.get("status"):
        parts.append(f"**Status:** {_status_text(artefacts, meta, start)}")
    owner = (meta.get("register") or {}).get("owner")
    if owner:
        parts.append(f"**Owner:** {owner}")
    evidence = meta.get("beleg") or {}
    for field, name in (("wer", "Beleg von"), ("datum", "Beleg vom"), ("notiz", "Notiz")):
        if evidence.get(field):
            parts.append(f"**{name}:** {evidence[field]}")
    if evidence.get("quelle"):
        parts.append("**Quelle:** " + ", ".join(str(q) for q in evidence["quelle"]))
    return "> " + " · ".join(parts)


def design_index_lines(artefacts: dict[str, dict], folder: Path) -> list[str]:
    """Return the tables of a family index, one table per area.

    ``bereiche`` of the index gives the order of the areas. ``order`` and then
    the number give the order of the rows.

    :param artefacts: result of :func:`design_artefacts`.
    :param folder: absolute folder of the family.
    :returns: Markdown lines.
    """
    meta = _frontmatter(folder / "index.md")
    family = str(meta.get("familie", ""))
    members = [a for a in artefacts.values() if a["pfad"].parent == folder]

    def number(artefact: dict) -> int:
        return int(str(artefact["id"])[len(family) :] or 0)

    lines: list[str] = []
    for area in meta.get("bereiche") or []:
        rows = [a for a in members if a.get("bereich") == area]
        if not rows:
            continue
        lines += [f"## {area}", "", "| ID | Titel | Status |", "|---|---|---|"]
        for artefact in sorted(rows, key=lambda a: (a.get("order", 0), number(a))):
            link = design_ref(artefacts, artefact["id"], folder)
            title = str(artefact.get("titel", "")).replace("|", "\\|")
            lines.append(f"| {link} | {title} | {_status_text(artefacts, artefact, folder)} |")
        lines.append("")
    return lines or ["_Diese Familie hat noch keine Artefakte._"]


def define_env(env) -> None:
    """Register the macros with the macro environment of Zensical.

    :param env: environment of the extension; ``env.conf`` holds the project
        configuration.
    """
    root = Path(env.conf.get("root_dir", ".")).resolve()
    docs_dir = root / env.conf.get("docs_dir", "docs")
    repo_url = (env.conf.get("repo_url") or "https://github.com/seletz/passierschein").rstrip("/")
    cache: dict[str, dict] = {}

    def _design() -> dict[str, dict]:
        if not cache:
            cache.update(design_artefacts(docs_dir))
        return cache

    def _page_folder() -> Path:
        page = getattr(env, "variables", {}).get("page")
        if page is None:
            raise RuntimeError("ref() needs the current page in the macro context")
        return (docs_dir / page.path).resolve().parent

    @env.macro
    def version_banner() -> str:
        return version_banner_text(root, repo_url)

    @env.macro
    def ref(artefact_id: str) -> str:
        return design_ref(_design(), artefact_id, _page_folder())

    @env.macro
    def design_kopf(page) -> str:
        path = (docs_dir / page.path).resolve()
        return design_head_text(_design(), _frontmatter(path), path.parent)

    @env.macro
    def design_index(page) -> str:
        folder = (docs_dir / page.path).resolve().parent
        return "\n".join(design_index_lines(_design(), folder))
