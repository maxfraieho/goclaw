# Alpine NVIDIA Minimax Migration Report

Date: 2026-04-15
Host: Alpine (192.168.3.184)

---

## Summary

Successfully migrated Alpine GoClaw surveys agent:
- From: claude2-cc / cc/claude-haiku-4-5
- To: nvidia-minimax-1 / minimaxai/minimax-m2.7

---

## Repository Status

| Item | Value |
|------|-------|
| Local path | /home/vokov/projects/goclaw |
| Branch | main |
| Fork remote | origin → maxfraieho/goclaw |
| Upstream remote | upstream → nextlevelbuilder/goclaw |
| Commits behind upstream | ~230 |

---

## UI Hot-Swap Bug

Bug: UI cached provider list with 60s stale time.

Fix: Already applied via commit 8aa351b0 (refreshProviders on dialog open).

No PR needed - fix already in upstream main.

---

## Agent Config

| Field | Value |
|-------|-------|
| Agent ID | 019d75bf-b32f-7eb6-8573-35dac8a56927 |
| Agent Key | surveys |
| Provider | nvidia-minimax-1 |
| Model | minimaxai/minimax-m2.7 |

---

## Provider Config

| Field | Value |
|-------|-------|
| Name | nvidia-minimax-1 |
| Type | openai_compat |
| API Base | https://integrate.api.nvidia.com/v1 |
| API Key | nvapi-*** (set) |

---

## Verification

curl -s -H Authorization "Bearer TOKEN" -H "X-GoClaw-User-Id: system" http://localhost:18790/v1/agents/019d75bf-b32f-7eb6-8573-35dac8a56927 | jq .provider, .model

Result: nvidia-minimax-1, minimaxai/minimax-m2.7

---

## Safety

- No surveys submitted
- No routing/Telegram changes
- Only surveys agent provider/model modified
