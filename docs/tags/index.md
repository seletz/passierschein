---
title: Tags
description: "Tag-Index der Doku: Art der Seite, Familie im Design-Set und Thema."
icon: lucide/tags
weight: 90
---
# Tags

Jede Seite trägt wenige, stabile Tags. Die Tags sind hierarchisch:
`Design-Set/Gap` liegt unter `Design-Set`.

| Zweig | Bedeutung | Gesetzt in |
|---|---|---|
| `Art/…` | Art der Seite: Tutorial, How-To, Referenz, Erklärung, Spike | `docs/<abschnitt>/.meta.yml` |
| `Design-Set/…` | Familie des Artefakts: Use-Case, Entscheidung, Feature, Testfall, Gap, Risiko | `docs/design/<familie>/.meta.yml` |
| `Thema/…` | Thema der Seite, zum Beispiel Release oder Betrieb | Front Matter der Seite |

Ein Ordner gibt seine Tags über die Datei `.meta.yml` vor. Die Tags eines
Ordners gelten auch für seine Unterordner. Eine Seite ergänzt eigene Tags
unter `tags` im Front Matter.

Ein neuer Tag braucht einen Grund. Wenige stabile Tags sind besser als viele
Tags.

<!-- material/tags -->
