# Agent Behavior Storage Summary

## Коротко

У GoClaw поведінка агента в managed mode визначається переважно даними в БД, а не фізичними `.md` файлами в workspace. Основні таблиці:

- `agent_context_files`
- `user_context_files`
- `user_agent_profiles`
- `agent_heartbeats`

Embedded `.md` templates існують у `internal/bootstrap/templates`, але вони використовуються як seed/bootstrap, а не як runtime source of truth.

## Розподіл по типах агентів

- `predefined`
  - shared files: `AGENTS.md`, `SOUL.md`, `IDENTITY.md`, `USER_PREDEFINED.md`, `HEARTBEAT.md`
  - per-user files: `USER.md`, `BOOTSTRAP.md`
- `open`
  - фактично весь поведінковий набір живе per-user:
  - `AGENTS.md`, `SOUL.md`, `IDENTITY.md`, `USER.md`, `BOOTSTRAP.md`

## Що важливо для інтеграції

- agent-level files уже можна синхронізувати через WS RPC `agents.files.get/set`
- `HEARTBEAT.md` уже можна синхронізувати через WS RPC `heartbeat.checklist.get/set`
- per-user `USER.md` уже можна читати/оновлювати через HTTP instance endpoints
- повного public CRUD для всіх per-user files зараз нема
- для bulk sync уже є import/export архівів

## Найкраща стратегія

- не писати напряму в Postgres
- використовувати write-through поверх існуючих API/store-path
- для shared persona/config використовувати `agents.files.set`
- для bulk-операцій використовувати import/export
- для per-user sync короткостроково обмежитися `USER.md`

## Головний ризик

Не можна проектувати sync без урахування різниці між `open` і `predefined`, бо в них різна модель зберігання і різний runtime read/write path.
