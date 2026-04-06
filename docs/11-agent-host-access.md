# Agent Host Access — Projects & Docker

## Overview

За замовчуванням goclaw-агенти ізольовані в `/app/workspace`. Цей оверлей розширює доступ:

| Можливість | Стан за замовч. | Після оверлею |
|------------|----------------|---------------|
| Workspace | `/app/workspace` | `/home/vokov/projects` |
| `/home/vokov/projects` | недоступний | bind-mount RW |
| Docker daemon (`docker.sock`) | немає | bind-mount RO |
| `docker` CLI | не встановлений | встановлюється при старті |

---

## Активація

Файл: `~/projects/goclaw/docker-compose.projects-access.yml`

Запуск:

```bash
cd ~/projects/goclaw
docker compose \
  -f docker-compose.yml \
  -f docker-compose.override.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  -f docker-compose.alpine-pinchtab.yml \
  -f docker-compose.projects-access.yml \
  up -d --force-recreate goclaw
```

---

## Зміст оверлею

```yaml
# docker-compose.projects-access.yml
services:
  goclaw:
    entrypoint: ["/bin/sh", "-c", "apk add --no-cache docker-cli -q 2>/dev/null; exec /app/docker-entrypoint.sh \"$@\"", "--"]
    command: ["serve"]
    environment:
      - GOCLAW_WORKSPACE=/home/vokov/projects
    volumes:
      - /home/vokov/projects:/home/vokov/projects:rw
      - /var/run/docker.sock:/var/run/docker.sock:ro
    group_add:
      - "${DOCKER_GID}"
```

**Пояснення:**
- `entrypoint` встановлює `docker-cli` перед запуском goclaw (образ `kroschu/goclaw:pinchtab-fix` не включає його)
- `GOCLAW_WORKSPACE=/home/vokov/projects` — агенти бачать весь projects-каталог як workspace
- `docker.sock:ro` — дозволяє `docker ps`, `docker logs`, `docker exec`, але не `docker rm` та інші деструктивні операції через клієнт (клієнт все одно може, якщо має дозвіл на сокет; `ro` тільки означає readonly для самого файлу сокету в FS)
- `group_add: DOCKER_GID` — додає контейнер до групи `docker` хоста щоб мати права на сокет

### Передумова: DOCKER_GID у .env

```bash
# ~/projects/goclaw/.env
DOCKER_GID=103   # значення з: getent group docker | cut -d: -f3
```

Перевірити GID на хості:
```bash
getent group docker | cut -d: -f3
```

---

## Перевірка після запуску

```bash
# Workspace
docker exec goclaw-goclaw-1 sh -c 'echo $GOCLAW_WORKSPACE'
# → /home/vokov/projects

# Projects доступні
docker exec goclaw-goclaw-1 ls /home/vokov/projects

# Docker daemon доступний
docker exec goclaw-goclaw-1 docker ps --format '{{.Names}}'
```

---

## Примітки

- `docker-cli` встановлюється через `apk add` при кожному старті контейнера (ephemeral install). Базовий образ не містить його.
- `docker.sock` дає агентам повний доступ до Docker daemon хоста. Переконайтесь що агент (Claude CLI всередині goclaw) заслуговує на довіру перед активацією.
- Для продакшн-оточень де потрібен тільки read-only доступ до проектів — прибрати `docker.sock` і `group_add`.
