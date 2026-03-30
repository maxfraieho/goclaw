# GoClaw macOS Deployment Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Розгорнути повну копію GoClaw AI-стека на Mac (100.84.163.96) — точна копія Linux-інстанції, підключена до Claude proxy `http://100.65.225.122:8084`.

**Architecture:** Docker Compose на macOS (postgres + goclaw + goclaw-ui). Секрети у `~/workspace/goclaw/.env`. Autostart через launchd при логіні. Claude-запити через `GOCLAW_ANTHROPIC_BASE_URL`.

**Tech Stack:** Docker Desktop for Mac, docker-compose, ghcr.io/nextlevelbuilder/goclaw, pgvector/pgvector:pg18, nginx SPA, launchd

---

## Змінні

```
SSH_HOST=100.84.163.96
SSH_USER=arsen111999
TARGET_DIR=~/workspace/goclaw
REPO=git@github.com:maxfraieho/goclaw.git
CLAUDE_PROXY_URL=http://100.65.225.122:8084
GOCLAW_PORT=18790
GOCLAW_UI_PORT=3000
```

## Ключові файли репо

| Файл | Призначення |
|------|-------------|
| `docker-compose.yml` | Base: goclaw сервіс, образ ghcr.io |
| `docker-compose.postgres.yml` | Postgres pgvector overlay; встановлює `GOCLAW_POSTGRES_DSN` автоматично |
| `docker-compose.selfservice.yml` | Web UI (nginx + React SPA) на порту 3000 |
| `docker-compose.override.yml` | Монтує `~/.claude` + `ANTHROPIC_API_KEY` для Claude CLI |
| `prepare-env.sh` | Генерує `GOCLAW_GATEWAY_TOKEN` та `GOCLAW_ENCRYPTION_KEY` |
| `.env.example` | Шаблон для `.env` |

## Ключові env vars

```
GOCLAW_GATEWAY_TOKEN       → авто-генерується, зберігати між переінсталяціями
GOCLAW_ENCRYPTION_KEY      → AES-256 ключ для API keys у БД, ЗБЕРІГАТИ
GOCLAW_POSTGRES_DSN        → встановлюється автоматично через docker-compose.postgres.yml
GOCLAW_ANTHROPIC_BASE_URL  → Claude proxy URL (http://100.65.225.122:8084)
GOCLAW_ANTHROPIC_API_KEY   → API key (може бути dummy якщо proxy не вимагає)
ANTHROPIC_API_KEY          → для Claude CLI у docker-compose.override.yml
```

---

## Task 1: SSH підключення та перевірка Mac

**Files:** `~/.ssh/config` (optional, на твоєму ноутбуці)

### Step 1: Підключитися до Mac

```bash
ssh arsen111999@100.84.163.96
# password: 0523
```

Зручний alias (на твоєму ноутбуці в `~/.ssh/config`):
```
Host goclaw-mac
    HostName 100.84.163.96
    User arsen111999
```
Після цього: `ssh goclaw-mac`

### Step 2: Перевірити середовище

```bash
whoami        # → arsen111999
hostname
uname -m      # → arm64 або x86_64
sw_vers       # → macOS version
```

### Step 3: Перевірити Docker

```bash
docker --version
docker compose version
docker ps
```

**Якщо Docker не встановлено:**
```bash
brew install --cask docker
open /Applications/Docker.app
# Дочекатись запуску (docker ps без помилок)
```

---

## Task 2: Клонування репозиторію

### Step 1: Перевірити SSH ключ для GitHub

```bash
ssh -T git@github.com
# → "Hi maxfraieho! You've successfully authenticated..."
```

**Якщо ключа немає:**
```bash
# Додай ключ Mac в GitHub Settings → SSH Keys
ssh-keygen -t ed25519 -C "mac" && cat ~/.ssh/id_ed25519.pub
# Або використовуй HTTPS замість SSH:
# git clone https://github.com/maxfraieho/goclaw.git ~/workspace/goclaw
```

### Step 2: Клонувати або оновити репо

```bash
# Якщо директорії ще немає:
mkdir -p ~/workspace
git clone git@github.com:maxfraieho/goclaw.git ~/workspace/goclaw
cd ~/workspace/goclaw && git checkout main

# Якщо вже існує — safe update:
cd ~/workspace/goclaw
git fetch origin
git status
git pull --ff-only origin main
```

Очікуваний результат: `Already up to date.` або `Fast-forward`.

### Step 3: Перевірити структуру

```bash
ls ~/workspace/goclaw
# Має бути: docker-compose.yml, prepare-env.sh, .env.example, ...
git log --oneline -5
```

---

## Task 3: Підготовка .env

### Step 1: Запустити prepare-env.sh

```bash
cd ~/workspace/goclaw
chmod +x prepare-env.sh
./prepare-env.sh
```

Результат: `.env` з `GOCLAW_GATEWAY_TOKEN` та `GOCLAW_ENCRYPTION_KEY`.

> **ВАЖЛИВО:** Якщо хочеш щоб API keys з БД Linux-інстанції розшифровувались — скопіюй **ті самі** `GOCLAW_GATEWAY_TOKEN` та `GOCLAW_ENCRYPTION_KEY` з `.env` свого ноутбука. Різні ключі = нові зашифровані дані, треба буде переналаштовувати providers у dashboard.

### Step 2: Додати Claude proxy URL та виправити PinchTab

```bash
cat >> ~/workspace/goclaw/.env << 'EOF'

# Claude proxy
GOCLAW_ANTHROPIC_BASE_URL=http://100.65.225.122:8084
GOCLAW_ANTHROPIC_API_KEY=REPLACE_WITH_YOUR_KEY

# Postgres (defaults for docker-compose.postgres.yml)
# ВАЖЛИВО: явно задати щоб уникнути конфлікту зі змінними інших проектів у shell
POSTGRES_PASSWORD=goclaw
POSTGRES_USER=goclaw
POSTGRES_DB=goclaw
EOF
```

**Якщо proxy не вимагає ключа:**
```
GOCLAW_ANTHROPIC_API_KEY=sk-ant-placeholder
```

### Step 2b: Виправити PinchTab URL для macOS

У `.env` з репо є `GOCLAW_BROWSER_PINCHTAB_URL=http://172.17.0.1:9867` — це Linux docker0 bridge, на Mac не існує.

**Якщо PinchTab встановлений на Mac** (на хості): замінити на `host.docker.internal` (Docker Desktop автоматично резолвить його):
```bash
sed -i '' 's|GOCLAW_BROWSER_PINCHTAB_URL=.*|GOCLAW_BROWSER_PINCHTAB_URL=http://host.docker.internal:9867|' \
  ~/workspace/goclaw/.env
```
Також у `~/.pinchtab/config.json` має бути `"bind": "0.0.0.0"` (не `127.0.0.1`).

**Якщо PinchTab на Mac не встановлений**: прибрати рядок з `.env`:
```bash
sed -i '' '/GOCLAW_BROWSER_PINCHTAB_URL/d' ~/workspace/goclaw/.env
sed -i '' '/GOCLAW_BROWSER_PINCHTAB_TOKEN/d' ~/workspace/goclaw/.env
```
Тоді goclaw використає go-rod (Chrome CDP) як fallback якщо `GOCLAW_BROWSER_ENABLED=true`, або браузер-тул буде вимкнений.

### Step 3: Перевірити .env

```bash
grep -E 'GOCLAW_GATEWAY_TOKEN|GOCLAW_ENCRYPTION_KEY|GOCLAW_ANTHROPIC|POSTGRES' ~/workspace/goclaw/.env
# Всі 4+ змінні мають бути заповнені
chmod 600 ~/workspace/goclaw/.env
```

---

## Task 4: Адаптація docker-compose.override.yml під macOS

Файл монтує `/home/vokov/.claude` — це Linux шлях. На Mac треба змінити.

### Step 1: Переглянути файл

```bash
cat ~/workspace/goclaw/docker-compose.override.yml
```

### Step 2: Виправити шлях ~/.claude

```bash
sed -i '' 's|/home/vokov/.claude|/Users/arsen111999/.claude|g' \
  ~/workspace/goclaw/docker-compose.override.yml
```

Перевірити:
```bash
grep claude ~/workspace/goclaw/docker-compose.override.yml
# → /Users/arsen111999/.claude:/root/.claude:ro
```

### Step 3: Переконатись що ~/.claude існує

```bash
mkdir -p ~/.claude
# Якщо є ~/.claude/credentials.json з ноутбука — скопіюй його сюди
```

---

## Task 5: Запуск стека

### Step 1: ~~Створити shared network~~ (не потрібно)

> docker-compose автоматично створює мережу `goclaw-net`. Крок `docker network create shared` — застарілий, пропустити.

### Step 2: Запустити повний стек

```bash
cd ~/workspace/goclaw
# ВАЖЛИВО: явно передати POSTGRES_PASSWORD щоб уникнути конфлікту зі змінними у shell
POSTGRES_PASSWORD=goclaw docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  -f docker-compose.override.yml \
  up -d
```

Перший запуск завантажує образи (~500MB). Дочекатись завершення.

### Step 3: Перевірити статус

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  ps
# Очікувано: postgres (healthy), goclaw (running), goclaw-ui (running)
```

### Step 4: Health check

```bash
curl -s http://localhost:18790/health
# → {"status":"ok"} або HTTP 200
curl -sI http://localhost:3000 | head -3
# → HTTP/1.1 200 OK
```

### Step 5: Перевірити логи

```bash
docker compose -f docker-compose.yml -f docker-compose.postgres.yml logs goclaw --tail=30
# Немає FATAL помилок; є рядки про "listening" або "migrated"
```

---

## Task 6: launchd автозапуск

### Step 1: Визначити шлях до docker

```bash
which docker
# → /usr/local/bin/docker  або  /Applications/Docker.app/Contents/Resources/bin/docker
# → на Apple Silicon brew: /opt/homebrew/bin/docker
```

Запам'ятати цей шлях — підставити у plist нижче.

### Step 2: Створити plist

```bash
mkdir -p ~/Library/LaunchAgents
cat > ~/Library/LaunchAgents/com.goclaw.gateway.plist << 'EOF'
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
        <string>cd /Users/arsen111999/workspace/goclaw &amp;&amp; POSTGRES_PASSWORD=goclaw docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml -f docker-compose.override.yml up -d</string>
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
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin</string>
        <key>HOME</key>
        <string>/Users/arsen111999</string>
    </dict>
</dict>
</plist>
EOF
```

> `KeepAlive=false` — launchd лише запускає `docker compose up -d` при логіні. Docker сам підтримує контейнери через `restart: unless-stopped`.

### Step 3: Перевірити XML

```bash
plutil -lint ~/Library/LaunchAgents/com.goclaw.gateway.plist
# → OK
```

### Step 4: Завантажити job

```bash
launchctl bootstrap gui/$(id -u) \
  ~/Library/LaunchAgents/com.goclaw.gateway.plist
```

### Step 5: Запустити вручну (тест)

```bash
launchctl kickstart gui/$(id -u)/com.goclaw.gateway
sleep 5
docker ps --format "table {{.Names}}\t{{.Status}}"
```

### Step 6: Перевірити статус

```bash
launchctl print gui/$(id -u)/com.goclaw.gateway
# → state = waiting (нормально після завершення up -d)

tail -30 ~/Library/Logs/goclaw-launch.log
tail -10 ~/Library/Logs/goclaw-launch-err.log
```

---

## Task 7: Shell environment (~/.zshrc)

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

## Task 8: Повна верифікація

```bash
# 1. Ідентифікація
whoami && hostname && pwd

# 2. Git стан
git -C ~/workspace/goclaw status
git -C ~/workspace/goclaw log --oneline -3

# 3. Env змінні
env | grep -E 'ANTHROPIC|CLAUDE|GOCLAW'

# 4. Docker контейнери
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# 5. GoClaw health
curl -s http://localhost:18790/health

# 6. Web UI
curl -sI http://localhost:3000 | head -3

# 7. Claude proxy доступний
curl -sv --max-time 5 http://100.65.225.122:8084 2>&1 | head -20

# 8. launchd job
launchctl list | grep goclaw

# 9. Логи (немає FATAL)
docker logs $(docker ps -qf name=goclaw) --tail 20 2>/dev/null
```

---

## Типові проблеми

| Проблема | Причина | Рішення |
|---------|---------|---------|
| `docker: command not found` в launchd | PATH в plist неповний | Додати `/opt/homebrew/bin` або реальний шлях в `EnvironmentVariables > PATH` |
| `Cannot connect to Docker daemon` | Docker Desktop не запущений | Preferences → General → Start at Login ✓ |
| `port 5432 already in use` | Локальний Postgres | `brew services stop postgresql` або змінити `POSTGRES_PORT` в `.env` |
| `port 18790 already in use` | Інша інстанція | Змінити `GOCLAW_PORT` в `.env` |
| Provider key decrypt error | Різний `GOCLAW_ENCRYPTION_KEY` | Скопіювати точний ключ з Linux `.env` |
| `git@github.com: Permission denied` | Немає SSH ключа | `ssh-keygen -t ed25519 && cat ~/.ssh/id_ed25519.pub` → GitHub Settings |
| launchd: "Load failed" | Помилка XML | `plutil -lint` → виправити plist |
| goclaw контейнер виходить одразу | Нема GOCLAW_ENCRYPTION_KEY | `./prepare-env.sh` → перезапустити |

---

## Фінальний чекліст

- [ ] SSH до Mac працює (`ssh arsen111999@100.84.163.96`)
- [ ] Docker Desktop запущений (`docker ps`)
- [ ] repo склонований з `git@github.com:maxfraieho/goclaw.git`
- [ ] `main` актуальна (`git log --oneline -1`)
- [ ] `.env` містить `GOCLAW_GATEWAY_TOKEN`, `GOCLAW_ENCRYPTION_KEY`, `GOCLAW_ANTHROPIC_BASE_URL`
- [ ] `docker-compose.override.yml` має шлях `/Users/arsen111999/.claude`
- [ ] Всі 3 контейнери запущені (`docker ps`)
- [ ] GoClaw відповідає (`curl http://localhost:18790/health`)
- [ ] Web UI відповідає (`curl http://localhost:3000`)
- [ ] Claude proxy доступний (`curl http://100.65.225.122:8084`)
- [ ] launchd plist завантажений (`launchctl list | grep goclaw`)
- [ ] Логи пишуться (`~/Library/Logs/goclaw-launch.log`)
- [ ] Після reboot стек підіймається автоматично
