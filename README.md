# ccx — Unified Codex Platform

A monorepo combining **cc-connect** (Discord→Codex bridge) and **codex-switcher** (account manager) with a `custom/` layer for extensions.

## Structure

- `cc-connect/` — git subtree from chenhg5/cc-connect (with custom workspace features)
- `codex-switcher/` — git subtree from Lampese/codex-switcher
- `custom/` — your custom scripts, skills, and commands (zero upstream conflict risk)

## Build & Deploy

```bash
make build           # Build cc-connect binary
make deploy          # Rebuild and restart cc-connect service
make sync-upstream   # Pull latest from both upstream repos
```

## Upstream Syncs

```bash
# Pull updates from cc-connect upstream
make sync-cc-connect

# Pull updates from codex-switcher upstream
make sync-codex-switcher
```
