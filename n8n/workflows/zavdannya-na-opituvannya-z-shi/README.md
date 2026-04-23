# Завдання на опитування з ШІ

Окремий експорт n8n workflow:

- source workflow id: `c4gZHN2sl3UMvkTs`
- source name: `завдання на опитування  з ШІ`

## Files

- `workflow.json` — sanitized export for import into another `n8n`
- `.env.example` — env vars used by the sanitized workflow

## What was sanitized

Hardcoded runtime values were replaced with env-based values where practical:

- primary GoClaw base URL
- fallback GoClaw base URL
- primary GoClaw bearer token
- fallback GoClaw bearer token
- Telegram chat id for alerts

## Still requires manual relinking after import

This export still references n8n credentials by name/id. On the target n8n instance, reattach:

- Gmail trigger credential
- Telegram credential(s)

If those credentials do not exist yet, create them first and then reconnect the nodes.

## Import

1. Open n8n.
2. Import `workflow.json`.
3. Set the env vars from `.env.example`.
4. Reattach Gmail and Telegram credentials.
5. Verify the HTTP request nodes point to the intended GoClaw hosts.
6. Run a manual test before activation.
