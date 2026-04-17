# Pipeline Execution Fix — Session Report 2026-04-17

**Session type:** Bug fix + verification  
**Duration:** ~3 hours  
**Operator:** Claude Sonnet 4.6 on OrangePi PC2  
**Status:** ✅ All fixes applied, 13/13 verification checks passed

---

## Root Cause

The SPCDOpinion survey automation pipeline was **detecting tasks and sending Telegram notifications but NOT executing surveys**. The root cause was found in the `Analyze Email + Extract Link` JavaScript node of n8n workflow `c4gZHN2sl3UMvkTs`.

The `primaryInstructions` and `fallbackInstructions` strings embedded in that node contained **explicit handoff-only instructions**:

```
"Never answer survey questions, never submit, never bypass CAPTCHA,
never continue past the handoff point. Leave the tab open for the human."
```

These instructions were injected at the task level and **overrode** the execution-mode `AGENTS.md` that lived in the agent workspaces. GoClaw received the task message and followed the task-level instructions (stop at landing page) rather than the workspace-level ones (execute fully).

**Secondary bug (Telegram):** The Telegram Alert node defaulted to Markdown parse mode. Status values like `agent_error` contain `_` (underscore), which Telegram's Markdown parser treats as an unclosed italic entity — causing `400 Bad Request: Can't find end of the entity starting at byte offset 69`.

---

## Changes Applied

All changes were applied via n8n REST API (`PUT /api/v1/workflows/c4gZHN2sl3UMvkTs`).  
Patch script: `/home/vokov/projects/goclaw-local/patch_n8n_workflow.py`  
Workflow last updated: `2026-04-17T11:04:34.501Z`

### 1. `Analyze Email + Extract Link` node

**`primaryInstructions`** (Mac path, surveys-arsen / Arno Dubois):
- **Before:** "stop at handoff point, never answer questions, leave tab for human"
- **After:** Full execution steps: verify CH egress, open link, complete ALL pages, run result pipeline via SSH, return structured JSON

**`fallbackInstructions`** (Alpine path, surveys-olena / Annet Buonassie):
- **Before:** "stop at landing/start page, never answer questions"
- **After:** Full execution steps via SOCKS5 proxy (100.100.74.9:9888), complete ALL pages, run result pipeline locally

**Output contract** extended:
- Before: `ready_for_handoff | needs_manual_review | not_a_task | task_unavailable | agent_error`
- After: `completed | screened_out | failed | vpn_failed | needs_manual_review | not_a_task | task_unavailable | agent_error`

### 2. `Primary GoClaw Responses API` + `Fallback GoClaw Responses API` nodes

- Timeout: **180,000 ms → 600,000 ms** (3 min → 10 min)
- Reason: real survey execution requires 3–8 minutes; old timeout caused premature `agent_error`

### 3. `Compose Final Alert` node

- Added `htmlEscape` helper: `s => String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')`
- Applied to: subject, fromDisplay, taskLink, handoffLink, pageTitle
- Added status branches: `completed` ✅, `screened_out` 🚫, `vpn_failed` 🔴, `failed` 🔴
- Updated `summarizeReason()` to include new statuses in Ukrainian

### 4. All 3 Telegram nodes (`Telegram Alert`, `Telegram Pre-Alert`, `Telegram No-Agent Alert`)

- Added `parse_mode: "HTML"` to `additionalFields`
- Reason: n8n Telegram node defaults to Markdown; underscore in status names breaks Telegram entity parsing

### 5. MemPalace ChromaDB fix (on OrangePi)

File: `/home/vokov/.local/share/pipx/venvs/mempalace/lib/python3.13/site-packages/mempalace/backends/chroma.py`

```python
class _TinyEF:
    DIM = 512
    def name(self):          # ← added: ChromaDB 1.5.8 requires this method
        return "tiny_ef"
    def __call__(self, input):
        ...
```

Mining completed: 1072 files, 13,192 drawers indexed into MemPalace at `/home/vokov/projects/goclaw-local/.palace/`

---

## Verification

All 13 checks passed against live workflow (verified via GET after PUT):

| Check | Result |
|-------|--------|
| AnalyzeEmail.execution_mode_primary | ✅ YES |
| AnalyzeEmail.execution_mode_fallback | ✅ YES |
| AnalyzeEmail.no_handoff | ✅ YES |
| AnalyzeEmail.output_contract_completed | ✅ YES |
| Compose.htmlEscape | ✅ YES |
| Compose.status_completed | ✅ YES |
| Compose.status_screened_out | ✅ YES |
| Compose.status_vpn_failed | ✅ YES |
| Primary GoClaw API timeout | ✅ 600000 |
| Fallback GoClaw API timeout | ✅ 600000 |
| Telegram Alert parse_mode | ✅ HTML |
| Telegram Pre-Alert parse_mode | ✅ HTML |
| Telegram No-Agent Alert parse_mode | ✅ HTML |

Test execution 1521: `status=success`, no Telegram errors.

---

## What Was NOT Tested (Needs Follow-up)

1. **E2E with real survey task** — all tests used fake/test links. Need a live SPCDOpinion email to confirm full execution.
2. **Mac SSH result pipeline** — `surveys-arsen` instructions include `ssh vokov@192.168.3.184 "...run_survey_result.sh arsen mac"`. SSH key from Mac Docker container to Alpine not verified.
3. **Interruption handling** — no watchdog/heartbeat for hung GoClaw sessions. If survey takes >600s or agent crashes mid-page, execution silently fails.
4. **Duplicate detection** — if same task arrives twice, no dedup logic in current workflow.
5. **CAPTCHA/popup handling** — not explicitly handled in agent instructions.

---

## Files Changed This Session

| File | Type | Status |
|------|------|--------|
| n8n workflow `c4gZHN2sl3UMvkTs` | Live n8n | Updated via API |
| `/home/vokov/projects/goclaw-local/patch_n8n_workflow.py` | Python patch script | Created |
| `.../mempalace/backends/chroma.py` | Python lib | Patched (added `name()`) |
| This report | Docs | New |
| `docs/prompts/opus47-audit-v2-2026-04-17.md` | Docs | New |
