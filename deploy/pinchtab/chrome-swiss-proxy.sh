#!/bin/sh

exec /usr/bin/chromium-browser \
  --proxy-server=http://100.66.97.93:8888 \
  --proxy-bypass-list=localhost\;127.*\;100.66.97.93 \
  --enforce-webrtc-ip-permission-check \
  --webrtc-ip-handling-policy=disable_non_proxied_udp \
  --lang=de-CH \
  --accept-lang=de-CH,de,en \
  "$@"
