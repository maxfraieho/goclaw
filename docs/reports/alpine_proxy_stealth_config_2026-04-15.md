# Alpine Proxy & Stealth Configuration Report

**Date:** 2026-04-15  
**Host:** Alpine (192.168.3.184)

## Target Setup

Route all browser traffic through SOCKS5 proxy on Android tablet in Switzerland:
- Proxy: `100.100.74.9:9888`
- Expected geo: Zürich, Switzerland
- IP verified: `62.202.190.158` (Swisscom)

## PinchTab Configuration

### Alpine Native PinchTab Status

PinchTab is running at `localhost:9867` with proxy configuration applied to Chromium:

**Proxy Settings:**
```
--proxy-server=socks5://100.100.74.9:9888
--proxy-bypass-list=localhost,127.0.0.1,100.100.74.9
```

**Stealth Settings:**
```
--disable-geolocation
--disable-webrtc
--webrtc-ip-handling-policy=disable_non_proxied_udp
--user-agent=Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36
--tz=Europe/Zurich
--stealthLevel=maximum
```

### Verified Working

From Alpine host:
```bash
curl --socks5-hostname 100.100.74.9:9888 https://ipinfo.io/json
# Returns: Zurich, CH IP 62.202.190.158
```

## Docker goclaw Issue

**Bug:** goclaw Docker image v3.7.0 ignores `GOCLAW_BROWSER_PINCHTAB_URL` environment variable.

**Evidence:**
- Log shows: `browser tool enabled headless=true` (not PinchTab)
- Even with `GOCLAW_BROWSER_PINCHTAB_URL=http://host.docker.internal:9867` set in .env
- PinchTab bridge still uses internal Docker bridge instead of Alpine native

**Impact:**
- Browser traffic does NOT route through SOCKS5 proxy when using Docker goclaw
- Surveys agent uses nvidia-minimax-1 correctly, but browser tool is headless only

**Options:**
1. Wait for upstream goclaw fix (issue: upstream bug in v3.7.0)
2. Use native goclaw binary (requires Go 1.26+, Alpine has 1.24.7)
3. Use Alpine native pinchtab directly for browser operations

## Working Components

| Component | Status | Notes |
|-----------|--------|-------|
| SOCKS5 Proxy | ✅ | 100.100.74.9:9888, Zürich IP confirmed |
| Alpine PinchTab | ✅ | Running with proxy flags |
| Proxy IP Verification | ✅ | 62.202.190.158 (Swisscom, Zürich) |
| Surveys Agent Provider | ✅ | nvidia-minimax-1 / minimaxai/minimax-m2.7 |
| Docker goclaw Browser | ❌ | Uses headless, ignores PINCHTAB_URL |

## Config File Location

- Alpine PinchTab: `/home/vokov/.pinchtab/config.json`
- Backup: `/home/vokov/.pinchtab/config.json.backup`
- goclaw .env: `/home/vokov/projects/goclaw/.env`

## Recommendations

1. **Short term:** Accept headless browser mode for Docker goclaw (no proxy for browser)
2. **Medium term:** Wait for upstream goclaw fix for PINCHTAB_URL support in Docker
3. **Long term:** Consider running goclaw natively with Go 1.26+ upgrade path

