#!/bin/sh
set -eu

HEALTH_URL="${GOCLAW_HEALTH_URL:-http://127.0.0.1:18790/health}"
STATE_FILE="${GOCLAW_WATCHDOG_STATE_FILE:-/tmp/goclaw-healthcheck.failcount}"
LOG_FILE="${GOCLAW_WATCHDOG_LOG_FILE:-/var/log/goclaw-watchdog.log}"
SERVICE_NAME="${GOCLAW_SERVICE_NAME:-goclaw}"
TIMEOUT_SECONDS="${GOCLAW_WATCHDOG_TIMEOUT:-8}"
MAX_FAILURES="${GOCLAW_WATCHDOG_MAX_FAILURES:-2}"

if curl -fsS --max-time "$TIMEOUT_SECONDS" "$HEALTH_URL" >/dev/null 2>&1; then
    rm -f "$STATE_FILE"
    exit 0
fi

failures=1
if [ -f "$STATE_FILE" ]; then
    previous="$(cat "$STATE_FILE" 2>/dev/null || printf '0')"
    case "$previous" in
        ''|*[!0-9]*)
            previous=0
            ;;
    esac
    failures=$((previous + 1))
fi

printf '%s\n' "$failures" > "$STATE_FILE"
timestamp="$(date -Iseconds)"
printf '%s watchdog: healthcheck failed (%s/%s) for %s\n' \
    "$timestamp" "$failures" "$MAX_FAILURES" "$HEALTH_URL" >> "$LOG_FILE"

if [ "$failures" -lt "$MAX_FAILURES" ]; then
    exit 0
fi

rm -f "$STATE_FILE"
printf '%s watchdog: restarting %s after repeated healthcheck failures\n' \
    "$timestamp" "$SERVICE_NAME" >> "$LOG_FILE"
exec rc-service "$SERVICE_NAME" restart
