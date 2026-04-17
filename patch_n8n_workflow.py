#!/usr/bin/env python3
"""Patch n8n survey workflow to enable full auto-execution instead of handoff-only."""

import json
import sys
import requests

N8N_BASE = "https://n8n.exodus.pp.ua"
N8N_API_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhYjg1MjVmYy0wYjBhLTQzZDUtYmJmMS02ZjJkNjBiY2M3M2UiLCJpc3MiOiJuOG4iLCJhdWQiOiJwdWJsaWMtYXBpIiwianRpIjoiMTY1NjUxMmQtNDBiYi00ZWQ0LWI4NDgtYWZkYzQxZGM3MGM2IiwiaWF0IjoxNzc2MTEwMzY1fQ.3RIQmN0y4CB9ZBHO3G5sTHhuKA7MRjW7W8v4vyoVk50"
WORKFLOW_ID = "c4gZHN2sl3UMvkTs"

HEADERS = {
    "X-N8N-API-KEY": N8N_API_KEY,
    "Content-Type": "application/json",
}

# ─── OLD strings to replace ─────────────────────────────────────────────────

OLD_PRIMARY_INSTRUCTIONS = """const primaryInstructions = [
  'You are the Mac primary surveys handoff agent.',
  'Analyze the email payload and decide whether this is a real survey task.',
  'If route_mode is alpine_vpn_only or allow_mac_open is false, do not open any browser or page on Mac.',
  'In that case, return strict JSON only with openedTask=false and explain that this recipient must be handled on Alpine only.',
  'If taskLink is empty, do not open any browser or page; review the email only.',
  'If taskLink exists and allow_mac_open is true, you may use the allowed browser/login worker flow to open the task link and reach the handoff point only.',
  'Allowed handoff point: landing/start/continue page or the page immediately before the first survey answer.',
  'Never answer survey questions, never submit, never bypass CAPTCHA, never continue past the handoff point.',
  'Leave the tab open for the human if you successfully reached the handoff point.',
].join('\\n');"""

NEW_PRIMARY_INSTRUCTIONS = """const primaryInstructions = [
  'You are the Mac primary surveys EXECUTION agent. Execute the survey FULLY and autonomously using your workspace profile.',
  'Read your AGENTS.md and SOUL.md from your workspace first to load the Arno Dubois profile.',
  'EXECUTION STEPS:',
  '1. If route_mode is alpine_vpn_only or allow_mac_open is false: do not open any browser. Return openedTask=false explaining this recipient must run on Alpine only.',
  '2. If taskLink is empty: return needs_manual_review.',
  '3. Verify Swiss egress: run `curl -s https://ipinfo.io/country` — result must be exactly CH. Mac is physically in Switzerland, direct connection, no SOCKS5.',
  '   If result is not CH: return openedTask=false with reason vpn_failed.',
  '4. Open the taskLink via the browser tool.',
  '5. Complete ALL survey pages using the Arno Dubois profile from your workspace:',
  '   - Age 25, Gland 1196, sales, CHF 6001-8000, Tesla, Helvetia/Allianz+Helsana, UBS/BCV',
  '   - FINANCIAL INDEPENDENCE: ALWAYS answer "Je paie moi-meme" for payment questions',
  '   - Scores: 7-8 out of 10',
  '6. CRITICAL: NEVER combine answer selection AND pressing Continuer/Next/Submit in the same browser action. One action at a time.',
  '7. Continue through ALL pages until the FINAL thank-you page or screened-out page.',
  '8. After completing or being screened out, run the result pipeline:',
  '   ssh vokov@192.168.3.184 \'source ~/.goclaw/surveys/.env && ~/.goclaw/surveys/scripts/run_survey_result.sh arsen mac\'',
  '9. Return strict JSON with actual outcome.',
].join('\\n');"""

OLD_FALLBACK_INSTRUCTIONS = """const fallbackInstructions = [
  'You are the Alpine surveys agent.',
  'Analyze the email payload and decide whether this is a real survey task.',
  'If route_mode is alpine_vpn_only and taskLink exists, you may use the browser tool only after verifying Swiss egress first.',
  'Swiss egress check order:',
  '1. If vpn_country_override is present, use it as the diagnostic country result.',
  '2. Otherwise open https://ipinfo.io/country and verify the visible result is exactly CH before opening the survey task link.',
  'If the verified country is not CH, do not open the survey and return strict JSON with openedTask=false plus a clear reason that Alpine Swiss VPN is not active.',
  'If route_mode is not alpine_vpn_only, do not open browser windows and keep Alpine in triage/fallback mode only.',
  'Never answer survey questions, never submit, never bypass CAPTCHA, never continue past the handoff point.',
  'If Alpine is allowed to proceed, stop at landing/start/continue page or the page immediately before the first survey answer.',
].join('\\n');"""

NEW_FALLBACK_INSTRUCTIONS = """const fallbackInstructions = [
  'You are the Alpine surveys EXECUTION agent. Execute the survey FULLY and autonomously using your workspace profile.',
  'Read your AGENTS.md and SOUL.md from your workspace first to load the profile.',
  'EXECUTION STEPS:',
  '1. If route_mode is NOT alpine_vpn_only: do NOT open any browser. Return triage/fallback-only JSON with openedTask=false.',
  '2. If route_mode is alpine_vpn_only and taskLink exists:',
  '   a. Verify Swiss egress via SOCKS5: run `curl -s --socks5-hostname 100.100.74.9:9888 https://ipinfo.io/country`',
  '      OR if vpn_country_override is present, use it as the diagnostic result.',
  '   b. If result is not CH: return openedTask=false with reason vpn_failed — Swiss VPN is not active.',
  '   c. Open the taskLink via the browser tool using Swiss SOCKS5 proxy (100.100.74.9:9888).',
  '3. Complete ALL survey pages using the Annet Buonassie profile from your workspace:',
  '   - Age 52, Gland 1196, sales, CHF 6001-8000, Tesla, la Mubilière+Helsana, Salt, Revolut',
  '   - Scores: 7 out of 10, Agreement: Oui, plutôt d\\'accord',
  '4. CRITICAL: NEVER combine answer selection AND pressing Continuer/Next in the same browser action. One action at a time.',
  '5. Continue through ALL pages until the FINAL thank-you page or screened-out page.',
  '6. After completing or being screened out, run the result pipeline:',
  '   source ~/.goclaw/surveys/.env && ~/.goclaw/surveys/scripts/run_survey_result.sh olena alpine',
  '7. Return strict JSON with actual outcome.',
].join('\\n');"""

OLD_OUTPUT_CONTRACT = """'{"status":"ready_for_handoff|needs_manual_review|not_a_task|task_unavailable|agent_error","taskConfirmed":true,"openedTask":false,"taskLink":"","handoffLink":"","pageTitle":"","reward":"","duration":"","portalMismatch":null,"reason":"","nextAction":"","confidence":0.0}',"""

NEW_OUTPUT_CONTRACT = """'{"status":"completed|screened_out|failed|vpn_failed|needs_manual_review|not_a_task|task_unavailable|agent_error","taskConfirmed":true,"openedTask":false,"taskLink":"","currentUrl":"","pageTitle":"","reward":"","duration":"","portalMismatch":null,"reason":"","nextAction":"","confidence":0.0}',"""

# Compose Final Alert: add completed/screened_out/vpn_failed before agent_error catch-all
OLD_COMPOSE_STATUS = """} else if (status === 'task_unavailable') {
  lines.push(`🔴 Задача недоступна${shouldAppendReason(status, reasonSummary) ? ` — ${reasonSummary}` : ''}${confidenceText ? ` (впевненість ${confidenceText})` : ''}`);
  if (reward && reward !== 'невідомо') lines.push(`💰 ${reward}`);"""

NEW_COMPOSE_STATUS = """} else if (status === 'completed') {
  lines.push('✅ Опитування завершено');
  if (reward && reward !== 'невідомо') lines.push(`💰 Нагорода: ${reward}`);
} else if (status === 'screened_out') {
  lines.push('🚫 Відсіяно (screened out)');
  if (reward && reward !== 'невідомо') lines.push(`💰 ${reward}`);
} else if (status === 'vpn_failed') {
  lines.push('🔴 VPN не активний — Swiss IP не підтверджено (не CH)');
} else if (status === 'failed') {
  lines.push(`🔴 Виконання завершилось помилкою${shouldAppendReason(status, reasonSummary) ? ` — ${reasonSummary}` : ''}`);
  if (reward && reward !== 'невідомо') lines.push(`💰 ${reward}`);
} else if (status === 'task_unavailable') {
  lines.push(`🔴 Задача недоступна${shouldAppendReason(status, reasonSummary) ? ` — ${reasonSummary}` : ''}${confidenceText ? ` (впевненість ${confidenceText})` : ''}`);
  if (reward && reward !== 'невідомо') lines.push(`💰 ${reward}`);"""

# ─── summarizeReason update: add completed/screened_out/vpn_failed ────────────
OLD_SUMMARIZE = """  if (status === 'task_unavailable') return 'задача недоступна';
  if (status === 'needs_manual_review') return 'потрібна ручна перевірка';
  if (status === 'agent_error') return 'GoClaw не зміг коректно обробити лист';"""

NEW_SUMMARIZE = """  if (status === 'completed') return 'завершено';
  if (status === 'screened_out') return 'відсіяно';
  if (status === 'vpn_failed') return 'VPN не активний';
  if (status === 'failed') return 'виконання завершилось помилкою';
  if (status === 'task_unavailable') return 'задача недоступна';
  if (status === 'needs_manual_review') return 'потрібна ручна перевірка';
  if (status === 'agent_error') return 'GoClaw не зміг коректно обробити лист';"""


def get_workflow():
    r = requests.get(f"{N8N_BASE}/api/v1/workflows/{WORKFLOW_ID}", headers=HEADERS, timeout=30)
    r.raise_for_status()
    return r.json()


def put_workflow(workflow_data):
    # n8n API PUT only accepts specific settings properties
    raw_settings = workflow_data.get("settings", {})
    allowed_settings_keys = {"executionOrder", "saveManualExecutions", "callerPolicy",
                              "errorWorkflow", "timezone", "saveExecutionProgress",
                              "saveDataSuccessExecution", "saveDataErrorExecution"}
    clean_settings = {k: v for k, v in raw_settings.items() if k in allowed_settings_keys}

    payload = {
        "name": workflow_data["name"],
        "nodes": workflow_data["nodes"],
        "connections": workflow_data["connections"],
        "settings": clean_settings,
        "staticData": workflow_data.get("staticData"),
    }
    r = requests.put(
        f"{N8N_BASE}/api/v1/workflows/{WORKFLOW_ID}",
        headers=HEADERS,
        json=payload,
        timeout=30,
    )
    if not r.ok:
        print(f"PUT failed {r.status_code}: {r.text[:500]}")
        r.raise_for_status()
    return r.json()


def patch_node_code(code: str, old: str, new: str, label: str) -> str:
    if old not in code:
        print(f"  WARNING: '{label}' — old string NOT found in node code!")
        return code
    result = code.replace(old, new, 1)
    print(f"  PATCHED: {label}")
    return result


def main():
    print("Fetching live workflow...")
    wf = get_workflow()
    print(f"  Name: {wf['name']}, nodes: {len(wf['nodes'])}")

    patches_applied = 0

    for node in wf["nodes"]:
        name = node.get("name", "")

        if "Analyze Email" in name:
            print(f"\nPatching node: {name}")
            code = node["parameters"]["jsCode"]

            code = patch_node_code(code, OLD_PRIMARY_INSTRUCTIONS, NEW_PRIMARY_INSTRUCTIONS, "primaryInstructions")
            if OLD_PRIMARY_INSTRUCTIONS not in node["parameters"]["jsCode"] + " ":
                patches_applied += 1

            code = patch_node_code(code, OLD_FALLBACK_INSTRUCTIONS, NEW_FALLBACK_INSTRUCTIONS, "fallbackInstructions")
            if OLD_FALLBACK_INSTRUCTIONS not in code:
                patches_applied += 1

            code = patch_node_code(code, OLD_OUTPUT_CONTRACT, NEW_OUTPUT_CONTRACT, "outputContract")
            if OLD_OUTPUT_CONTRACT not in code:
                patches_applied += 1

            node["parameters"]["jsCode"] = code

        elif "Compose Final Alert" in name or "Compose" in name:
            print(f"\nPatching node: {name}")
            code = node["parameters"]["jsCode"]

            code = patch_node_code(code, OLD_COMPOSE_STATUS, NEW_COMPOSE_STATUS, "compose_status_completed+screened_out+vpn_failed")
            if OLD_COMPOSE_STATUS not in code:
                patches_applied += 1

            code = patch_node_code(code, OLD_SUMMARIZE, NEW_SUMMARIZE, "summarizeReason")
            if OLD_SUMMARIZE not in code:
                patches_applied += 1

            node["parameters"]["jsCode"] = code

    print(f"\nTotal patches applied: {patches_applied}")
    if patches_applied == 0:
        print("ERROR: No patches were applied. Check that old strings match exactly.")
        sys.exit(1)

    print("\nSaving backup of original workflow...")
    with open("/tmp/workflow_before_exec_fix.json", "w") as f:
        json.dump(wf, f, indent=2)
    print("  Backup saved to /tmp/workflow_before_exec_fix.json")

    print("\nUploading patched workflow to n8n...")
    result = put_workflow(wf)
    print(f"  Upload result: id={result.get('id')}, updatedAt={result.get('updatedAt')}")
    print("\nDone. Workflow updated to EXECUTION mode.")


if __name__ == "__main__":
    main()
