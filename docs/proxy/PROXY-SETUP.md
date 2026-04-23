# Swiss AI Proxy — Access Setup Guide
**Oracle Server:** 100.66.97.93 (Tailscale)  
**Proxy chain:** Client → Oracle smart_proxy.py → Tablet microsocks (100.100.74.9:9888) → Swiss VPN → Zürich exit (62.202.190.158, CH)  
**Exit IP:** 62.202.190.158 · Zürich, Switzerland (Swisscom)

---

## Architecture

```
Windows Laptop (UA)          Oracle Server (UA)          Tablet (CH VPN)       Internet
┌──────────────┐             ┌──────────────────┐         ┌──────────────┐      ┌──────────┐
│ Claude app   │─────────┐   │ smart_proxy.py   │         │ microsocks   │      │          │
│ Chrome Swiss │  Tailsc │   │  :8888 HTTP      │────────▶│ :9888 SOCKS5 │─────▶│ ipinfo.io│
│ Comet/Perpl. │  ale    └──▶│  :1080 SOCKS5    │         │ (Swiss VPN)  │      │ claude.ai│
└──────────────┘             │                  │         └──────────────┘      │ openai.. │
                             │ Codex CLI        │                               └──────────┘
                             │ GoLaw agents     │
                             │ Pinchtab Chrome  │
                             └──────────────────┘
```

---

## 1. Oracle Server — System Proxy (already configured)

File `/etc/environment`:
```
HTTP_PROXY="http://127.0.0.1:8888"
HTTPS_PROXY="http://127.0.0.1:8888"
http_proxy="http://127.0.0.1:8888"
https_proxy="http://127.0.0.1:8888"
SOCKS5_PROXY="socks5://127.0.0.1:1080"
NO_PROXY="localhost,127.0.0.1,100.66.97.93,::1"
no_proxy="localhost,127.0.0.1,100.66.97.93,::1"
```

Service: `systemctl status swiss-proxy` — must be **active (running)**.  
Re/start: `sudo systemctl restart swiss-proxy`

---

## 2. Claude Code (CLI) — Windows Laptop

Claude Code respects standard `HTTPS_PROXY` / `HTTP_PROXY` environment variables.  
The `setup-windows.ps1` script sets these at user level automatically.

**After running setup-windows.ps1** (once, as Administrator):
```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\setup-windows.ps1
```

Then open a new terminal — `$env:HTTPS_PROXY` will be `http://100.66.97.93:8888`.

**Verify Claude Code is using the proxy:**
```powershell
claude --version
# In Claude Code session:
# Run: curl https://ipinfo.io/json
# Expected: "country": "CH"
```

**If Claude Code needs explicit proxy (future versions):**
```powershell
# ~/.claude/settings.json — no proxy field needed, uses env vars
# OR launch with env override:
$env:HTTPS_PROXY="http://100.66.97.93:8888"; claude
```

---

## 3. Codex CLI — Oracle Server

OpenAI Codex CLI (`openai` / `codex`) reads `HTTPS_PROXY` from environment.  
The system `/etc/environment` already exports it to every shell session.

**Verify after login:**
```bash
echo $HTTPS_PROXY
# Should output: http://127.0.0.1:8888

# Quick connectivity test:
curl -s https://ipinfo.io/json | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['country'], d['city'])"
# Expected: CH Zürich
```

**If running Codex in a Docker container**, pass proxy vars:
```bash
docker run --rm \
  -e HTTPS_PROXY=http://100.66.97.93:8888 \
  -e HTTP_PROXY=http://100.66.97.93:8888 \
  -e NO_PROXY=localhost,127.0.0.1 \
  your-codex-image
```

---

## 4. GoLaw Agents — Oracle Server

GoLaw Python agents inherit proxy vars from the shell environment via `/etc/environment`.  
No per-agent config needed unless the agent explicitly overrides proxy settings.

**Check agent HTTP client (if using `requests` or `httpx`):**
```python
import os
print(os.environ.get('HTTPS_PROXY'))  # → http://127.0.0.1:8888
```

**If using `requests` library** — it automatically reads `HTTPS_PROXY` env var.  
**If using `httpx`** — set explicitly if env vars not picked up:
```python
import httpx
proxies = {"https://": "http://127.0.0.1:8888", "http://": "http://127.0.0.1:8888"}
client = httpx.Client(proxies=proxies)
```

**systemd service**: if GoLaw runs as a systemd service, add to `[Service]` section:
```ini
Environment="HTTPS_PROXY=http://127.0.0.1:8888"
Environment="HTTP_PROXY=http://127.0.0.1:8888"
Environment="NO_PROXY=localhost,127.0.0.1,100.66.97.93,::1"
```

---

## 5. Pinchtab Chrome — Oracle Server

Pinchtab uses an embedded Chromium, but this build does **not** accept proxy flags on
`pinchtab-main server`. The working setup is to point `browser.binary` at a Chromium
wrapper script and keep the rest of the browser flags in `browser.extraFlags`.

**Wrapper script:** `/home/vokov/.pinchtab/chrome-swiss-proxy.sh`

```bash
#!/bin/sh
exec /usr/bin/chromium-browser \
  --proxy-server=http://100.66.97.93:8888 \
  --proxy-bypass-list=localhost\;127.*\;100.66.97.93 \
  --enforce-webrtc-ip-permission-check \
  --webrtc-ip-handling-policy=disable_non_proxied_udp \
  --lang=de-CH \
  --accept-lang=de-CH,de,en \
  "$@"
```

**Pinchtab config:** `/home/vokov/.pinchtab/config.json`

```json
{
  "browser": {
    "binary": "/home/vokov/.pinchtab/chrome-swiss-proxy.sh",
    "extraFlags": "--disable-webrtc --force-fieldtrials=*WebRTCDisableIPv6/default --disable-geolocation --disable-backgrounding-occluded-windows --disable-renderer-backgrounding --disable-features=CalculateNativeWinOcclusion --disable-hang-monitor --disable-dev-shm-usage --disable-gpu --no-first-run --no-default-browser-check --disable-client-side-phishing-detection --disable-extensions --disable-sync --lang=de-CH --accept-lang=de-CH,de,en"
  }
}
```

**Verified Chromium flags on the running Pinchtab browser:**
- `--proxy-server=http://100.66.97.93:8888`
- `--proxy-bypass-list=localhost;127.*;100.66.97.93`
- `--enforce-webrtc-ip-permission-check`
- `--webrtc-ip-handling-policy=disable_non_proxied_udp`
- `--lang=de-CH`
- `--accept-lang=de-CH,de,en`

**For plain Chromium/Chrome on server:**
```bash
chromium-browser \
  --proxy-server=http://100.66.97.93:8888 \
  --proxy-bypass-list="localhost;127.*;100.66.97.93" \
  --enforce-webrtc-ip-permission-check \
  --webrtc-ip-handling-policy=disable_non_proxied_udp \
  --lang=de-CH \
  --accept-lang=de-CH,de,en \
  --no-sandbox \
  "$@"
```

**WebRTC leak check:** open `https://browserleaks.com/webrtc` in the browser.  
Expected: no local IP visible, IP shown = 62.202.190.158 (CH).

---

## 6. Chrome on Windows Laptop (Swiss Profile)

After running `setup-windows.ps1`, a shortcut **"Chrome Swiss"** is created on the Desktop.  
This shortcut uses a separate Chrome profile (`SwissProfile`) with:
- SOCKS5 proxy: `100.66.97.93:1080`
- WebRTC disabled
- Language: `de-CH`

**Always use "Chrome Swiss" shortcut** for GoLaw/Claude browsing.  
Standard Chrome shortcut bypasses proxy.

---

## 7. Verification Commands

**On Oracle server:**
```bash
# Exit IP check
curl -s https://ipinfo.io/json | python3 -m json.tool | grep -E '"ip"|"city"|"country"'

# Proxy service status
systemctl status swiss-proxy

# SOCKS5 test
curl -s --socks5 127.0.0.1:1080 https://ipinfo.io/json | python3 -m json.tool
```

**On Windows laptop (PowerShell):**
```powershell
# Exit IP through proxy
(Invoke-WebRequest -Uri "https://ipinfo.io/json" -Proxy "http://100.66.97.93:8888" -UseBasicParsing).Content | ConvertFrom-Json | Select ip,city,country

# Check env vars are set
[System.Environment]::GetEnvironmentVariable("HTTPS_PROXY", "User")
```

---

## 8. Quick Troubleshooting

| Symptom | Check |
|---|---|
| `curl: (7) Failed to connect` | `systemctl status swiss-proxy` — restart if inactive |
| Country = UA instead of CH | Tablet offline? `ping 100.100.74.9` — check Tailscale |
| Claude Code ignores proxy | Re-open terminal after running setup-windows.ps1 |
| Docker container bypasses proxy | Pass `-e HTTPS_PROXY=http://100.66.97.93:8888` explicitly |
| WebRTC leaks real IP | Use launch-pinchtab.sh wrapper, not bare binary |
| swap pressure / OOM | `sudo systemctl restart swiss-proxy n8n` — check `free -m` |

---

*Last updated: 2026-04-19*  
*Setup script: `swiss-ai-proxy-legion-go/setup-windows.ps1`*
