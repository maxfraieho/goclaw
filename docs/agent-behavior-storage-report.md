# 1. Executive summary

У managed/DB-backed GoClaw джерелом істини для поведінки агента є не `.md` на диску, а записи в БД, насамперед `agent_context_files` і `user_context_files`. Шаблони `.md` існують як embedded templates в коді і використовуються для seed/bootstrap, але runtime агента читає контекст через DB-backed `ContextFileInterceptor`, а не з файлової системи.

Різниця по типах агентів підтверджена:

- `predefined`: shared persona/config файли живуть в `agent_context_files`; `USER.md` і `BOOTSTRAP.md` живуть per-user в `user_context_files`
- `open`: поведінкові файли фактично живуть per-user в `user_context_files`; agent-level файли для них не seed-яться
- `HEARTBEAT.md`: окремий agent-level context file в `agent_context_files`, плюс schedule/config у таблиці `agent_heartbeats`

Синхронізація через API частково вже можлива, але surface асиметричний:

- agent-level files: є через WebSocket RPC `agents.files.list/get/set`
- per-user instance files: є HTTP list-all і edit only `USER.md`
- `HEARTBEAT.md`: є через WebSocket RPC `heartbeat.checklist.get/set`
- bulk sync: є через HTTP import/export архівів, які вже включають `context_files`, `user_context_files`, `user_profiles`

Висновок: для інтеграції найкращий шлях не прямий імпорт у Postgres, а write-through через існуючі store/service/API шари. Для повного CRUD по per-user files бракує публічного API.

# 2. Exact storage model

## Що зберігається в БД

- `agents`: метадані агента, тип, провайдер, модель, конфіг JSONB, `frontmatter`, `status`
  - [migrations/000001_init_schema.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000001_init_schema.up.sql)
- `agent_context_files`: shared agent files
  - `[agent_id, file_name, content]`
  - [migrations/000001_init_schema.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000001_init_schema.up.sql)
- `user_context_files`: per-user files
  - `[agent_id, user_id, file_name, content]`
  - [migrations/000001_init_schema.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000001_init_schema.up.sql)
- `user_agent_profiles`: per-user instance/profile/workspace/metadata
  - [migrations/000001_init_schema.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000001_init_schema.up.sql)
  - [migrations/000011_session_profile_metadata.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000011_session_profile_metadata.up.sql)
- `agent_heartbeats`, `heartbeat_run_logs`: heartbeat schedule/config/logs
  - [migrations/000022_agent_heartbeats.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000022_agent_heartbeats.up.sql)

## Що існує як `.md` шаблони

- Embedded templates: `AGENTS.md`, `SOUL.md`, `IDENTITY.md`, `USER.md`, `USER_PREDEFINED.md`, `BOOTSTRAP.md`, `BOOTSTRAP_PREDEFINED.md`, `TOOLS.md`
  - [internal/bootstrap/templates](/Users/arsen111999/workspace/goclaw/internal/bootstrap/templates)
- Вони embed-яться через `//go:embed templates/*.md`
  - [internal/bootstrap/seed.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed.go)

## Що генерується

- Для `predefined` агентів `SOUL.md`, `IDENTITY.md`, інколи `USER_PREDEFINED.md` генеруються LLM summoner-ом і записуються в `agent_context_files`
  - [internal/http/summoner.go](/Users/arsen111999/workspace/goclaw/internal/http/summoner.go)
  - [internal/http/summoner_regenerate.go](/Users/arsen111999/workspace/goclaw/internal/http/summoner_regenerate.go)
- `AGENTS.md` не генерується summoner-ом; це template/seed artifact
  - [internal/http/summoner.go](/Users/arsen111999/workspace/goclaw/internal/http/summoner.go)

## Що є шаблоном

- `AGENTS.md`, `SOUL.md`, `IDENTITY.md`, `USER.md`, `USER_PREDEFINED.md`, `BOOTSTRAP.md`, `BOOTSTRAP_PREDEFINED.md`, `TOOLS.md` початково походять з embedded templates
  - [internal/bootstrap/seed.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed.go)
- Далі реальна робоча копія для managed runtime переходить у БД

## Що є per-user

- `open` agents: `AGENTS.md`, `SOUL.md`, `IDENTITY.md`, `USER.md`, `BOOTSTRAP.md` у `user_context_files`
  - [internal/bootstrap/seed_store.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed_store.go)
- `predefined` agents: per-user тільки `USER.md` і `BOOTSTRAP.md`
  - [internal/bootstrap/seed_store.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed_store.go)

## Confirmed по окремих файлах

- `AGENTS.md`: template + DB content; open = per-user, predefined = agent-level
- `SOUL.md`: template seed; predefined може бути LLM-generated; open = per-user
- `IDENTITY.md`: template seed; predefined часто LLM-generated; open = per-user
- `USER.md`: always per-user at runtime; але для predefined може існувати agent-level default seed, який потім копіюється в per-user
- `USER_PREDEFINED.md`: agent-level only
- `BOOTSTRAP.md`: per-user only; видаляється з `user_context_files` після завершення onboarding
- `HEARTBEAT.md`: agent-level only, зберігається як context file в БД

## Про файлову систему

- `bootstrap.EnsureWorkspaceFiles()` все ще створює фізичні `.md` у workspace як fallback/backup, але managed runtime не використовує їх як primary source
  - [internal/bootstrap/seed.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed.go)
  - [internal/gateway/methods/agents_create.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/agents_create.go)

# 3. Exact code locations

| File path | Function / type / handler | Що відбувається |
|---|---|---|
| [migrations/000001_init_schema.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000001_init_schema.up.sql) | schema | Створює `agents`, `agent_context_files`, `user_context_files`, `user_agent_profiles`, `user_agent_overrides` |
| [migrations/000022_agent_heartbeats.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000022_agent_heartbeats.up.sql) | schema | Створює `agent_heartbeats`, `heartbeat_run_logs`, `agent_config_permissions` |
| [internal/bootstrap/seed.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed.go) | `templateFS`, `ReadTemplate`, `EnsureWorkspaceFiles` | Embed/read template files; optional filesystem seeding |
| [internal/bootstrap/seed_store.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed_store.go) | `SeedToStore`, `SeedUserFiles`, `EmbeddedUserFiles` | DB seeding logic для agent-level і per-user files |
| [internal/bootstrap/load_store.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/load_store.go) | `LoadFromStore` | Завантажує agent-level context files з БД |
| [internal/tools/context_file_interceptor.go](/Users/arsen111999/workspace/goclaw/internal/tools/context_file_interceptor.go) | `ContextFileInterceptor`, `ReadFile`, `WriteFile`, `LoadContextFiles` | Центральний router: читає/пише context files через БД залежно від `agent_type` і `user_id` |
| [internal/tools/filesystem.go](/Users/arsen111999/workspace/goclaw/internal/tools/filesystem.go) | `ReadFileTool.Execute` | `read_file` спочатку йде в context-file interceptor, не в disk |
| [internal/tools/edit.go](/Users/arsen111999/workspace/goclaw/internal/tools/edit.go) | `EditTool.Execute` | `edit` теж пише context files через interceptor |
| [internal/store/pg/agents_context.go](/Users/arsen111999/workspace/goclaw/internal/store/pg/agents_context.go) | `Get/SetAgentContextFile`, `Get/SetUserContextFile`, `DeleteUserContextFile`, `ListUserInstances`, `UpdateUserProfileMetadata` | Postgres CRUD по context/profile даних |
| [internal/store/agent_store.go](/Users/arsen111999/workspace/goclaw/internal/store/agent_store.go) | `AgentContextStore`, `AgentProfileStore` | Офіційні інтерфейси store-шару |
| [internal/agent/resolver.go](/Users/arsen111999/workspace/goclaw/internal/agent/resolver.go) | managed resolver | При створенні loop бере base context files з БД |
| [cmd/gateway_callbacks.go](/Users/arsen111999/workspace/goclaw/cmd/gateway_callbacks.go) | `buildContextFileLoader`, `buildSeedUserFiles` | Вшиває DB-backed loader/seed callbacks у agent loop |
| [internal/agent/loop_utils.go](/Users/arsen111999/workspace/goclaw/internal/agent/loop_utils.go) | `getOrCreateUserSetup` | На first chat створює profile, seed-ить per-user files |
| [internal/agent/loop_history.go](/Users/arsen111999/workspace/goclaw/internal/agent/loop_history.go) | `resolveContextFiles` | Мержить base files + per-user overrides перед system prompt |
| [internal/agent/systemprompt.go](/Users/arsen111999/workspace/goclaw/internal/agent/systemprompt.go) | `BuildSystemPrompt` | Інжектить persona/context files у system prompt |
| [internal/gateway/methods/agents_create.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/agents_create.go) | `handleCreate` | WS RPC create agent + seed DB files |
| [internal/gateway/methods/agents_files.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/agents_files.go) | `handleFilesList/Get/Set` | WS RPC для agent-level files |
| [internal/gateway/methods/agents_update.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/agents_update.go) | `handleUpdate` | WS RPC update agent + patch `IDENTITY.md` |
| [internal/http/agents.go](/Users/arsen111999/workspace/goclaw/internal/http/agents.go) | `RegisterRoutes`, `handleCreate/Get/Update/Delete`, `handleRegenerate`, `handleResummon` | HTTP agent CRUD і summoning actions |
| [internal/http/agents_instances.go](/Users/arsen111999/workspace/goclaw/internal/http/agents_instances.go) | `handleListInstances`, `handleGetInstanceFiles`, `handleSetInstanceFile` | HTTP API для per-user instances; write only `USER.md` |
| [internal/http/summoner.go](/Users/arsen111999/workspace/goclaw/internal/http/summoner.go) | `SummonAgent` | Генерує `SOUL.md`, `IDENTITY.md`, `USER_PREDEFINED.md` і зберігає в БД |
| [internal/http/summoner_regenerate.go](/Users/arsen111999/workspace/goclaw/internal/http/summoner_regenerate.go) | `RegenerateAgent`, `storeFiles` | LLM-based edit/regenerate поверх існуючих files |
| [internal/gateway/methods/heartbeat.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/heartbeat.go) | `handleChecklistGet/Set` | WS RPC CRUD для `HEARTBEAT.md`; окремо `handleGet/Set/Toggle/Test/Logs` для schedule/config |
| [internal/http/agents_import.go](/Users/arsen111999/workspace/goclaw/internal/http/agents_import.go) | `doMergeImport` | Bulk import у `agent_context_files`, `user_context_files`, `user_profiles` |
| [internal/store/pg/agents_export_queries.go](/Users/arsen111999/workspace/goclaw/internal/store/pg/agents_export_queries.go) | `ExportAgentContextFiles`, `ExportUserContextFiles`, `ExportUserProfiles` | Bulk export тих самих сутностей |

# 4. API surface

| Method | Route / RPC | Purpose | Request body | Response | Чи придатний для sync |
|---|---|---|---|---|---|
| `WS RPC` | `agents.files.list` | list agent-level files | `{agentId}` | `{files:[{name,missing,size}]}` | Частково |
| `WS RPC` | `agents.files.get` | get agent-level file | `{agentId,name}` | `{file:{name,content,...}}` | Так |
| `WS RPC` | `agents.files.set` | upsert agent-level file | `{agentId,name,content,propagate?}` | `{file,...,propagated}` | Так, найкращий поточний CRUD для shared files |
| `GET` | `/v1/agents/{id}/instances` | list user instances | path id | `{instances:[...]}` | Допоміжний |
| `GET` | `/v1/agents/{id}/instances/{userID}/files` | list all per-user files | path id,userID | `{files:[{agent_id,user_id,file_name,content}]}` | Так, read-only |
| `PUT` | `/v1/agents/{id}/instances/{userID}/files/{fileName}` | update per-user file | `{"content":"..."}` | `{"status":"updated"}` | Лише для `USER.md` |
| `PATCH` | `/v1/agents/{id}/instances/{userID}/metadata` | update `user_agent_profiles.metadata` | `{"metadata":{...}}` | `{"status":"updated"}` | Супутнє |
| `POST` | `/v1/agents/{id}/regenerate` | LLM edit persona files | `{"prompt":"..."}` | async/success | Не deterministic sync |
| `POST` | `/v1/agents/{id}/resummon` | regenerate predefined persona | no/implicit body | async/success | Ні, це re-generation |
| `WS RPC` | `heartbeat.checklist.get` | read `HEARTBEAT.md` | `{agentId}` | `{content}` | Так |
| `WS RPC` | `heartbeat.checklist.set` | write `HEARTBEAT.md` | `{agentId,content}` | `{ok,length}` | Так |
| `WS RPC` | `heartbeat.get/set/toggle/...` | heartbeat schedule/config | various | config/logs | Так, але не про markdown persona |
| `POST` | `/v1/agents/import` / `/v1/agents/{id}/import` | bulk import archive | tar.gz sections | summary/progress | Так, strongest bulk path |
| `GET` | `/v1/agents/{id}/export*` | bulk export archive | path/query | tar/preview | Так |

## Чого нема у готовому API

- Нема HTTP/WS endpoint для general per-user `get file` by filename
- Нема general per-user `create/update/delete file` для всіх file types
- Нема explicit `replace all context` endpoint
- Нема general HTTP endpoint для agent-level files; UI робить це через WS RPC
- Нема окремого endpoint для `SOUL.md`/`IDENTITY.md`/`AGENTS.md` per-user у open agents

## Внутрішні service-функції, які можна швидко обгорнути в API

- `AgentStore.GetUserContextFiles`, `SetUserContextFile`, `DeleteUserContextFile`
  - [internal/store/agent_store.go](/Users/arsen111999/workspace/goclaw/internal/store/agent_store.go)
- `ContextFileInterceptor.WriteFile` already contains correct routing rules
  - [internal/tools/context_file_interceptor.go](/Users/arsen111999/workspace/goclaw/internal/tools/context_file_interceptor.go)

# 5. Data flow

## Create agent

1. HTTP `POST /v1/agents` або WS `agents.create` створює row в `agents`
   - [internal/http/agents.go](/Users/arsen111999/workspace/goclaw/internal/http/agents.go)
   - [internal/gateway/methods/agents_create.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/agents_create.go)
2. `bootstrap.SeedToStore()` seed-ить agent-level templates для `predefined`; `open` skip
   - [internal/bootstrap/seed_store.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed_store.go)
3. Для `predefined` з description запускається `AgentSummoner`, який переписує/доповнює `SOUL.md`, `IDENTITY.md`, `USER_PREDEFINED.md` в БД
   - [internal/http/summoner.go](/Users/arsen111999/workspace/goclaw/internal/http/summoner.go)
4. Паралельно `EnsureWorkspaceFiles()` може створити physical workspace `.md` як fallback
   - [internal/bootstrap/seed.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed.go)

## First chat

1. Loop викликає `GetOrCreateUserProfile`
   - [internal/agent/loop_utils.go](/Users/arsen111999/workspace/goclaw/internal/agent/loop_utils.go)
2. Потім `SeedUserFiles(...)`
   - `open`: seed per-user `AGENTS/SOUL/IDENTITY/USER/BOOTSTRAP`
   - `predefined`: seed per-user `USER/BOOTSTRAP`
3. Якщо seed упав, loop використовує `EmbeddedUserFiles(...)` як in-memory fallback
   - [internal/bootstrap/seed_store.go](/Users/arsen111999/workspace/goclaw/internal/bootstrap/seed_store.go)

## Per-user file generation

- Не робиться через disk writes
- Робиться через `SetUserContextFile(...)` у `user_context_files`
- `BOOTSTRAP.md` після завершення onboarding видаляється через `DeleteUserContextFile(...)`
  - [internal/tools/context_file_interceptor.go](/Users/arsen111999/workspace/goclaw/internal/tools/context_file_interceptor.go)

## Runtime read path

1. Resolver бере base agent files з БД
   - [internal/agent/resolver.go](/Users/arsen111999/workspace/goclaw/internal/agent/resolver.go)
2. `ContextFileLoader -> ContextFileInterceptor.LoadContextFiles(...)` дістає per-user/user+agent files
   - [cmd/gateway_callbacks.go](/Users/arsen111999/workspace/goclaw/cmd/gateway_callbacks.go)
   - [internal/tools/context_file_interceptor.go](/Users/arsen111999/workspace/goclaw/internal/tools/context_file_interceptor.go)
3. `resolveContextFiles(...)` мержить per-user поверх base
   - [internal/agent/loop_history.go](/Users/arsen111999/workspace/goclaw/internal/agent/loop_history.go)
4. `BuildSystemPrompt(...)` інжектить їх у system prompt
   - [internal/agent/systemprompt.go](/Users/arsen111999/workspace/goclaw/internal/agent/systemprompt.go)

## Update path

- Через chat tools `write_file`/`edit`: interceptor пише в БД, не в disk
  - [internal/tools/filesystem.go](/Users/arsen111999/workspace/goclaw/internal/tools/filesystem.go)
  - [internal/tools/edit.go](/Users/arsen111999/workspace/goclaw/internal/tools/edit.go)
- Через owner UI files tab: WS `agents.files.get/set` для agent-level files
  - [ui/web/src/pages/agents/hooks/use-agent-detail.ts](/Users/arsen111999/workspace/goclaw/ui/web/src/pages/agents/hooks/use-agent-detail.ts)
- Через admin instance UI/API: only `USER.md` per-user over HTTP
  - [internal/http/agents_instances.go](/Users/arsen111999/workspace/goclaw/internal/http/agents_instances.go)
- `HEARTBEAT.md`: WS `heartbeat.checklist.get/set`
  - [internal/gateway/methods/heartbeat.go](/Users/arsen111999/workspace/goclaw/internal/gateway/methods/heartbeat.go)

# 6. Sync feasibility assessment

## Що вже можна синхронізувати прямо зараз

- `predefined` agent-level persona/config files через `agents.files.*`
- `HEARTBEAT.md` через `heartbeat.checklist.*`
- full archive import/export через `/v1/agents/.../import|export`
- per-user `USER.md` через instance HTTP endpoint
- `user_agent_profiles.metadata` через instance metadata endpoint

## Чого бракує

- General per-user CRUD для open-agent behavior files
- General per-user CRUD для predefined user-scoped files beyond `USER.md`
- Public `delete file` / `replace all files` endpoint
- Consistent HTTP API for files; зараз file CRUD розірваний між WS RPC, HTTP instance API, import/export

## Найкращий інтеграційний шлях

- Для shared persona/config: write-through через WS RPC `agents.files.set`
- Для bulk bootstrap/sync: HTTP import/export archive
- Для per-user owner profile: HTTP instance `USER.md`
- Для повного per-user sync потрібен новий thin API layer над `AgentStore.SetUserContextFile/DeleteUserContextFile`

# 7. Risks and constraints

- `open` vs `predefined` flows різні на рівні моделі даних; один API не можна проектувати без type-aware routing
- `open` agent files не представлені як agent-level UI files; owner dashboard не є джерелом правди для них
- `agents.files.set` працює лише з `agent_context_files`; для open agents це не покриває runtime persona
- `BOOTSTRAP.md` ephemeral: система його видаляє; зовнішній sync не повинен blindly recreate його після onboarding
- `USER.md` у predefined може бути і agent-level default, і per-user override; це двошарова модель
- `summoner/regenerate` може перезаписати `SOUL.md`, `IDENTITY.md`, `USER_PREDEFINED.md` і створити version drift
- Є кеші в `ContextFileInterceptor`; API write path має інвалідовувати кеші
- Race condition між зовнішнім sync і chat-driven self-updates
- `HEARTBEAT.md` і `agent_heartbeats` це дві різні сутності; треба синхронізувати обидві, якщо потрібна повна heartbeat behavior/config
- Прямий DB import обійде permission logic, cache invalidation і routing rules

# 8. Recommendation

## Preferred sync architecture

- Write-through integration поверх існуючих service/API шарів
- Shared definitions sync:
  - `agents` metadata via HTTP `PUT /v1/agents/{id}`
  - agent-level files via WS `agents.files.set`
  - heartbeat checklist via WS `heartbeat.checklist.set`
- Bulk onboarding/migration:
  - archive-based `/v1/agents/{id}/import`
- Per-user sync:
  - short-term only `USER.md` via existing instance API
  - medium-term додати dedicated per-user files API над `AgentStore`

## Minimal viable integration path

1. Синхронізувати `agent definition` і shared persona files для `predefined`
2. Синхронізувати `HEARTBEAT.md` окремо від heartbeat schedule/config
3. Не чіпати `BOOTSTRAP.md` після першого чату
4. Для per-user частини спочатку синхронізувати тільки `USER.md`
5. Для масових переносів/резервних копій використовувати export/import archives

## Optional future improvements

- Додати HTTP/WS endpoints:
  - `list/get/set/delete` per-user file
  - `replace context` для agent-level і per-user
- Додати revision/version field для context files
- Додати explicit sync service, який працює через `ContextFileInterceptor` замість прямого store

# 9. Appendix

## Точні команди пошуку

```bash
find . -maxdepth 2 -type d | sort
rg --files
rg -n --glob '!ui/**' --glob '!skills/**' --glob '!_readmes/**' --glob '!docs/**' \
  'AGENTS\.md|SOUL\.md|IDENTITY\.md|USER\.md|USER_PREDEFINED\.md|BOOTSTRAP\.md|BOOTSTRAP_PREDEFINED\.md|HEARTBEAT\.md|user_context_files|context files|predefined|open agent|agent files|summon|bootstrap|soul|identity|per-user' .
rg -n 'agent-files|agent files|user context|context files|predefined|open agent|summon|heartbeat' \
  ui/web/src/pages/agents ui/web/src/hooks ui/web/src/api
rg -n 'CREATE TABLE|ALTER TABLE|agent|user_context|heartbeat|bootstrap|agents_md|files' migrations
rg -n 'ContextFileLoader|LoadContextFiles|bootstrap.LoadFromStore|LoadWorkspaceFiles|FilterForSession|BuildContextFiles|context files' internal/agent cmd internal/http
```

## Важливі `rg/grep` знахідки

- `internal/bootstrap/templates/*.md` існують, але runtime routing іде через `ContextFileInterceptor`
- `internal/http/agents_instances.go`: `only USER.md can be edited via this endpoint`
- `internal/tools/context_file_interceptor.go`: routing rules:
  - open => all files per-user
  - predefined => `USER.md`/`BOOTSTRAP.md` per-user, решта agent-level
- `internal/http/summoner.go`: summoner генерує лише `SOUL.md`, `IDENTITY.md`, `USER_PREDEFINED.md`
- `internal/gateway/methods/heartbeat.go`: `HEARTBEAT.md` читається/пишеться як agent context file

## Ключові уривки коду з поясненням

```sql
CREATE TABLE agent_context_files (...)
CREATE TABLE user_context_files (...)
```

Пояснення: основна модель зберігання context files у БД, не на диску.

- [migrations/000001_init_schema.up.sql](/Users/arsen111999/workspace/goclaw/migrations/000001_init_schema.up.sql)

```go
if agentType == store.AgentTypeOpen && userID != "" {
    err := b.agentStore.SetUserContextFile(...)
}
```

Пояснення: open agents пишуть behavior files у `user_context_files`.

- [internal/tools/context_file_interceptor.go](/Users/arsen111999/workspace/goclaw/internal/tools/context_file_interceptor.go)

```go
if agentType == store.AgentTypePredefined && userID != "" && fileName == bootstrap.UserFile {
    err := b.agentStore.SetUserContextFile(...)
}
```

Пояснення: predefined агенти мають per-user write path лише для `USER.md` у chat/runtime.

- [internal/tools/context_file_interceptor.go](/Users/arsen111999/workspace/goclaw/internal/tools/context_file_interceptor.go)

```go
var summoningFiles = []string{
    bootstrap.SoulFile,
    bootstrap.IdentityFile,
    bootstrap.UserPredefinedFile,
}
```

Пояснення: summoner не генерує `AGENTS.md` і не працює з `BOOTSTRAP.md`.

- [internal/http/summoner.go](/Users/arsen111999/workspace/goclaw/internal/http/summoner.go)

```go
// Only USER.md can be edited via this endpoint
if fileName != "USER.md" { ... }
```

Пояснення: публічний per-user HTTP write API зараз дуже вузький.

- [internal/http/agents_instances.go](/Users/arsen111999/workspace/goclaw/internal/http/agents_instances.go)

## Confirmed / inferred

- Confirmed: DB є source of truth для managed runtime context files
- Confirmed: templates існують як embedded seed material
- Confirmed: filesystem `.md` у workspace не є primary runtime source в DB-backed mode
- Confirmed: UI file editing для agent detail йде через WS RPC, не через filesystem
- Inferred but strongly supported: прямий sync у Postgres технічно спрацює, але порушить intended service boundaries і дасть більше ризиків, ніж write-through API/store path
