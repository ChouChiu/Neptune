# Neptune 部署指南

Docker 单容器部署。Hermes Agent 需单独部署或由用户自行管理。

---

## 前置条件

- 一台 Debian/Ubuntu VPS（推荐 1C1G 以上）
- 一个域名，已解析到该服务器的 IP
- VPS 已安装 Docker + Docker Compose（`setup.sh` 会自动安装）
- VPS 上已运行 Hermes Agent（AI 聊天后端），安装方法见 [附录：Hermes Agent 安装](#附录hermes-agent-安装)

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
| `HERMES_API_URL` | Hermes Agent API 地址（Docker 内用 `http://host.docker.internal:8642/v1`） |
| `HERMES_API_KEY` | 与 Hermes `API_SERVER_KEY` 一致 |
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

**容器内无法访问宿主机的 Hermes？**

确认 `.env` 中 `HERMES_API_URL` 使用 `http://host.docker.internal:8642/v1`。部分 Linux 内核需在 `docker-compose.yml` 中添加：

```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
```

**如何重新注册 webhook？**

```bash
./deploy/register-webhook.sh 你的域名 你的密钥
```

---

## 附录：Hermes Agent 安装

Neptune 的 AI 聊天功能依赖 Hermes Agent 作为后端。在 VPS 上执行以下步骤。

### 安装并配置模型

```bash
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
source ~/.bashrc
hermes --version
```

安装脚本会交互式引导你配置模型和 API Key。安装后检查 `~/.hermes/config.yaml`，确保已填入你的 provider、API 地址和密钥。

### 配置 API Server

```bash
HERMES_KEY=$(openssl rand -hex 32)
cat > ~/.hermes/.env << EOF
API_SERVER_ENABLED=true
API_SERVER_HOST=127.0.0.1
API_SERVER_PORT=8642
API_SERVER_KEY=$HERMES_KEY
EOF
echo "保存此密钥用于 Neptune 的 HERMES_API_KEY: $HERMES_KEY"
```

### 部署角色卡

Hermes 角色卡（SOUL.md）请参考 Neptune 仓库中的示例或自行配置。

### 注册为 systemd 服务

Hermes 继续使用 systemd 管理（不在 Docker 中）：

```bash
# 创建 service 文件
cat > /etc/systemd/system/hermes.service << 'EOF'
[Unit]
Description=Hermes Agent Gateway
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/hermes gateway
Restart=always
RestartSec=5
WorkingDirectory=/root
EnvironmentFile=/root/.hermes/.env

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable hermes
systemctl start hermes
systemctl status hermes
```

### 验证运行

```bash
curl -s http://127.0.0.1:8642/health
# 应返回: {"status": "ok"}

curl -s http://127.0.0.1:8642/v1/chat/completions \
  -H "Authorization: Bearer <HERMES_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"model": "hermes-agent", "messages": [{"role": "user", "content": "你好"}]}'
```
