# Acer Aspire — GoClaw + PinchTab Network Access

Date: 2026-04-18
Host: Acer Aspire (vokov-Aspire-5742G)
Scope: external network reachability with token auth — aligned with MacBook reference pattern.

## Addresses

- LAN: `192.168.3.172`
- Tailscale: `100.82.201.41`
- SSH: `vokov@192.168.3.172`

## Services

| Service | Port | Bind | Process | Unit |
|---|---|---|---|---|
| GoClaw gateway | 18790 | `*:18790` (all ifaces) | native binary `/home/vokov/projects/goclaw/goclaw --verbose` | user systemd `goclaw.service` |
| PinchTab | 9867 | `0.0.0.0:9867` | native `/home/vokov/.pinchtab/bin/0.8.5/pinchtab-linux-amd64 server` | user systemd `pinchtab.service` |
| PinchTab default instance | 9868 | `*:9868` | bridge spawned by PinchTab | same unit |

Both services are already bound on wildcard — no firewall (ufw inactive, nftables rules do not block these ports). Reachable from any LAN client and Tailscale peers without port-forward work.

## GoClaw — runtime token source

- Unit: `/home/vokov/.config/systemd/user/goclaw.service`
- EnvironmentFile: `/home/vokov/.goclaw.local.env`  ← systemd reads only this file
- Historical full secret set also in: `/home/vokov/projects/goclaw/.env`  ← NOT auto-loaded by systemd
- Code refs: `cmd/gateway.go:358` → `httpapi.InitGatewayToken(cfg.Gateway.Token)`; `internal/config/config_load.go:115` → `envStr("GOCLAW_GATEWAY_TOKEN", &c.Gateway.Token)`; auth logic at `internal/http/auth.go:174`.

### Active values (live runtime, read from /proc/PID/environ)

```
GOCLAW_GATEWAY_TOKEN=39a92a6ad43dc001fe7c61657b70a456
GOCLAW_ENCRYPTION_KEY=ad14fdbf74a6cbe1d04d73faa80ad9a787b0ced515cc921ac7df37cf70d907ae
GOCLAW_BROWSER_PINCHTAB_TOKEN=eafaeac1492de35551672d3b8b40a3b88dae8edaf036df0c
GOCLAW_BROWSER_PINCHTAB_URL=http://127.0.0.1:9867
GOCLAW_OWNER_IDS=system
```

Same values now persisted in `/home/vokov/.goclaw.local.env`, so they survive `systemctl --user restart goclaw`.

### Auth format that works

```
Authorization: Bearer 39a92a6ad43dc001fe7c61657b70a456
X-GoClaw-User-Id: system
```

`X-GoClaw-User-Id` header is required. Value `system` matches `GOCLAW_OWNER_IDS=system` so the caller gets `RoleOwner`. Any id listed in `GOCLAW_OWNER_IDS` works.

`/health` is public (no auth). All `/v1/*` endpoints require both headers.

## PinchTab — runtime token source

- Unit: `/home/vokov/.config/systemd/user/pinchtab.service`
- Env: `PINCHTAB_CONFIG=/home/vokov/.pinchtab/config.json`
- Token field: `.server.token` in `config.json`
- Live config also sets `server.bind: 0.0.0.0`, `server.port: "9867"`

### Active token

```
server.token = eafaeac1492de35551672d3b8b40a3b88dae8edaf036df0c
```

This is the same value goclaw uses as `GOCLAW_BROWSER_PINCHTAB_TOKEN` (sha256 verified to match) — so goclaw → pinchtab handshake works end-to-end.

### Auth format that works

```
Authorization: Bearer eafaeac1492de35551672d3b8b40a3b88dae8edaf036df0c
```

No user-id header needed. All non-OPTIONS requests return 401 without this header (response sets `WWW-Authenticate: Bearer realm="pinchtab"`).

## Verify commands (run from any LAN / Tailscale peer, e.g. Orange Pi)

```sh
GTOK='39a92a6ad43dc001fe7c61657b70a456'
PTOK='eafaeac1492de35551672d3b8b40a3b88dae8edaf036df0c'

# GoClaw
curl -s -o /dev/null -w '%{http_code}\n' http://192.168.3.172:18790/health
# expect 200

curl -s -H "Authorization: Bearer $GTOK" -H 'X-GoClaw-User-Id: system' \
     http://192.168.3.172:18790/v1/agents | head -c 200
# expect 200 + JSON {"agents":[...]} or {"agents":null}

# PinchTab
curl -s -H "Authorization: Bearer $PTOK" http://192.168.3.172:9867/health
# expect 200 + JSON {"status":"ok",...,"authRequired":true,...}
```

Expected matrix (confirmed 2026-04-18 from Orange Pi):

| test | http | note |
|---|---|---|
| goclaw /health (no auth) | 200 | public |
| goclaw /v1/agents (no auth) | 401 | |
| goclaw /v1/agents (bearer only) | 400 | missing X-GoClaw-User-Id |
| goclaw /v1/agents (bearer + user) | 200 | working |
| goclaw /v1/agents (wrong bearer) | 401 | |
| pinchtab /health (no auth) | 401 | |
| pinchtab /health (bearer) | 200 | working |
| pinchtab /health (wrong bearer) | 401 | |

## What was changed on 2026-04-18

Root cause: `GOCLAW_GATEWAY_TOKEN`, `GOCLAW_ENCRYPTION_KEY`, `GOCLAW_BROWSER_PINCHTAB_TOKEN` existed in `/home/vokov/projects/goclaw/.env` but not in `/home/vokov/.goclaw.local.env`. systemd loads only `.goclaw.local.env`, so the running process had empty gateway token → HTTP auth effectively disabled (dev/single-user fallback) + no goclaw→pinchtab authenticated handshake.

Fix:

```sh
cp ~/.goclaw.local.env ~/.goclaw.local.env.bak.$(date +%s)

grep -E '^(GOCLAW_GATEWAY_TOKEN|GOCLAW_ENCRYPTION_KEY|GOCLAW_BROWSER_PINCHTAB_TOKEN)=' \
  /home/vokov/projects/goclaw/.env >> ~/.goclaw.local.env

systemctl --user restart goclaw.service
```

Backup written: `/home/vokov/.goclaw.local.env.bak.1776527318`.

PinchTab required no changes — already bound on `0.0.0.0` and token already present in `config.json`.

## Post-restart / post-redeploy behavior

- `systemctl --user restart goclaw.service` — re-reads env file, token persists.
- `systemctl --user restart pinchtab.service` — re-reads config.json, token persists.
- If `/home/vokov/projects/goclaw/.env` is regenerated (e.g. `./goclaw onboard` produces new values), re-copy the three secret lines into `~/.goclaw.local.env` (delete the old ones first).
- If PinchTab `config.json` is regenerated, update `GOCLAW_BROWSER_PINCHTAB_TOKEN` in `~/.goclaw.local.env` in lockstep so goclaw can still authenticate to it.

## Do not

- Do not enable Acer in live survey processing stack (still dev).
- Do not overwrite `.goclaw.local.env` from an older backup without merging the three secret lines back.
- Do not rotate only one side of the pinchtab token pair — rotate both `~/.pinchtab/config.json` and `~/.goclaw.local.env` together.
