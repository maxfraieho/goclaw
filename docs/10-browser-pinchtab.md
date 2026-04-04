# Browser Automation — PinchTab Setup

## Overview

GoClaw підтримує два бекенди для браузерної автоматизації:

| Бекенд | Env var | Токени/сторінку | Опис |
|--------|---------|-----------------|------|
| **PinchTab** (рекомендовано) | `GOCLAW_BROWSER_PINCHTAB_URL` | ~800 | HTTP API до окремого локального демона |
| **go-rod/CDP** | `GOCLAW_BROWSER_REMOTE_URL` | ~8000 | Прямий CDP до Chrome |

## Архітектура

```
agent (claude-cli)
  └─► browser tool (goclaw bridge MCP)
        └─► pkg/browser/PinchTabManager
              └─► HTTP + Bearer token → PinchTab daemon (:9867)
                    └─► headless або visible Chrome
```

**Ключові файли:**
- `pkg/browser/pinchtab.go` — HTTP клієнт PinchTab API (Start/Stop/Snapshot/Screenshot/…)
- `pkg/browser/backend.go` — інтерфейс `Backend` (Manager + PinchTabManager)
- `pkg/browser/tool.go` — `BrowserTool` приймає будь-який `Backend`
- `internal/config/config_channels.go` — `BrowserToolConfig`: `PinchTabURL`, `PinchTabToken`
- `internal/config/config_load.go` — env var mapping
- `cmd/gateway_setup.go` — вибір бекенду при старті

---

## Швидке розгортання (нова машина)

### 1. Встановити PinchTab

```bash
npm install -g --prefix ~/.local pinchtab   # встановлює ~/.local/bin/pinchtab
```

### 2. Ініціалізувати і запустити демон

```bash
~/.local/bin/pinchtab daemon install   # systemd-user helper, якщо він доступний
systemctl --user daemon-reload
systemctl --user enable --now pinchtab
```

Для Alpine/OpenRC, де `systemd --user` зазвичай немає, запускайте PinchTab як окремий хостовий сервіс будь-яким наявним supervisor-ом. Мінімальний ручний запуск:

```bash
mkdir -p ~/.local/bin ~/.pinchtab
nohup ~/.local/bin/pinchtab daemon start >/tmp/pinchtab.log 2>&1 &
```

### 3. Отримати токен

```bash
~/.local/bin/pinchtab config            # показує Token: xxxx...
# або
cat ~/.pinchtab/config.json | python3 -c "import sys,json; print(json.load(sys.stdin)['server']['token'])"
```

### 4. Додати до goclaw .env

```bash
# Для docker compose на Linux/Alpine хості:
# ~/projects/goclaw/.env
GOCLAW_BROWSER_PINCHTAB_URL=http://host.docker.internal:9867
GOCLAW_BROWSER_PINCHTAB_TOKEN=<token з ~/.pinchtab/config.json>
```

Якщо `goclaw` запускається не в контейнері, а напряму на хості, тоді URL може бути `http://localhost:9867`.

### 5. Застосувати конфіг PinchTab (знімає обмеження IDPI, дозволяє всі дії)

```bash
cat > ~/.pinchtab/config.json << 'CONF'
{
  "configVersion": "0.8.0",
  "server": {
    "port": "9867",
    "bind": "0.0.0.0",
    "token": "<TOKEN>",
    "stateDir": "/home/<USER>/.pinchtab",
    "engine": ""
  },
  "browser": {
    "version": "144.0.7559.133",
    "binary": "/home/<USER>/.pinchtab/chrome-visible.sh",
    "extraFlags": "",
    "extensionPaths": []
  },
  "instanceDefaults": {
    "mode": "head",
    "noRestore": null,
    "timezone": "",
    "blockImages": null,
    "blockMedia": null,
    "blockAds": null,
    "maxTabs": 20,
    "maxParallelTabs": null,
    "userAgent": "",
    "noAnimations": null,
    "stealthLevel": "light",
    "tabEvictionPolicy": "close_lru"
  },
  "security": {
    "allowEvaluate": true,
    "allowMacro": true,
    "allowScreencast": true,
    "allowDownload": true,
    "downloadAllowedDomains": [],
    "downloadMaxBytes": 20971520,
    "allowUpload": true,
    "allowClipboard": true,
    "uploadMaxRequestBytes": 10485760,
    "uploadMaxFiles": 8,
    "uploadMaxFileBytes": 5242880,
    "uploadMaxTotalBytes": 10485760,
    "maxRedirects": -1,
    "attach": {
      "enabled": false,
      "allowHosts": ["127.0.0.1", "localhost", "::1"],
      "allowSchemes": ["ws", "wss"]
    },
    "idpi": {
      "enabled": false,
      "allowedDomains": ["127.0.0.1", "localhost", "::1", "*"],
      "strictMode": false,
      "scanContent": false,
      "wrapContent": false,
      "customPatterns": [],
      "scanTimeoutSec": 5
    }
  },
  "profiles": {
    "baseDir": "/home/<USER>/.pinchtab/profiles",
    "defaultProfile": "default"
  },
  "multiInstance": {
    "strategy": "always-on",
    "allocationPolicy": "fcfs",
    "instancePortStart": 9868,
    "instancePortEnd": 9968,
    "restart": {"maxRestarts": 20, "initBackoffSec": 2, "maxBackoffSec": 60, "stableAfterSec": 300}
  },
  "timeouts": {"actionSec": 30, "navigateSec": 60, "shutdownSec": 10, "waitNavMs": 1000},
  "scheduler": {"enabled": null, "strategy": "", "maxQueueSize": null, "maxPerAgent": null, "maxInflight": null, "maxPerAgentInflight": null, "resultTTLSec": null, "workerCount": null},
  "observability": {"activity": {"enabled": true, "sessionIdleSec": 1800, "retentionDays": 1}}
}
CONF
```

> Замінити `<TOKEN>` і `<USER>` на реальні значення.

### 6. Wrapper-скрипт для видимого Chrome (X11)

```bash
cat > ~/.pinchtab/chrome-visible.sh << 'SCRIPT'
#!/bin/bash
# Strips headless flags so Chrome opens a real window on X11 desktop.
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --headless*) ;;
    --ozone-platform=headless) ;;
    --ozone-override-screen-size=*) ;;
    *) ARGS+=("$arg") ;;
  esac
done
export DISPLAY=:0
export XAUTHORITY=/home/<USER>/.Xauthority
exec /opt/google/chrome/chrome --ozone-platform=x11 "${ARGS[@]}"
SCRIPT
chmod +x ~/.pinchtab/chrome-visible.sh
```

> Замінити `<USER>` та шлях до chrome якщо відрізняється (`which google-chrome`).

### 7. Додати DISPLAY до pinchtab.service

```bash
# Відредагувати ~/.config/systemd/user/pinchtab.service
# Додати під [Service]:
#   Environment="DISPLAY=:0"
#   Environment="XAUTHORITY=/home/<USER>/.Xauthority"

systemctl --user daemon-reload
systemctl --user restart pinchtab
```

Для Alpine/OpenRC сенс той самий: процес PinchTab повинен бачити `DISPLAY` і `XAUTHORITY`, якщо потрібен видимий Chrome.

### 8. Перезапустити goclaw

```bash
cd ~/projects/goclaw
/usr/local/go/bin/go build -o goclaw .
systemctl --user stop goclaw
sudo cp goclaw /usr/local/bin/goclaw
systemctl --user start goclaw
systemctl --user start goclaw-ui    # ← обов'язково після goclaw, бо Requires=
```

---

## Сервіси

| Сервіс | Порт | Команда |
|--------|------|---------|
| `pinchtab.service` | 9867 | PinchTab daemon (Chrome) |
| `goclaw.service` | 18790 | GoClaw gateway |
| `goclaw-ui.service` | 3000 | Web UI (nginx Docker) |

**Важливо:** `goclaw-ui` залежить від `goclaw` (`Requires=goclaw.service`).
При зупинці/рестарті `goclaw` → `goclaw-ui` теж зупиняється. Завжди запускати обидва.

## Docker Compose на Alpine/Linux хості

Рекомендований стек для цього форку:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  -f docker-compose.alpine-pinchtab.yml \
  up -d
```

Що робить `docker-compose.alpine-pinchtab.yml`:
- підключає `goclaw` до хостового PinchTab через `http://host.docker.internal:9867`
- не публікує `postgres:5432` на хості, щоб не чіпати інші сервіси
- не піднімає `chrome` sidecar, бо для PinchTab він не потрібен

Fallback на прямий CDP потрібен лише якщо PinchTab недоступний:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.browser.yml \
  -f docker-compose.browser.alpine.yml \
  up -d
```

У цьому режимі healthcheck sidecar залишається через `http://127.0.0.1:9222/json/version`, але порт `9222` не публікується на хості.

---

## Перевірка стану

```bash
# Всі сервіси
systemctl --user status pinchtab goclaw goclaw-ui

# API PinchTab
TOKEN=$(cat ~/.pinchtab/config.json | python3 -c "import sys,json; print(json.load(sys.stdin)['server']['token'])")
curl -H "Authorization: Bearer $TOKEN" http://localhost:9867/profiles

# Реальне з'єднання container -> PinchTab
docker compose exec goclaw sh -lc \
  'wget -qO- --header="Authorization: Bearer '"$TOKEN"'" http://host.docker.internal:9867/profiles >/dev/null && echo OK'

# UI
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/

# Лог при успішному старті goclaw
journalctl --user -u goclaw -n 30 | grep "browser tool enabled"
# Очікується: level=INFO msg="browser tool enabled (PinchTab)" url=http://host.docker.internal:9867
```

---

## Очищення застряглих сесій

PinchTab тримає інстанси навіть після зависання. Якщо агент повідомляє про `409` або `profile already active`:

```bash
TOKEN=$(cat ~/.pinchtab/config.json | python3 -c "import sys,json; print(json.load(sys.stdin)['server']['token'])")

# Видалити профіль goclaw
PROF=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:9867/profiles | \
  python3 -c "import sys,json; p=[x for x in json.load(sys.stdin) if x['name']=='goclaw']; print(p[0]['id'] if p else '')")
[ -n "$PROF" ] && curl -s -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:9867/profiles/$PROF

# Або повний перезапуск (єдиний спосіб очистити активні інстанси)
systemctl --user restart pinchtab
```

---

## Баги що були виправлені (commits 40416fe – fcc558e)

| Симптом | Причина | Виправлення |
|---------|---------|-------------|
| `HTTP 401 /profiles` | `GOCLAW_BROWSER_PINCHTAB_TOKEN` не передавався до `NewPinchTabManager` | `40416fe` — додано поле `PinchTabToken` у config + `Authorization` header у `do()` |
| `409 Conflict` при повторному запуску | `Stop()` видаляв інстанс але не профіль | `2ccfa07` — `Stop()` тепер видаляє профіль; `Start()` перевикористовує профіль якщо вже існує |
| `screenshot: HTTP 401` | `doGetRaw()` (бінарна відповідь) не додавав `Authorization` header | `813b92b` — токен додано і в `doGetRaw()` |
| `snapshot: 0 refs` | PinchTab v0.8.x повертає `nodes[]` замість рядка `snapshot` | `fcc558e` — додано `ptSnapshotNode`, `buildSnapshotText()` для конвертації nodes → текст |

---

## Нотатки щодо PinchTab v0.8.x

- API snapshot повертає `{"count": N, "nodes": [...], "title": "...", "url": "..."}` (не `{"snapshot": "..."}`)
- `DELETE /instances/<id>` повертає `405` якщо інстанс активний → тільки restart сервісу очищає
- `mode: "head"` в config ігнорується PinchTab-ом — Chrome завжди стартує з `--headless=new`
- Обхід для видимого Chrome: wrapper-скрипт що видаляє `--headless` та `--ozone-platform=headless` флаги
