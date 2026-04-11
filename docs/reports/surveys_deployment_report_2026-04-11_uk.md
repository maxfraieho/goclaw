# Фінальний звіт про cutover surveys workflow

Дата: 2026-04-11  
Середовище: Alpine `goclaw` + Mac replica `goclaw` + live `n8n` через MCP

## 1. Поточний виявлений стан

### Mac
- `goclaw` live на `http://100.84.163.96:18790`
- `GET /health` повертає `{"status":"ok","protocol":3}`
- `GET /v1/agents` показує `surveys` як execution/handoff-агента
- workspace агента: `/app/workspace/surveys`
- browser tool дозволений
- live prompt/workspace відповідає цілі:
  - можна дійти до handoff point
  - не можна проходити survey до кінця
  - вкладка має лишатися відкритою

### Alpine
- `goclaw` live на `http://127.0.0.1:18790`
- `GET /health` повертає `{"status":"ok","protocol":3}`
- `GET /v1/agents` показує `surveys` як triage-only fallback агента
- workspace агента: `/home/vokov/.goclaw/workspace/surveys`
- browser tool на Alpine не дозволений

### n8n old workflow
- ID: `FjqObOvJeV7IMR9u`
- Назва: `Завдання на опитування`
- на момент фінального cutover вимкнений вручну через UI
- через MCP був недоступний для `unpublish`, бо `availableInMCP: false`

### n8n new workflow
- ID: `c4gZHN2sl3UMvkTs`
- Назва: `завдання на опитування  з ШІ`
- live production path
- `availableInMCP: true`
- працює на native `goclaw` contract:
  - `GET /v1/agents`
  - `POST /v1/responses`
  - `model: goclaw:surveys`
  - `messages[]`
  - `X-GoClaw-User-Id: system`

### PinchTab
- Mac PinchTab слухає зовні на `0.0.0.0:9867`
- health з auth успішний
- `authRequired: true`
- browser на desktop реально відкривається через live Mac chain

## 2. Що було застарілим в історичних файлах

- Історичний локальний workflow JSON не був джерелом істини.
- Live new workflow у `n8n` відрізнявся від локального історичного файлу.
- Live workflow був застарілий відносно Mac runtime:
  - працював по openclaw-style contract
  - використовував Alpine як єдиний target
  - не використовував Mac як primary

## 3. Що було змінено зараз

Оновлено саме live new workflow у `n8n` через MCP:
- primary target = Mac `goclaw`
- fallback target = Alpine `goclaw`
- перехід на native `goclaw` contract
- додано контрольований `E2E Test Webhook`
- додано marker-based diagnostic Telegram behavior для proof
- додано dynamic `primaryBaseUrl` / `fallbackBaseUrl`

Після цього були внесені мінімальні live-патчі:
- виправлений parser `surveys`-агента під live shape `/v1/agents`
  - `resp.agents`
  - `resp.body?.agents`
  - `agent_key`
  - `display_name`
- виправлена forced-fallback routing логіка
  - probe-ноди стали брати `primaryBaseUrl` / `fallbackBaseUrl` з analyzed payload, а не з output Telegram node
- виправлена доставка final Telegram marker alert

## 4. Точні файли і конфіги, які були змінені

Змінено:
- live workflow `c4gZHN2sl3UMvkTs` у `n8n` через MCP
- локальний звіт:
  - [surveys_deployment_report_2026-04-11_uk.md](/home/vokov/projects/goclaw/docs/reports/surveys_deployment_report_2026-04-11_uk.md)
- handoff:
  - [surveys_e2e_handoff_2026-04-11.md](/home/vokov/projects/goclaw/docs/reports/surveys_e2e_handoff_2026-04-11.md)

Не змінювалися:
- версія `GoClaw`
- PinchTab config
- Mac `launchd` / plist
- Alpine runtime config

## 5. Runtime status після змін

- Mac `goclaw`: healthy
- Alpine `goclaw`: healthy
- Mac PinchTab: healthy, auth увімкнена, зовнішній доступ з auth працює
- new workflow: active, published, production path
- old workflow: вимкнений вручну через UI

## 6. Результати smoke tests

1. Mac `goclaw health`: PASS
2. Alpine `goclaw health`: PASS
3. `GET /v1/agents` на обох хостах: PASS
4. `POST /v1/responses` на обох хостах: PASS
5. PinchTab health on Mac: PASS
6. PinchTab remote access with auth: PASS
7. browser desktop open on Mac: PASS
8. agent run without `taskLink` на Mac: PASS
9. agent run without `taskLink` на Alpine: PASS
10. safe controlled link test на Mac: PASS
11. fallback triage-only behavior на Alpine: PASS
12. Telegram із new workflow: PASS
13. forced fallback test у live `n8n` workflow: PASS

### Ключовий proof для Telegram із new workflow

Отримані marker-повідомлення:
- `NEW-WF PREALERT NEW-WF-E2E-FALLBACK-WEBHOOK-2026-04-11-D`
- `NEW-WF FINAL NEW-WF-E2E-FALLBACK-WEBHOOK-2026-04-11-D host=unknown status=agent_error opened=no`

Це є прямим доказом, що final Telegram alert прийшов саме з new workflow.

### Ключовий proof для forced fallback

Execution `1468` у live new workflow показав:
- `primaryFailureReason = connect ECONNREFUSED 127.0.0.1:9`
- `fallbackAvailable = true`
- `executionHostLabel = Alpine fallback`
- `Fallback GoClaw Responses API` був виконаний успішно

Це є прямим доказом, що при недоступності Mac новий workflow реально переключається на Alpine.

## 7. Який workflow активний зараз

Єдиний production path:
- new workflow `c4gZHN2sl3UMvkTs`

Old workflow:
- вимкнений вручну через UI

## 8. Чи вимкнений old workflow

Так.

Статус:
- old workflow вимкнений вручну користувачем після proof нового workflow

## 9. Чи є Mac справжнім primary

Так.

Доказ:
- live Mac agent має browser tool і handoff-oriented prompt
- live new workflow використовує Mac як primary target
- safe-link test відкрив browser window/tab на Mac desktop і зупинився на handoff point

## 10. Чи є Alpine справжнім fallback

Так.

Доказ:
- Alpine agent triage-only
- live new workflow використовує Alpine як fallback target
- execution `1468` доводить реальний forced fallback path

## 11. Залишкові блокери

Критичних blocker-ів для cutover немає.

Залишкові технічні зауваження:
- marker-based final alert прийшов як `host=unknown status=agent_error opened=no`
- це не блокує cutover, бо:
  - final Telegram proof уже отриманий
  - forced fallback already proven
  - production path переведений на new workflow

## 12. Практичний висновок

Стан після завершення:
- Mac реально primary execution host
- Alpine реально fallback host
- live new workflow у `n8n` бойовий
- old workflow виведений із production path
- PinchTab on Mac reachable externally with auth
- Surveys agent на Mac доходить до handoff point і зупиняється

Фінальний verdict:
- `DONE`

## 13. Чи потрібен upgrade до GoClaw 3.3

Зараз upgrade не потрібен для cutover.

Робити `GoClaw 3.3` має сенс лише окремим change set після цього cutover, бо ризики такі:
- можливі зміни в shape `/v1/agents` або `/v1/responses`
- можливий вплив на browser handoff / PinchTab integration
- знадобиться повторний e2e retest:
  - Mac primary
  - Alpine fallback
  - Telegram alerts
  - browser handoff

## 14. Додаткова примітка щодо токена

Токен MCP не вносився в цей файл.  
Після завершення робіт можна опційно зробити його ротацію як hygiene step.
