# GoClaw External Sync Integration Design

## 1. Goal

Побудувати безпечну і передбачувану синхронізацію між зовнішнім проєктом, де створюються:

- agent definitions
- persona descriptions
- knowledge/base docs
- per-user context presets

і GoClaw, де ці дані використовуються для runtime роботи агентів.

Цей документ спирається на підтверджену поточну модель GoClaw:

- source of truth для runtime context files у managed mode = БД
- безпечний шлях інтеграції = write-through через існуючі API/store-path
- `open` і `predefined` мають різні моделі зберігання

Пов’язаний звіт:

- [agent-behavior-storage-report.md](/Users/arsen111999/workspace/goclaw/docs/agent-behavior-storage-report.md)
- [agent-behavior-storage-summary.md](/Users/arsen111999/workspace/goclaw/docs/agent-behavior-storage-summary.md)

## 2. Scope

У scope:

- синхронізація metadata агента
- синхронізація shared persona/config files
- синхронізація heartbeat checklist/config
- sync strategy для per-user context
- sync strategy для knowledge/base docs
- conflict handling
- MVP і future phases

Поза scope:

- реалізація нового API в GoClaw
- прямі SQL migration/change proposals
- двостороння realtime sync engine

## 3. Integration principles

1. Не писати напряму в Postgres.
2. Не вважати workspace `.md` файлами джерелом істини.
3. Писати лише через API або через thin service wrappers над існуючим store-шаром.
4. Поважати різницю між `predefined` і `open`.
5. Не синхронізувати ephemeral bootstrap state як постійний артефакт.
6. Всі sync-операції мають бути idempotent.
7. Для файлових артефактів потрібна versioning/etag policy на стороні зовнішнього проєкту.

## 4. Recommended target model

### 4.1 Canonical ownership

Рекомендована ownership-модель:

- зовнішній проєкт є canonical source для:
  - `agent definition`
  - `SOUL.md`
  - `IDENTITY.md`
  - `AGENTS.md`
  - `USER_PREDEFINED.md`
  - `HEARTBEAT.md`
  - knowledge/base docs, які вважаються curated
- GoClaw є canonical source для:
  - runtime-generated `BOOTSTRAP.md` lifecycle
  - per-user evolved `USER.md`
  - user instance metadata
  - heartbeat run logs
  - session state
  - memory and KG artifacts, якщо вони генеруються самим агентом

### 4.2 Sync modes

Потрібно розділити sync на 3 режими:

1. `Provisioning sync`
   - create/update agent definition
   - create/update shared files
   - встановлення heartbeat checklist/config

2. `Import sync`
   - одноразове або batch завантаження великого набору контексту
   - через archive import

3. `Per-user sync`
   - опціональна робота з `USER.md`
   - лише для сценаріїв, де зовнішній проєкт реально володіє user profile data

## 5. Entity mapping

| External entity | GoClaw target | Sync direction | Preferred path |
|---|---|---|---|
| Agent slug/key | `agents.agent_key` | external -> GoClaw | HTTP `POST/PUT /v1/agents` |
| Display name | `agents.display_name` | external -> GoClaw | HTTP `PUT /v1/agents/{id}` |
| Provider/model/default configs | `agents.*`, JSONB config columns | external -> GoClaw | HTTP `PUT /v1/agents/{id}` |
| Agent type | `agents.agent_type` | create-time external -> GoClaw | HTTP `POST /v1/agents` |
| Expertise summary | `agents.frontmatter` | external -> GoClaw | HTTP `PUT /v1/agents/{id}` |
| `AGENTS.md` | `agent_context_files` or per-user for open | external -> GoClaw | WS `agents.files.set` for predefined only |
| `SOUL.md` | `agent_context_files` or per-user for open | external -> GoClaw | WS `agents.files.set` for predefined only |
| `IDENTITY.md` | `agent_context_files` or per-user for open | external -> GoClaw | WS `agents.files.set` for predefined only |
| `USER_PREDEFINED.md` | `agent_context_files` | external -> GoClaw | WS `agents.files.set` |
| `USER.md` default seed | `agent_context_files.USER.md` seed source for predefined | optional external -> GoClaw | future API or import path |
| Per-user `USER.md` | `user_context_files` | depends on ownership | existing HTTP instance endpoint |
| `BOOTSTRAP.md` | `user_context_files` | GoClaw-owned | do not sync after onboarding |
| `HEARTBEAT.md` | `agent_context_files.HEARTBEAT.md` | external -> GoClaw | WS `heartbeat.checklist.set` |
| Heartbeat schedule | `agent_heartbeats` | external -> GoClaw | WS `heartbeat.set` |
| Knowledge base docs | memory / KG import or curated workspace docs | external -> GoClaw | archive import or dedicated memory endpoints |

## 6. Recommended architecture

## 6.1 High-level flow

```text
External Project
  -> Sync Orchestrator
    -> GoClaw HTTP API
    -> GoClaw WebSocket RPC
    -> optional archive import channel
```

## 6.2 Sync orchestrator responsibilities

Окремий sync service або module у зовнішньому проєкті має:

- зберігати external revision/version для кожного synced object
- тримати mapping `external_agent_id -> goclaw_agent_id/goclaw_agent_key`
- нормалізувати файли перед sync
- знати тип агента (`open` / `predefined`)
- застосовувати різні policies для shared і per-user files
- писати audit trail sync-операцій у себе

## 6.3 Why mixed HTTP + WS

Поточний GoClaw API surface вже розділений:

- HTTP краще підходить для CRUD metadata, import/export, instance operations
- WS RPC потрібен для agent files і heartbeat checklist

Тому рекомендована інтеграція вже на старті має підтримувати обидва транспорти.

## 7. Preferred sync design by agent type

## 7.1 Predefined agents

Це preferred integration target.

### Syncable now

- `agents` metadata
- `AGENTS.md`
- `SOUL.md`
- `IDENTITY.md`
- `USER_PREDEFINED.md`
- `HEARTBEAT.md`
- heartbeat config
- export/import archive

### Recommended contract

- зовнішній проєкт володіє всіма shared persona files
- GoClaw володіє per-user `USER.md` після першої взаємодії, якщо нема окремої policy override
- `BOOTSTRAP.md` не синхронізується зовні

### Concrete write path

1. Ensure agent exists via HTTP create/update
2. Update shared files via WS `agents.files.set`
3. Update `HEARTBEAT.md` via WS `heartbeat.checklist.set`
4. Update heartbeat schedule via WS `heartbeat.set`
5. Optional: import curated memory/docs via archive import

## 7.2 Open agents

Не рекомендований перший target для інтеграції shared persona sync.

Причина:

- runtime files живуть per-user
- існуючий public API не дає повного per-user CRUD
- owner-level `agents.files.set` не є достатнім для реального runtime sync

### Recommended position

- у MVP не синхронізувати `open` agents як primary managed artifacts
- або конвертувати інтегровані агенти у `predefined`
- або додати в GoClaw новий per-user files API перед інтеграцією open-agent flow

## 8. Concrete sync mapping

## 8.1 Agent provisioning payload

Зовнішній проєкт має тримати canonical object такого рівня:

```json
{
  "agent_key": "sales-assistant",
  "display_name": "Sales Assistant",
  "agent_type": "predefined",
  "provider": "openai",
  "model": "gpt-5",
  "frontmatter": "B2B sales qualification and routing",
  "config": {
    "tools_config": {},
    "memory_config": {"enabled": true},
    "other_config": {
      "self_evolve": false
    }
  },
  "files": {
    "AGENTS.md": "...",
    "SOUL.md": "...",
    "IDENTITY.md": "...",
    "USER_PREDEFINED.md": "...",
    "HEARTBEAT.md": "..."
  },
  "heartbeat": {
    "enabled": true,
    "intervalSec": 1800,
    "timezone": "Europe/Zurich"
  }
}
```

## 8.2 GoClaw API mapping

### Step A: Upsert agent

Use:

- `POST /v1/agents` for create
- `PUT /v1/agents/{id}` for update

### Step B: Upsert shared files

Use WS:

- `agents.files.set`

Payload shape:

```json
{
  "agentId": "sales-assistant",
  "name": "SOUL.md",
  "content": "...",
  "propagate": false
}
```

### Step C: Upsert heartbeat checklist

Use WS:

- `heartbeat.checklist.set`

Payload shape:

```json
{
  "agentId": "agent-uuid",
  "content": "..."
}
```

### Step D: Upsert heartbeat schedule

Use WS:

- `heartbeat.set`

Payload shape:

```json
{
  "agentId": "agent-uuid",
  "enabled": true,
  "intervalSec": 1800,
  "timezone": "Europe/Zurich"
}
```

## 8.3 Per-user sync mapping

### Supported now

Only `USER.md`.

Use:

- `GET /v1/agents/{id}/instances/{userID}/files`
- `PUT /v1/agents/{id}/instances/{userID}/files/USER.md`

Recommended ownership policy:

- якщо `USER.md` в GoClaw формується розмовою, зовнішній проєкт не має його overwrite-ити blindly
- якщо зовнішній проєкт є canonical CRM/profile source, sync має бути patch/merge-oriented, не replace-oriented

### Recommended merge rules for `USER.md`

1. External owned fields:
   - canonical name
   - preferred language
   - timezone
   - static org/team metadata
2. GoClaw owned fields:
   - discovered preferences
   - interaction notes
   - style hints learned from chat
3. Integration should merge by section, not whole-file replacement

## 9. Knowledge/base docs sync

Є два практичні варіанти.

## 9.1 Curated docs via archive import

Коли база знань готується зовні як curated package:

- зібрати archive
- імпортувати як частину agent import

Плюси:

- один batch path
- вже сумісно з існуючим import/export

Мінуси:

- не granular
- важче робити часткові оновлення

## 9.2 Memory/document endpoints

Для більш granular sync використовувати memory endpoints:

- `GET /v1/agents/{agentID}/memory/documents`
- `PUT /v1/agents/{agentID}/memory/documents/{path...}`
- `DELETE /v1/agents/{agentID}/memory/documents/{path...}`
- `POST /v1/agents/{agentID}/memory/index`

Це краще для knowledge docs, ніж намагатися запхати все в persona `.md` files.

## 10. Conflict model

## 10.1 Revision model

Sync orchestrator має тримати для кожного synced entity:

- `external_revision`
- `last_synced_at`
- `last_synced_hash`
- `last_goclaw_hash`

## 10.2 Conflict classes

### Class A: External-only managed shared files

Файли:

- `AGENTS.md`
- `SOUL.md`
- `IDENTITY.md`
- `USER_PREDEFINED.md`
- `HEARTBEAT.md`

Policy:

- external wins
- якщо в GoClaw файл змінився локально, це drift
- drift логувати і або overwrite, або блокувати sync залежно від policy

### Class B: Mixed ownership files

Файли:

- `USER.md`

Policy:

- merge by section
- whole-file overwrite only if explicitly forced

### Class C: GoClaw-owned ephemeral files

Файли:

- `BOOTSTRAP.md`

Policy:

- never restore automatically after completion

## 10.3 Drift detection

Перед write операцією рекомендовано:

1. read current file from GoClaw
2. compare hash/content with `last_goclaw_hash`
3. if changed unexpectedly:
   - log drift
   - choose overwrite or manual review

## 11. MVP proposal

## Phase 1

Підтримати тільки `predefined` agents.

### Scope

- create/update agent metadata
- sync `AGENTS.md`
- sync `SOUL.md`
- sync `IDENTITY.md`
- sync `USER_PREDEFINED.md`
- sync `HEARTBEAT.md`
- sync heartbeat schedule

### Do not include yet

- open agents
- full per-user file sync
- two-way sync
- auto-import of runtime memory/KG back into external system

## Phase 2

Додати:

- import/export archive flow
- curated memory docs sync
- partial `USER.md` merge sync

## Phase 3

Додати або запропонувати в GoClaw:

- per-user files CRUD API
- bulk replace context API
- revision metadata for context files

## 12. Suggested implementation sequence

1. Build agent registry mapping in external project
2. Implement GoClaw auth + HTTP client
3. Implement GoClaw WS RPC client
4. Implement `upsertAgent()`
5. Implement `syncSharedFiles()`
6. Implement `syncHeartbeat()`
7. Add drift detection
8. Add archive import/export support
9. Add optional per-user `USER.md` merge sync

## 13. Proposed sync functions

```text
upsertAgentDefinition(externalAgent)
resolveGoClawAgent(externalAgent)
syncSharedFile(agentKey, fileName, content)
syncHeartbeatChecklist(agentUUID, content)
syncHeartbeatConfig(agentUUID, config)
syncKnowledgeDocs(agentUUID, docs)
syncUserProfile(agentUUID, userID, profileSections)
detectDrift(agentKey, fileName, expectedHash)
```

## 14. Error handling policy

### Retryable

- network/ws transport errors
- 5xx from GoClaw
- temporary provider/timeouts during import-related operations

### Non-retryable

- invalid `agent_type`
- invalid file name
- forbidden per-user file update
- unknown agent id/key
- attempting open-agent sync through predefined-only path

## 15. Security and safety constraints

- Sync token повинен мати лише потрібний scope
- Не дозволяти зовнішній системі довільний DB-level access
- Не синхронізувати secrets у markdown files
- Не зберігати per-user PII в shared files
- Не дозволяти auto-resummon як частину deterministic sync pipeline

## 16. Final recommendation

### Preferred sync architecture

- Canonical external source for shared agent definition
- Write-through sync into GoClaw via:
  - HTTP for agent metadata/import
  - WS RPC for agent files and heartbeat checklist
- MVP only for `predefined` agents

### Minimal viable integration path

1. Provision `predefined` agent via HTTP
2. Push shared persona files via `agents.files.set`
3. Push `HEARTBEAT.md` via `heartbeat.checklist.set`
4. Push heartbeat config via `heartbeat.set`
5. Optionally push curated knowledge docs through memory/import path

### Deferred until new API exists

- full `open` agent sync
- generic per-user files CRUD
- safe bidirectional sync of runtime-evolved files

## 17. Appendix: exact GoClaw touchpoints

- Shared files RPC:
  - [internal/gateway/methods/agents_files.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/agents_files.go)
- Context routing:
  - [internal/tools/context_file_interceptor.go](/Users/arsen111999/workspace/goclaw/internal/tools/context_file_interceptor.go)
- Agent HTTP CRUD:
  - [internal/http/agents.go](/Users/arsen111999/workspace/goclaw/internal/http/agents.go)
- Per-user instance API:
  - [internal/http/agents_instances.go](/Users/arsen111999/workspace/goclaw/internal/http/agents_instances.go)
- Heartbeat RPC:
  - [internal/gateway/methods/heartbeat.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/heartbeat.go)
- Import/export:
  - [internal/http/agents_import.go](/Users/arsen111999/workspace/goclaw/internal/http/agents_import.go)
  - [internal/store/pg/agents_export_queries.go](/Users/arsen111999/workspace/goclaw/internal/store/pg/agents_export_queries.go)
