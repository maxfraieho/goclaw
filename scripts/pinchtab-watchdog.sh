#!/bin/bash
# PinchTab Watchdog v3
# Monitors goclaw Chrome instance health. GoClaw manages its own instance lifecycle.
# Install: copy to ~/.pinchtab/watchdog.sh, set TOKEN, register with launchd (StartInterval=120)

TOKEN="${PINCHTAB_TOKEN:-your_token_here}"
BASE="http://localhost:9867"
LOG="/tmp/pinchtab-watchdog.log"

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG"
  tail -400 "$LOG" > "$LOG.tmp" 2>/dev/null && mv "$LOG.tmp" "$LOG"
}

# 1. Is pinchtab alive?
HTTP=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 3 "$BASE/health" 2>/dev/null)
if [ "$HTTP" != "401" ] && [ "$HTTP" != "200" ]; then
  log "WARN pinchtab not responding (HTTP=$HTTP)"
  exit 0
fi

# 2. Find goclaw instance
INSTANCES=$(curl -s --connect-timeout 5 -H "Authorization: Bearer $TOKEN" "$BASE/instances" 2>/dev/null)
GOCLAW_ID=$(echo "$INSTANCES" | python3 -c "
import sys,json
try:
    for i in json.load(sys.stdin):
        if i.get('profileName')=='goclaw': print(i['id']); break
except: pass
" 2>/dev/null)

# 3. No instance — GoClaw will create when needed
if [ -z "$GOCLAW_ID" ]; then
  log "INFO no goclaw instance — GoClaw will create when needed"
  exit 0
fi

# 4. Liveness probe — open blank tab
PROBE=$(curl -s --connect-timeout 8 -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"url":"about:blank"}' \
  "$BASE/instances/$GOCLAW_ID/tabs/open" 2>/dev/null)

TAB_ID=$(echo "$PROBE" | python3 -c "
import sys,json
try: print(json.load(sys.stdin).get('tabId',''))
except: pass
" 2>/dev/null)

if [ -n "$TAB_ID" ]; then
  curl -s -X DELETE -H "Authorization: Bearer $TOKEN" \
    "$BASE/tabs/$TAB_ID" > /dev/null 2>&1 || true
  log "OK $GOCLAW_ID alive (probe tab=$TAB_ID)"
else
  # Chrome dead — stop it, GoClaw will recreate on next request
  log "WARN $GOCLAW_ID probe failed (resp: ${PROBE:0:80}) — stopping dead instance"
  curl -s -X POST -H "Authorization: Bearer $TOKEN" \
    "$BASE/instances/$GOCLAW_ID/stop" > /dev/null 2>&1
  log "INFO stopped $GOCLAW_ID — GoClaw will recreate on next request"
fi
