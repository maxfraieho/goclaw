# Alpine GoClaw Migration Report - 2026-04-15

## Summary

Successfully migrated Alpine GoClaw surveys agent from claude2-cc to nvidia-minimax-1 with model minimaxai/minimax-m2.7.

## Repository Status

| Item | Value |
|-------|-------|
| Local path | /home/vokov/projects/goclaw |
| Branch | main |
| Fork remote | origin -> maxfraieho/goclaw |
| Upstream remote | upstream -> nextlevelbuilder/goclaw |
| Commits behind upstream | ~230 |
| Docker image version | v3.7.0 |

## Agent Migration

### Before
| Agent | Provider | Model |
|-------|----------|-------|
| surveys | claude2-cc | cc/claude-haiku-4-5 |

### After
| Agent | Provider | Model |
|-------|----------|-------|
| surveys | nvidia-minimax-1 | minimaxai/minimax-m2.7 |

### Agent ID
019d75bf-b32f-7eb6-8573-35dac8a56927

## Provider Configuration

| Field | Value |
|-------|-------|
| Name | nvidia-minimax-1 |
| Type | openai_compat |
| API Base | https://integrate.api.nvidia.com/v1 |
| API Key | nvapi-*** (set) |
| Enabled | true |

## UI Hot-Swap Bug

**Status:** Already fixed locally via upstream commit 8aa351b0
- Calls refreshProviders() when dialog opens
- Removes verify-blocking from agent creation flow
- No PR needed - fix already in upstream main

## Known Issue: Browser PinchTab Not Activating

### Bug Description
GOCLAW_BROWSER_PINCHTAB_URL env var is ignored. Browser tool always starts in headless mode instead of using PinchTab.

### Environment Variables Set


### Observed Behavior
- Log shows: browser tool enabled headless=true
- Expected: browser tool enabled (PinchTab) with url=http://host.docker.internal:9867
- Affects: Docker Compose deployment with latest ghcr.io/nextlevelbuilder/goclaw:latest (v3.7.0)

### Workarounds
1. Wait for upstream fix
2. Build goclaw natively from source with Go 1.26+
3. Report bug to upstream with reproduction steps

## Safety

- No surveys submitted
- No routing/Telegram changes
- No n8n changes
- Only surveys agent provider/model modified
- Telegram bot has conflict errors from multiple polling instances (secondary issue)

## Verification



## Files Modified

- /home/vokov/projects/goclaw/.env - GOCLAW_BROWSER_PINCHTAB_URL updated
- docs/reports/alpine_nvidia_minimax_migration_2026-04-15.md - This report

## Git Commit

Branch: fix/pinchtab-stop-recovery
Commit: d8daf5dd - docs: add Alpine NVIDIA Minimax migration report

