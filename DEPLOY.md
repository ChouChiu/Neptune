# Neptune 部署指南

Docker 单容器部署。MaiBot 需单独部署（独立 docker-compose）。

---

## 前置条件

- 一台 Debian/Ubuntu VPS（推荐 1C1G 以上）
- 一个域名，已解析到该服务器的 IP
- VPS 已安装 Docker + Docker Compose（`setup.sh` 会自动安装）
- VPS 上已运行 MaiBot（AI 聊天后端），安装方法见 [附录：MaiBot 安装](#附录maibot-安装)

## 第一步：初始化服务器

```bash
ssh root@你的服务器
bash -s < deploy/setup.sh
```

或远程执行：

```bash
ssh root@你的服务器 "bash -s" < deploy/setup.sh
```

脚本自动完成：

- 安装 Docker（如未安装）
- 克隆仓库到 `/opt/neptune`
- 创建 `.env` 配置模板
- 创建 `data/` 目录

## 第二步：配置 secrets

```bash
nano /opt/neptune/.env
```

填入以下必填项：

| 变量 | 获取方式 |
|------|----------|
| `BOT_TOKEN` | Telegram [@BotFather](https://t.me/BotFather) → `/newbot` |
| `BOT_USERNAME` | BotFather 创建时分配的用户名（不含 `@`） |
| `MAIBOT_WS_URL` | MaiBot WebSocket 地址（默认 `ws://127.0.0.1:8090/ws`） |
| `MAIBOT_API_KEY` | 与 MaiBot `bot_config.toml` 中 `api_server_allowed_api_keys` 一致 |
| `GITHUB_WEBHOOK_SECRET` | `openssl rand -hex 32` |

## 第三步：启动服务

```bash
cd /opt/neptune
docker compose up --build -d
docker compose ps         # 确认状态为 running
docker compose logs -f    # 查看实时日志
```

## 第四步：配置外部反向代理

Docker 容器绑定在 `127.0.0.1:8080`，需要外部反向代理提供 HTTPS。

### Nginx 示例

```nginx
server {
    listen 443 ssl;
    server_name bot.example.com;

    ssl_certificate     /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }
}
```

### Caddy 示例（自动 HTTPS）

```
bot.example.com {
    reverse_proxy localhost:8080
}
```

## 第五步：注册 Telegram Webhook

```bash
cd /opt/neptune
./deploy/register-webhook.sh 你的域名 你的密钥
```

## 第六步：验证部署

```bash
./deploy/e2e-test.sh https://你的域名
```

在 Telegram 给机器人发送 `/ping`，应收到 "Pong!" 回复。

---

## 日常更新

推送到 `main` 分支即自动部署（GitHub Actions）。需在仓库 Settings → Secrets 配置：

| Secret | 说明 |
|--------|------|
| `DEPLOY_HOST` | 服务器 IP |
| `DEPLOY_USER` | SSH 用户名（如 `root`） |
| `VPS_SSH_KEY` | SSH 私钥（与服务器 `authorized_keys` 对应） |

手动更新：

```bash
ssh root@你的服务器
cd /opt/neptune
git pull origin main
docker compose up --build -d
docker image prune -f
```

---

## 常用运维命令

```bash
# 查看容器状态
docker compose ps

# 查看实时日志
docker compose logs -f

# 重启服务
docker compose restart

# 重新构建并启动
docker compose up --build -d

# 进入容器调试
docker compose exec neptune sh

# 查看数据库
docker compose exec neptune sqlite3 data/neptune.db

# 停止服务
docker compose down
```

---

## 目录结构（服务器）

```
/opt/neptune/
├── Dockerfile
├── docker-compose.yml
├── .env                    # 配置文件（权限 600，gitignore）
├── data/
│   ├── neptune.db          # SQLite 数据库
│   └── captcha/            # 验证码图片
├── migrations/             # SQL 迁移文件
└── deploy/
    ├── setup.sh            # 初始化脚本
    ├── register-webhook.sh # Webhook 注册
    └── e2e-test.sh         # 端到端测试
```

---

## 常见问题

**Docker 构建很慢？**

首次构建需下载 Go 依赖和编译，约 3-5 分钟。后续构建有缓存会快很多。

**容器内无法连接 MaiBot？**

确认 `.env` 中 `MAIBOT_WS_URL` 使用 `ws://host.docker.internal:8090/ws`。部分 Linux 内核需在 `docker-compose.yml` 中添加：

```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
```

**如何重新注册 webhook？**

```bash
./deploy/register-webhook.sh 你的域名 你的密钥
```

---

## 附录：MaiBot 安装

Neptune 的 AI 聊天功能依赖 MaiBot 作为后端。在 VPS 上执行以下步骤。

### 克隆并配置

```bash
cd /opt
git clone https://github.com/Mai-with-u/MaiBot.git
cd MaiBot
```

### 创建精简 docker-compose.yml

移除 NapCat、sqlite-web，只保留 core：

```yaml
services:
  core:
    build: .
    restart: unless-stopped
    ports:
      - "127.0.0.1:8001:8001"   # WebUI
      - "127.0.0.1:8090:8090"   # WebSocket API Server
    volumes:
      - ./docker-config/mmc:/app/config
      - ./data/MaiMBot:/app/data
    environment:
      - TZ=Asia/Shanghai
```

### 首次启动生成配置

```bash
docker compose up -d
# 等待配置文件生成后停止
docker compose down
```

### 配置 bot_config.toml

编辑 `./docker-config/mmc/bot_config.toml`：

```toml
[bot]
platform = "neptune"
platforms = ["neptune:neptune"]
nickname = "你的机器人昵称"

[personality]
personality = "你的人格设定（200字以内）"
reply_style = "你的回复风格"

[maim_message]
enable_api_server = true
api_server_host = "0.0.0.0"
api_server_port = 8090
api_server_allowed_api_keys = ["你的密钥"]

[webui]
enabled = true
host = "0.0.0.0"
port = 8001
```

### 配置 model_config.toml

编辑 `./docker-config/mmc/model_config.toml`，填入 LLM API 密钥（OpenAI/DeepSeek 等）。

### 启动 MaiBot

```bash
docker compose up -d
```

### 配置 Nginx 反代

```nginx
# MaiBot WebUI
server {
    listen 443 ssl;
    server_name mai.example.com;
    ssl_certificate ...;
    ssl_certificate_key ...;
    location / {
        proxy_pass http://127.0.0.1:8001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# MaiBot WebSocket API Server
server {
    listen 443 ssl;
    server_name mai-ws.example.com;
    ssl_certificate ...;
    ssl_certificate_key ...;
    location /ws {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
}
```

### 验证运行

```bash
# 检查容器状态
docker compose ps

# 查看日志
docker compose logs -f core

# 访问 WebUI
# 浏览器打开 https://mai.example.com
```

### 配置 LLM 模型

通过 WebUI（`https://mai.example.com`）配置：
1. 进入配置向导
2. 填入 LLM API 密钥
3. 选择模型（planner 和 replyer 可用同一个）
4. 配置人格和回复风格
5. 在聊天页面测试对话
