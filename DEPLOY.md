# Neptune 部署指南

Docker 单容器部署。

---

## 前置条件

- 一台 Debian/Ubuntu VPS（推荐 1C1G 以上）
- 一个域名，已解析到该服务器的 IP
- VPS 已安装 Docker + Docker Compose（`setup.sh` 会自动安装）

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

**如何重新注册 webhook？**

```bash
./deploy/register-webhook.sh 你的域名 你的密钥
```
