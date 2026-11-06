# Development Guide

## Architecture

This is a **monorepo using git subtrees** that combines two open-source projects:

- **cc-connect** (`/cc-connect`) — Discord→Codex CLI bridge (Go)
  - Upstream: `https://github.com/chenhg5/cc-connect`
  - Fork with customizations: `https://github.com/disc0nnctd/cc-connect`
- **codex-switcher** (`/codex-switcher`) — Codex account manager (Rust/Tauri)
  - Upstream: `https://github.com/Lampese/codex-switcher`
- **custom/** — Your custom extensions (zero upstream conflict risk)
  - `custom/commands/codex-switch` — Python CLI for account switching
  - `custom/skills/` — Planned: cc-connect skill `.md` files
  - `custom/commands/` — Planned: additional command scripts

## Building

### Build cc-connect only (primary)

```bash
make build
# or
make build-cc-connect
```

Binary goes to `bin/cc-connect`.

### Build codex-switcher (Tauri desktop app)

```bash
make build-codex-switcher
```

### Clean build artifacts

```bash
make clean
```

## Deployment

### Deploy to running service

```bash
make deploy
```

This rebuilds `cc-connect` and restarts the `cc-connect` systemd service.

### Check service status

```bash
make status
# or manually
sudo systemctl status cc-connect
```

## Syncing from Upstream

### Sync cc-connect from original upstream

When `chenhg5/cc-connect` ships updates:

```bash
make sync-cc-connect
```

This runs:
```bash
git subtree pull --prefix=cc-connect cc-connect-upstream main --squash
```

**Conflict risk zones** (if upstream touched these, you'll need to resolve):
- `cc-connect/core/engine.go`
- `cc-connect/platform/discord/discord.go`

### Sync codex-switcher from upstream

When `Lampese/codex-switcher` ships updates:

```bash
make sync-codex-switcher
```

### Sync both at once

```bash
make sync-upstream
```

## Adding Custom Features

### Zero-conflict pattern: Shell commands

Add a new script to `custom/commands/`:

```bash
# Example: custom/commands/my-feature
#!/usr/bin/env python3
import json
import sys

if __name__ == "__main__":
    print(f"Hello from custom feature: {sys.argv[1:]}")
```

Wire it in cc-connect config.toml:

```toml
[[commands]]
  name = "myfeature"
  description = "My custom feature"
  exec = "/home/notdc/Tools/ccx/custom/commands/my-feature {{args}}"
```

### Adding Discord slash commands

**Every command must be registered in two places or it won't appear in Discord's UI:**

1. **`cc-connect/platform/discord/discord.go`** — add to `builtinSlashCommands()`:
   ```go
   {Name: "mycommand", Description: "What it does", Options: []*discordgo.ApplicationCommandOption{
       optStr("args", "argument description", false),
   }},
   ```
2. **`config.toml`** — wire it to a script:
   ```toml
   [[commands]]
     name = "mycommand"
     description = "What it does"
     exec = "/home/notdc/Tools/ccx/custom/commands/my-script {{args}}"
   ```

Then `make build` + restart service.

> **Note:** Discord command names cannot contain spaces. Use hyphens (e.g. `account-add` not `account add`).

### Low-conflict pattern: New files

Add isolated files that upstream won't create. Examples:
- New platform: `cc-connect/platform/discord/my_integration.go`
- New agent: `cc-connect/agent/custom_agent/custom_agent.go`
- Custom data: `cc-connect/custom_data.go`

Import them in `cc-connect/cmd/cc-connect/main.go` or the relevant package.

### Medium-conflict pattern: Core modifications

If you must modify `engine.go`, `discord.go`, or other shared files:

1. Add a `// CUSTOM:` marker before your changes
2. Document what you changed in a comment
3. When syncing upstream, manually resolve conflicts (usually just append your code)

Example:
```go
// CUSTOM: workspace feature — maps channels to filesystem paths
func (e *Engine) workspaceSession(msg *Message) (string, error) {
    // ...
}
```

## Testing

After building or syncing:

1. Run `/account` in Discord (via cc-connect)
   ```
   /account              # lists accounts
   /account dcpro        # switches to dcpro
   ```

2. Check logs:
   ```bash
   sudo journalctl -u cc-connect -f
   ```

3. Verify binary paths:
   ```bash
   ps aux | grep cc-connect
   # Should show: /home/notdc/Tools/ccx/bin/cc-connect
   ```

## Directory Layout

```
~/Tools/ccx/
├── Makefile                   # Build targets
├── DEVELOPMENT.md             # This file
├── README.md                  # Project overview
├── .gitignore
├── bin/                       # Built binaries (gitignored)
│   └── cc-connect
├── custom/
│   └── commands/
│       └── codex-switch       # Account switcher CLI
├── cc-connect/                # Git subtree (upstream: chenhg5/cc-connect)
│   ├── cmd/
│   ├── core/
│   ├── agent/
│   ├── platform/
│   └── ...
└── codex-switcher/            # Git subtree (upstream: Lampese/codex-switcher)
    ├── src/                   # React frontend
    ├── src-tauri/             # Rust backend
    └── ...
```

## Git Workflow

### View remotes

```bash
git remote -v
```

You'll see:
- `cc-connect-fork` — your fork (disc0nnctd/cc-connect) — use this to push customizations
- `cc-connect-upstream` — original (chenhg5/cc-connect) — pull from here
- `codex-switcher-upstream` — original (Lampese/codex-switcher)

### Committing changes

- **Custom features**: Commit normally to `master`
- **Fixes to cc-connect or codex-switcher code**: Consider pushing to your fork as a PR to upstream, or document in your commits why the change was needed

### Managing upstream merges

When you run `make sync-cc-connect`:
1. New upstream commits are pulled into `cc-connect/` directory
2. Git creates a merge commit showing the subtree integration
3. If there are conflicts, you'll need to resolve them manually in the affected files
4. Test the merged code (`make build` + `make deploy`)
5. The merge commit is created automatically; just `git add` and commit

## Troubleshooting

### Binary not found after deploy

```bash
# Rebuild explicitly
make clean build

# Check binary exists
ls -lh bin/cc-connect

# Check service is pointing to right path
cat /etc/systemd/system/cc-connect.service | grep ExecStart
```

### Sync conflict in cc-connect/core/engine.go

```bash
# View the conflict
git status

# Edit the file to keep both sides
vim cc-connect/core/engine.go

# Mark as resolved
git add cc-connect/core/engine.go

# Complete the merge
git commit
```

### Account command not working

```bash
# Test the script directly
/home/notdc/Tools/ccx/custom/commands/codex-switch

# Verify config path
grep -A2 "name = \"account\"" /home/notdc/Tools/ccx/config.toml

# Check Discord bot can execute (logs)
sudo journalctl -u cc-connect -n 50
```

## codex-lb

Account load balancer — pools multiple Codex/ChatGPT accounts and exposes an OpenAI-compatible endpoint at port 2455.

- **Image:** `ghcr.io/soju06/codex-lb:latest`
- **Data volume:** `codex-lb-data` (Docker volume — accounts, config, usage history)
- **Dashboard:** `http://localhost:2455` (local access, no auth required)
- **API:** `http://127.0.0.1:2455/api/accounts`

### Run command

```bash
docker run -d --name codex-lb \
  --network host \
  -v codex-lb-data:/var/lib/codex-lb \
  ghcr.io/soju06/codex-lb:latest
```

**`--network host` is required.** Without it, requests from the host arrive from `172.17.0.1` (Docker bridge) instead of `127.0.0.1`, which triggers the remote bootstrap auth requirement and breaks the `/api/accounts` endpoint used by custom commands.

### Systemd service

Managed by `/etc/systemd/system/codex-lb.service` (enabled, auto-restarts).

```bash
sudo systemctl start codex-lb
sudo systemctl stop codex-lb
sudo systemctl status codex-lb
```

### Update procedure

```bash
docker pull ghcr.io/soju06/codex-lb:latest
sudo systemctl restart codex-lb
```

The data volume is preserved across updates.

---

**Last Updated:** 2026-04-15
