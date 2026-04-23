# GoClaw + PinchTab Linux Mint Handoff

Date: 2026-04-23

## Current state

The current `goclaw + PinchTab` setup is considered working in its present configuration.

This branch contains three practical changes that matter for the Linux Mint deployment path:

1. GoClaw can use PinchTab directly through `GOCLAW_BROWSER_PINCHTAB_URL` instead of trying to launch a local Chrome process through the legacy go-rod path.
2. OpenAI-compatible assistant output and live WS events are sanitized so leaked tool-call markup and structured content wrappers do not reach the UI as raw text.
3. ACP idle session cleanup is relaxed to `2h`, which avoids dropping long survey/browser runs during real pauses.

## Files that matter

- PinchTab backend:
  - `pkg/browser/backend.go`
  - `pkg/browser/pinchtab.go`
  - `pkg/browser/tool.go`
  - `cmd/gateway_setup.go`
  - `internal/config/config_channels.go`
  - `internal/config/config_load.go`
- Output sanitization:
  - `internal/providers/openai_content.go`
  - `internal/providers/openai_types.go`
  - `internal/providers/openai_http.go`
  - `internal/providers/openai_chat.go`
  - `internal/providers/tool_markup_sanitize.go`
  - `internal/gateway/event_sanitize.go`
  - `internal/gateway/server.go`
- Session stability:
  - `internal/providers/acp_provider.go`
- Local run helper:
  - `start-goclaw.sh`

## Required runtime inputs

At minimum, the Linux Mint host should provide:

```sh
GOCLAW_BROWSER_PINCHTAB_URL=http://127.0.0.1:9867
GOCLAW_BROWSER_PINCHTAB_TOKEN=...
```

Optional but currently used in this integration:

```sh
GOCLAW_BROWSER_PINCHTAB_MODE=headed
GOCLAW_BROWSER_PINCHTAB_PROFILE=goclaw
```

If `GOCLAW_BROWSER_PINCHTAB_URL` is missing, GoClaw falls back to the older browser startup path and may try to locate a local Chrome/Chromium binary.

## Local start flow on Linux Mint

`start-goclaw.sh` assumes:

1. repo path: `/home/vokov/projects/goclaw`
2. env file: `/home/vokov/projects/goclaw/.env.local`
3. binary path: `/home/vokov/projects/goclaw/goclaw`

Start command:

```sh
/home/vokov/projects/goclaw/start-goclaw.sh
```

The script exports variables from `.env.local` and then execs the GoClaw binary.

## Deployment checklist for Linux Mint

1. Pull the branch/commit with this patch set.
2. Ensure the target host already has a working PinchTab daemon.
3. Verify PinchTab API reachability from the GoClaw host:

```sh
curl -fsS http://127.0.0.1:9867/health
```

4. Verify `.env.local` contains the PinchTab URL/token and the required provider credentials.
5. Rebuild GoClaw with Go `1.26+`.
6. Start GoClaw through `start-goclaw.sh` or the equivalent service wrapper.
7. Smoke test:
   - `curl -fsS http://127.0.0.1:18790/health`
   - browser tool `start`
   - browser tool `open`
   - browser tool `snapshot`
   - one real survey / handoff path that previously exposed tool-call garbage in UI text

## Known validation note

`go test ./...` was not runnable in the current shell as-is because the installed local Go toolchain is `1.24.7`, while this repo requires `go 1.26.0`. Re-run tests with `GOTOOLCHAIN=auto` or a native Go `1.26+` installation on the deployment host.
