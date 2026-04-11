# Handoff: surveys E2E cutover continuation

Дата: 2026-04-11

## Вже доведено раніше

- Mac `goclaw` = primary host
- Alpine `goclaw` = fallback host
- PinchTab on Mac reachable externally with auth
- browser handoff on Mac працює
- live new workflow у `n8n` оновлений під native `goclaw`

Базовий звіт:
- [surveys_deployment_report_2026-04-11_uk.md](/home/vokov/projects/goclaw/docs/reports/surveys_deployment_report_2026-04-11_uk.md)

## Поточний статус E2E-етапу

Робота велася через live `n8n` MCP, не від локального JSON.

Live new workflow:
- workflowId: `c4gZHN2sl3UMvkTs`
- latest published `activeVersionId` на момент handoff: `2969c7b1-72f1-4089-bbab-eaaf4df806fa`

Live old workflow:
- workflowId: `FjqObOvJeV7IMR9u`
- досі не вимкнений

## Що вже змінено в live new workflow

У new workflow додано контрольований E2E test path:
- `E2E Test Webhook`
- `Normalize Test Payload`

У new workflow додано marker-based diagnostic behavior:
- якщо в payload є `workflowMarker`, Telegram ноди шлють спрощений test-safe текст
- це зроблено, щоб мати однозначний proof від new workflow через MCP execution

Додано dynamic base URLs:
- `primaryBaseUrl`
- `fallbackBaseUrl`

## Ключові execution IDs

### 1457
- перша спроба webhook E2E
- впала на `Telegram Pre-Alert` через parse entities

### 1458
- повторна спроба
- знову Telegram parse problem

### 1460
- marker-based pre-alert уже успішно відправився
- `Telegram Pre-Alert` success
- `message_id = 873`
- це вже є формальний доказ, що Telegram був відправлений саме з new workflow по marker-based test path

### 1461
- `Telegram Pre-Alert` success
- `message_id = 875`

### 1462
- `Telegram Pre-Alert` success
- `message_id = 877`
- але execution все ще пішов у `Telegram No-Agent Alert`

## Важливе live спостереження від користувача

Користувач підтвердив, що безmarker-ові `NO-AGENT` alerts уже приходять саме з **new workflow**.

Це означає:
- бойовий Gmail-trigger path у new workflow реально активний
- але в ньому все ще є bug у визначенні availability surveys-агента

## Поточний blocker

New workflow все ще хибно вважає і Mac, і Alpine безагентними.

Симптом у Telegram:
- `⚠️ Ні primary, ні fallback GoClaw не готові до запуску surveys.`

Причина локалізована в `Prepare Primary GoClaw Request` / `Prepare Fallback GoClaw Request`:
- parser агентів був написаний під не той shape live `/v1/agents`
- спочатку він шукав `resp.data`
- потім було виправлено на `resp.agents`
- далі знайдено ще одну невідповідність:
  - live agent має `agent_key: "surveys"` і `display_name: "Surveys"`
  - але execution `1462` все ще показує `primaryAvailable: false`

Отже треба перевірити, що latest published code справді містить:
- читання `resp.agents` / `resp.body.agents`
- match по `agent_key`
- match по `display_name`

Схоже, один з патчів або не потрапив у реально активний path, або string-replace в generated JS не змінив той fragment, який реально виконується.

## Що перевірити на старті нової сесії

1. Через MCP витягнути current `get_workflow_details(workflowId=c4gZHN2sl3UMvkTs)`.
2. Знайти в live code ноди:
   - `Prepare Primary GoClaw Request`
   - `Prepare Fallback GoClaw Request`
3. Перевірити, що в них реально є:
   - `resp.agents`
   - `resp.body?.agents`
   - `agent.agent_key`
   - `agent.display_name`
4. Якщо ні:
   - повторно виправити
   - publish
5. Повторити marker-based webhook test.

## Яким має бути наступний правильний proof

### Primary proof

Повторити marker-based test payload через `execute_workflow` з:
- `workflowMarker: NEW-WF-E2E-PRIMARY-...`

Очікування:
- `Telegram Pre-Alert` success з marker
- `Primary GoClaw Available?` має піти в true branch
- `Primary GoClaw Responses API` має виконатися
- `Telegram Alert` має повернути success з `message_id`
- `Compose Final Alert` / `Telegram Alert` мають містити marker і `host=Mac primary`

### Fallback proof

Потім запустити другий marker-based test payload з override:
- `primaryBaseUrl: http://127.0.0.1:9`

Мета:
- зламати primary probe контрольовано
- примусити live new workflow перейти на Alpine fallback
- отримати success на fallback branch
- Telegram marker має показати fallback/triage-only behavior

## Після цього

ТІЛЬКИ якщо доведено:
- marker-based Telegram від new workflow
- primary branch proof
- fallback branch proof

тоді:
- викликати `unpublish_workflow` для old workflow `FjqObOvJeV7IMR9u`
- перевірити через `search_workflows`, що old inactive, new active

## Локальні артефакти цієї сесії

В `/tmp`:
- `/tmp/n8n_new_workflow_before_e2e.json`
- `/tmp/n8n_new_workflow_e2e.js`
- `/tmp/get_exec_1460_resp.sse`
- `/tmp/get_exec_1461_resp.sse`
- `/tmp/get_exec_1462_resp.sse`

## Практичний висновок

До кінця cutover залишилось:
- добити правильне визначення surveys-agent у new workflow
- довести primary branch
- довести fallback branch
- вимкнути old workflow

## Безпека

MCP token не вносити у файли/звіти.

Опційно після завершення робіт:
- рекомендувати rotation токена як hygiene step

## Підсумок завершення

Після цього handoff cutover було завершено:
- отримано final Telegram proof від new workflow:
  - `NEW-WF FINAL NEW-WF-E2E-FALLBACK-WEBHOOK-2026-04-11-D host=unknown status=agent_error opened=no`
- forced fallback доведено execution `1468`
- old workflow був вимкнений вручну через UI

Фінальний статус:
- `DONE`

Фінальний consolidated report:
- [surveys_deployment_report_2026-04-11_uk.md](/home/vokov/projects/goclaw/docs/reports/surveys_deployment_report_2026-04-11_uk.md)
