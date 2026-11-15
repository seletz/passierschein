---
title: Spikes
description: "Experimente, die eine offene Frage des Designs klären."
---
# Spikes

Ein Spike klärt eine offene Frage mit einer Recherche oder einem Experiment.
Das Ergebnis ist ein Bericht mit einer Empfehlung. Die Entscheidung trifft
das Design-Set. Der Code eines Spikes liegt unter `spikes/` und ist
Wegwerfcode.

| Bericht | Frage | Stand |
|---|---|---|
| [1Password-Go-SDK](10-1password-sdk.md) | Desktop-Integration oder Service Account? | aus der Doku; Praxistest offen |
| [Freigabe-UI](20-freigabe-ui.md) | Aufbau des Popups und Verhalten unter OmniWM | Entwurf des Popups für das MVP abgenommen; Praxistest offen |
| [Signatur der Swift-App](30-signatur-swift-app.md) | quill oder `codesign` und `notarytool`? | ohne Developer ID; Praxistest offen |
| [Zugang der VMs über Tailscale](40-tailscale-vm-zugang.md) | Identität der VM und Beschränkung des Zugriffs | aus der Doku; Entscheidung und Praxistest offen |
| [Token im Schlüsselbund](50-token-im-schluesselbund.md) | Wie schützt der Schlüsselbund das Token des Service Accounts vor anderen Prozessen? | gemessen ohne Developer ID; Entscheidung und Praxistest offen |
| [Hardened Runtime und WebAssembly](60-hardened-runtime-wasm.md) | Warum stirbt das signierte a38 beim ersten Zugriff auf 1Password? | Ursache belegt; Interpreter gemessen (Anmeldung 63,9 s); Entitlement empfohlen |
