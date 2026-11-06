# Known Issues

## codex-lb container lost on update — missing restart config

**Date:** 2026-04-15  
**Status:** Resolved 2026-04-15

### Problem

When updating the `ghcr.io/soju06/codex-lb:latest` Docker image, the old container was stopped and removed but the original `docker run` command was not documented anywhere. This means after an update, codex-lb cannot be restarted without knowing the correct flags (port bindings, volume mounts, env vars).

The container was running for 4 days before being removed and no run command existed in:
- bash history
- any shell script
- any systemd service file
- `~/miniclaw/docs/`
- `DEVELOPMENT.md`

### Impact

- codex-lb is down (port 2455 unreachable)
- All ccx integrations that depend on codex-lb are broken:
  - `/codex-usage` Discord command
  - `/usage-all` Discord command
  - `/accounts` Discord command
  - `custom/commands/usage`
  - `custom/commands/usage-all`
  - `custom/commands/lb-accounts`

### Fix Required

1. Determine the correct `docker run` command for codex-lb (check [https://github.com/Soju06/codex-lb](https://github.com/Soju06/codex-lb) README)
2. Restart codex-lb with the correct flags
3. Document the run command in `DEVELOPMENT.md` under a new **codex-lb** section
4. Create a systemd service or restart script so it survives reboots and can be easily updated in future

### Suggested systemd service (once run flags are known)

```ini
[Unit]
Description=codex-lb account load balancer
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=notdc
ExecStartPre=-/usr/bin/docker rm -f codex-lb
ExecStart=/usr/bin/docker run --rm --name codex-lb \
  -p 2455:2455 \
  # add volume mounts / env vars here
  ghcr.io/soju06/codex-lb:latest
ExecStop=/usr/bin/docker stop codex-lb
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Update procedure (once documented)

```bash
docker pull ghcr.io/soju06/codex-lb:latest
sudo systemctl restart codex-lb
```
