# GoClaw macOS Deployment — Resume Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Відновити розгортання з точки зупинки — Docker не встановлений, репо є але стара версія. Довести систему до робочого стану.

**Architecture:** Спочатку Docker, потім `git pull` (обов'язково!), потім re-apply macOS-специфічні фікси, потім запуск стека.

**Tech Stack:** Docker Desktop for Mac, docker-compose, ghcr.io/nextlevelbuilder/goclaw, pgvector/pgvector:pg18

---

## Контекст: що вже зроблено

| Task | Стан | Примітка |
|------|------|---------|
| Task 1 (SSH, перевірка) | ✅ | Зроблено |
| Task 2 (клонування) | ✅ | Але **стара версія** — потрібен `git pull` |
| Task 3 (.env) | ✅ | Але після `git pull` можуть бути зміни — перевірити |
| Task 4 (override.yml) | ✅ | **Буде скинуто** `git pull` якщо є локальні зміни — re-apply |
| Task 5+ (запуск) | ❌ | Docker не встановлений — заблоковано |

---

## Змінні

```
SSH_HOST=100.84.163.96
SSH_USER=arsen111999
TARGET_DIR=~/workspace/goclaw
```

---

## Task 1: Встановити та запустити Docker

### Step 1: Перевірити чи Docker вже є

```bash
docker --version 2>/dev/null && echo "INSTALLED" || echo "NOT INSTALLED"
which docker
```

**Якщо `INSTALLED` і `docker ps` без помилок → перейти одразу до Task 2.**

### Step 2: Встановити Docker Desktop через brew

```bash
brew install --cask docker
```

Якщо brew теж немає:
```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
# Після інсталяції виконати команди що brew вивів у "Next steps:"
# Зазвичай: eval "$(/opt/homebrew/bin/brew shellenv)"
brew install --cask docker
```

### Step 3: Запустити Docker Desktop

```bash
open /Applications/Docker.app
```

Дочекатись запуску — у menu bar з'явиться іконка Docker, статус "Docker Desktop is running".

Перевірити:
```bash
# Чекати до 60 секунд
docker ps
# → пустий список (не помилка)
```

**Якщо помилка `Cannot connect to the Docker daemon`:** Docker Desktop ще не запустився. Зачекати ще 30 секунд, повторити `docker ps`.

### Step 4: Переконатися що docker compose (v2) є

```bash
docker compose version
# → Docker Compose version v2.x.x
```

---

## Task 2: Оновити репо до актуальної версії (ОБОВ'ЯЗКОВО)

> Вчора було клоновано стару версію. З того часу в `maxfraieho/goclaw` main були запушені критичні зміни: PinchTab інтеграція, виправлений `docker-compose.override.yml` структура, fixes для `Backend` interface.

### Step 1: Перейти в директорію

```bash
cd ~/workspace/goclaw
git status
git log --oneline -3
```

Якщо є **незафіксовані зміни** в `docker-compose.override.yml` (macOS path fix зроблений вчора):

```bash
git diff docker-compose.override.yml
# Якщо є зміна /home/vokov/.claude → /Users/arsen111999/.claude — запам'ятати
```

### Step 2: Зберегти локальні зміни якщо є

```bash
# Якщо є uncommitted changes — zберегти в stash
git stash push -m "mac-override-path-fix"
# або просто скинути — fix буде re-applied нижче
git checkout -- docker-compose.override.yml
```

### Step 3: Оновити до останнього коміту

```bash
git fetch origin
git pull --ff-only origin main
```

Очікуваний результат: `Fast-forward` з кількома новими комітами.

**Якщо `Already up to date`** — перевірити що хеш актуальний:
```bash
git log --oneline -5
# Перший рядок має бути: 1f291471 docs(deploy): fix macOS plan — PinchTab URL...
# або пізніший
```

Якщо хеш старий і pull не допоміг:
```bash
git fetch origin --force
git reset --hard origin/main
```

### Step 4: Підтвердити актуальний стан

```bash
git log --oneline -5
ls docker-compose*.yml
# Має бути: docker-compose.yml, docker-compose.postgres.yml,
#            docker-compose.selfservice.yml, docker-compose.override.yml
```

---

## Task 3: Re-apply macOS фікс в docker-compose.override.yml

> Після `git pull` файл скинутий до Linux шляху. Треба знову виправити.

### Step 1: Переглянути поточний стан

```bash
cat ~/workspace/goclaw/docker-compose.override.yml
```

Очікуваний Linux вміст (з репо):
```yaml
services:
  goclaw:
    volumes:
      - /home/vokov/.claude:/root/.claude:ro
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}
```

### Step 2: Виправити шлях для macOS

```bash
sed -i '' 's|/home/vokov/.claude|/Users/arsen111999/.claude|g' \
  ~/workspace/goclaw/docker-compose.override.yml
```

### Step 3: Перевірити

```bash
grep claude ~/workspace/goclaw/docker-compose.override.yml
# → /Users/arsen111999/.claude:/root/.claude:ro
```

### Step 4: Переконатись що ~/.claude існує

```bash
ls ~/.claude 2>/dev/null && echo "EXISTS" || mkdir -p ~/.claude && echo "CREATED"
```

---

## Task 4: Перевірити/оновити .env

> `.env` був створений вчора — він не перезаписується `git pull`. Але треба перевірити наявність всіх потрібних змінних.

### Step 1: Перевірити наявність ключових змінних

```bash
grep -E 'GOCLAW_GATEWAY_TOKEN|GOCLAW_ENCRYPTION_KEY' ~/workspace/goclaw/.env
# Обидва мають бути непустими
```

Якщо якийсь відсутній або порожній:
```bash
cd ~/workspace/goclaw && chmod +x prepare-env.sh && ./prepare-env.sh
```

### Step 2: Перевірити Claude proxy та postgres

```bash
grep -E 'GOCLAW_ANTHROPIC|POSTGRES_PASSWORD|POSTGRES_USER|POSTGRES_DB' ~/workspace/goclaw/.env
```

Має бути:
```
GOCLAW_ANTHROPIC_BASE_URL=http://100.65.225.122:8084
GOCLAW_ANTHROPIC_API_KEY=<щось непусте>
POSTGRES_PASSWORD=goclaw
POSTGRES_USER=goclaw
POSTGRES_DB=goclaw
```

**Якщо чогось немає** — дописати:
```bash
cat >> ~/workspace/goclaw/.env << 'EOF'

# Claude proxy
GOCLAW_ANTHROPIC_BASE_URL=http://100.65.225.122:8084
GOCLAW_ANTHROPIC_API_KEY=sk-ant-placeholder

# Postgres (явно щоб не конфліктувало з іншими проектами в shell)
POSTGRES_PASSWORD=goclaw
POSTGRES_USER=goclaw
POSTGRES_DB=goclaw
EOF
```

### Step 3: Перевірити PinchTab URL

```bash
grep PINCHTAB ~/workspace/goclaw/.env
```

Якщо є `GOCLAW_BROWSER_PINCHTAB_URL=http://172.17.0.1:9867` (Linux docker0) — це не працює на Mac:

**Якщо PinchTab встановлений на Mac:**
```bash
sed -i '' 's|GOCLAW_BROWSER_PINCHTAB_URL=.*|GOCLAW_BROWSER_PINCHTAB_URL=http://host.docker.internal:9867|' \
  ~/workspace/goclaw/.env
```

**Якщо PinchTab на Mac не встановлений:**
```bash
sed -i '' '/GOCLAW_BROWSER_PINCHTAB_URL/d' ~/workspace/goclaw/.env
sed -i '' '/GOCLAW_BROWSER_PINCHTAB_TOKEN/d' ~/workspace/goclaw/.env
```

### Step 4: Фінальна перевірка .env

```bash
grep -E 'GOCLAW_GATEWAY_TOKEN|GOCLAW_ENCRYPTION_KEY|GOCLAW_ANTHROPIC_BASE_URL|POSTGRES_PASSWORD' \
  ~/workspace/goclaw/.env
# Всі 4 рядки мають бути непустими
chmod 600 ~/workspace/goclaw/.env
```

---

## Task 5: Запуск стека

### Step 1: Перевірити що Docker Desktop запущений

```bash
docker ps
# Якщо помилка — відкрити Docker.app та зачекати
```

### Step 2: Запустити повний стек

```bash
cd ~/workspace/goclaw
POSTGRES_PASSWORD=goclaw docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  -f docker-compose.override.yml \
  up -d
```

Перший запуск завантажує образи (~500MB+). Дочекатись повного завершення.

### Step 3: Перевірити статус контейнерів

```bash
POSTGRES_PASSWORD=goclaw docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  ps
```

Очікувано:
```
NAME          STATUS
goclaw        running
goclaw-ui     running
postgres      running (healthy)
```

Якщо `upgrade` контейнер — він має завершитись з `Exit 0`:
```bash
docker ps -a --format "table {{.Names}}\t{{.Status}}" | grep upgrade
# → upgrade   Exited (0) ...   ← OK
# → upgrade   Exited (1) ...   ← помилка, дивись логи нижче
```

### Step 4: Перевірити логи при проблемах

```bash
# Логи goclaw
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  logs goclaw --tail=40

# Логи upgrade контейнера якщо exit non-zero
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  logs upgrade --tail=40
```

**Типові помилки:**

| Помилка в логах | Причина | Рішення |
|-----------------|---------|---------|
| `FATAL: password authentication failed` | `POSTGRES_PASSWORD` у shell не `goclaw` | Завжди запускати з `POSTGRES_PASSWORD=goclaw docker compose ...` |
| `no such host: postgres` | Запуск без `docker-compose.postgres.yml` | Додати `-f docker-compose.postgres.yml` |
| `required env GOCLAW_ENCRYPTION_KEY is empty` | `.env` не підхоплений | Перевірити `cat .env` — файл має існувати в `~/workspace/goclaw/` |
| `port 5432 already in use` | Локальний Postgres на Mac | `brew services stop postgresql@<version>` |
| `port 18790 already in use` | Інша інстанція | `lsof -i :18790` → `kill <PID>` |

---

## Task 6: Health Check

### Step 1: GoClaw API

```bash
curl -s http://localhost:18790/health
# → {"status":"ok"} або HTTP 200
```

### Step 2: Web UI

```bash
curl -sI http://localhost:3000 | head -3
# → HTTP/1.1 200 OK
```

### Step 3: Claude proxy доступний з Mac

```bash
curl -sv --max-time 5 http://100.65.225.122:8084 2>&1 | grep -E 'Connected|HTTP|refused'
```

### Step 4: Відкрити Dashboard у браузері

```
http://localhost:3000
```

Має відкритись React SPA GoClaw dashboard.

---

## Task 7: launchd автозапуск

> Якщо вже налаштовано вчора — перевірити Step 1 і пропустити.

### Step 1: Перевірити чи є

```bash
ls ~/Library/LaunchAgents/com.goclaw.gateway.plist 2>/dev/null && echo "EXISTS" || echo "NOT FOUND"
launchctl list | grep goclaw
```

**Якщо є і `list` показує job → пропустити Task 7 повністю.**

### Step 2: Визначити шлях до docker

```bash
which docker
# Apple Silicon brew: /opt/homebrew/bin/docker
# Intel brew: /usr/local/bin/docker
# Docker Desktop: /usr/local/bin/docker → /Applications/Docker.app/...
```

### Step 3: Створити plist

```bash
mkdir -p ~/Library/LaunchAgents
DOCKER_PATH=$(which docker)
cat > ~/Library/LaunchAgents/com.goclaw.gateway.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.goclaw.gateway</string>

    <key>ProgramArguments</key>
    <array>
        <string>/bin/bash</string>
        <string>-c</string>
        <string>cd /Users/arsen111999/workspace/goclaw &amp;&amp; POSTGRES_PASSWORD=goclaw ${DOCKER_PATH} compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml -f docker-compose.override.yml up -d</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <false/>

    <key>StandardOutPath</key>
    <string>/Users/arsen111999/Library/Logs/goclaw-launch.log</string>

    <key>StandardErrorPath</key>
    <string>/Users/arsen111999/Library/Logs/goclaw-launch-err.log</string>

    <key>WorkingDirectory</key>
    <string>/Users/arsen111999/workspace/goclaw</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
        <key>HOME</key>
        <string>/Users/arsen111999</string>
        <key>POSTGRES_PASSWORD</key>
        <string>goclaw</string>
    </dict>
</dict>
</plist>
EOF
```

### Step 4: Перевірити та завантажити

```bash
plutil -lint ~/Library/LaunchAgents/com.goclaw.gateway.plist
# → OK

launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.goclaw.gateway.plist
launchctl list | grep goclaw
# → рядок з com.goclaw.gateway
```

---

## Task 8: Shell aliases

```bash
# Перевірити чи вже є
grep goclaw-up ~/.zshrc && echo "EXISTS" || echo "NOT FOUND"
```

Якщо **NOT FOUND**:
```bash
cat >> ~/.zshrc << 'ZSHEOF'

# ── GoClaw ──
export GOCLAW_ANTHROPIC_BASE_URL="http://100.65.225.122:8084"
alias goclaw-up='cd ~/workspace/goclaw && POSTGRES_PASSWORD=goclaw docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml -f docker-compose.override.yml up -d'
alias goclaw-down='cd ~/workspace/goclaw && docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml down'
alias goclaw-logs='cd ~/workspace/goclaw && docker compose -f docker-compose.yml -f docker-compose.postgres.yml logs -f goclaw'
alias goclaw-ps='cd ~/workspace/goclaw && docker compose -f docker-compose.yml -f docker-compose.postgres.yml ps'
ZSHEOF

source ~/.zshrc
```

---

## Фінальний чекліст

- [ ] Docker Desktop запущений (`docker ps` без помилок)
- [ ] `git log --oneline -1` показує `1f291471` або новіший
- [ ] `docker-compose.override.yml` має `/Users/arsen111999/.claude` (не `/home/vokov`)
- [ ] `.env` має `GOCLAW_GATEWAY_TOKEN`, `GOCLAW_ENCRYPTION_KEY`, `GOCLAW_ANTHROPIC_BASE_URL`, `POSTGRES_PASSWORD=goclaw`
- [ ] Всі контейнери running: `docker ps --format "table {{.Names}}\t{{.Status}}"`
- [ ] `curl http://localhost:18790/health` → 200
- [ ] `curl -I http://localhost:3000` → 200
- [ ] Dashboard відкривається в браузері: `http://localhost:3000`
- [ ] launchd job завантажений: `launchctl list | grep goclaw`
