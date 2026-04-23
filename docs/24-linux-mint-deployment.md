# Linux Mint Deployment

## Goal

Bring up another server with the same runtime shape as this machine:

- `goclaw`
- `PinchTab`
- the same proxy path
- local `Claude CLI`
- the same browser/tool wiring

Agents may differ on the target host.

## Source of truth for this setup

- `docs/reports/goclaw_pinchtab_linux_mint_handoff_2026-04-23.md`
- `docs/proxy/PROXY-SETUP.md`
- `start-goclaw.sh`

## What to do on the new server

1. Clone the repo.
2. Run `git pull`.
3. Recreate `.env.local` for that host.
4. Ensure PinchTab is already running and reachable.
5. Ensure the proxy path is configured exactly like on this machine.
6. Ensure `claude` is installed and logged in locally on that server.
7. Build `goclaw`.
8. Start it with `start-goclaw.sh` or an equivalent service wrapper.

## Required env

Minimum browser env:

```sh
GOCLAW_BROWSER_PINCHTAB_URL=http://127.0.0.1:9867
GOCLAW_BROWSER_PINCHTAB_TOKEN=...
```

Optional but used on this setup:

```sh
GOCLAW_BROWSER_PINCHTAB_MODE=headed
GOCLAW_BROWSER_PINCHTAB_PROFILE=goclaw
```

Proxy behavior should match this machine. Reuse the documented proxy chain and env/service settings from:

- `docs/proxy/PROXY-SETUP.md`

## Claude CLI

`Claude CLI` does not need to be rebuilt from this repo.

The target host only needs:

```sh
claude --version
claude login
```

GoClaw uses the local CLI session on that host.

API-level check after GoClaw starts:

```sh
curl -fsS http://127.0.0.1:18790/v1/providers/claude-cli/auth-status
```

## Build and start

```sh
go build -o goclaw .
/home/vokov/projects/goclaw/start-goclaw.sh
```

`start-goclaw.sh` assumes:

- repo: `/home/vokov/projects/goclaw`
- env file: `/home/vokov/projects/goclaw/.env.local`
- binary: `/home/vokov/projects/goclaw/goclaw`

## Checks

GoClaw:

```sh
curl -fsS http://127.0.0.1:18790/health
```

PinchTab:

```sh
curl -fsS http://127.0.0.1:9867/health
```

Claude CLI provider:

```sh
curl -fsS http://127.0.0.1:18790/v1/providers/claude-cli/auth-status
```

Browser flow:

1. `browser start`
2. `browser open`
3. `browser snapshot`
4. one real handoff flow

## Notes

- Do not copy agent data blindly if agents on the new host should differ.
- The committed code already includes the PinchTab backend and Linux Mint handoff changes.
- Proxy docs were committed separately so the new server can be made to behave the same way as this one.
