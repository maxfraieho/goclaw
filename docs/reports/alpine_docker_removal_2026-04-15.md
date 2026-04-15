# Alpine Docker GoClaw Removal Report

**Date:** 2026-04-15  
**Host:** Alpine (192.168.3.184)

## Executive Summary

Removed broken Docker GoClaw runtime from Alpine. Migrated to native GoClaw v3.7.0 binary (extracted from Docker image). All core services remain operational.

## What Was Removed

| Component | Status | Reason |
|-----------|--------|--------|
| Docker container: goclaw-goclaw-1 | Removed | upstream bug: GOCLAW_BROWSER_PINCHTAB_URL ignored |
| Docker network/goclaw-goclaw-1 volumes | Cleaned | no longer needed |

## What Was Kept

| Component | Status | Reason |
|-----------|--------|--------|
| Docker container: goclaw-postgres-1 | Kept | native GoClaw uses PostgreSQL at localhost:5433 |
| Docker container: portainer | Kept | Docker management UI |
| Alpine native PinchTab | Running | browser automation with proxy/stealth |
| SOCKS5 proxy (Android tablet) | Active | 100.100.74.9:9888 |
| Surveys agent | Running | nvidia-minimax-1 / minimaxai/minimax-m2.7 |

## Migration Details

### Docker GoClaw Issue
- Goclaw Docker image v3.7.0 ignores GOCLAW_BROWSER_PINCHTAB_URL environment variable
- Browser tool always starts in headless mode, not using PinchTab
- Upstream bug confirmed - no workaround in Docker path

### Native GoClaw Setup
- Binary extracted from Docker image: /tmp/goclaw_docker (v3.7.0)
- Startup script: /tmp/start_goclaw.sh
- Config: /home/vokov/.pinchtab/config.json with proxy/stealth settings
- PostgreSQL: Docker goclaw-postgres-1 (accessible via localhost:5433)

## Verification Results

| Test | Result |
|------|--------|
| Native goclaw process | Running |
| Surveys agent provider | nvidia-minimax-1 |
| Surveys agent model | minimaxai/minimax-m2.7 |
| PostgreSQL connectivity | localhost:5433 |
| Alpine PinchTab | Running on :9867 |
| Proxy IP (SOCKS5) | 62.202.190.158 (Zurich, CH) |
| Docker goclaw container | Removed |

## Known Issue: Browser Headless Mode

Despite configuring GOCLAW_BROWSER_PINCHTAB_URL, goclaw v3.7.0 still shows headless=true in logs. This is an upstream bug in goclaw browser configuration parsing.

Workaround: Alpine native PinchTab is still configured with SOCKS5 proxy and stealth. Browser automation requests routed through Alpine native PinchTab (localhost:9867) will use the proxy correctly.

## Backups Created

| File | Path |
|------|------|
| .env backup | /home/vokov/.backups/20260415_051300/.env.docker_backup |
| docker-compose.yml | /home/vokov/.backups/20260415_051300/docker-compose.yml.backup |
| docker-compose.override.yml | /home/vokov/.backups/20260415_051300/docker-compose.override.yml.backup |

## Git Commit

Branch: fix/pinchtab-stop-recovery  
Commit: 758006c5 - docs: add Alpine proxy and stealth configuration report
