# Neptune 部署指南

本指南帮助你在 Debian/Ubuntu VPS 上从零部署 Neptune Telegram 群管理机器人。完成后，机器人将通过 webhook 接收消息，支持关键词匹配、AI 聊天、验证码等功能。

---

## 前置条件

- 一台 Debian/Ubuntu VPS（推荐 1C1G 以上）
- 一个域名，已解析到该服务器的 IP
- 本地已安装 Go 1.26+（用于编译）
- VPS 上已安装 Hermes Agent（AI 聊天后端），安装方法见 [附录：Hermes Agent 安装](#附录hermes-agent-安装)

## 第一步：准备配置文件

在本地项目目录：

```bash
cp .env.example .env
```

编辑 `.env`，填入以下必填项：

| 变量 | 获取方式 |
|------|----------|
| `BOT_TOKEN` | 在 Telegram 找 [@BotFather](https://t.me/BotFather)，发送 `/newbot` 创建机器人 |
| `BOT_USERNAME` | BotFather 创建时分配的用户名（不含 `@`） |
| `HERMES_API_URL` | Hermes Agent API 地址（默认 `http://127.0.0.1:8642/v1`） |
| `HERMES_API_KEY` | 与 Hermes `API_SERVER_KEY` 一致（AI 聊天功能） |
| `GITHUB_WEBHOOK_SECRET` | 自行生成：`openssl rand -hex 32` |

## 第二步：初始化服务器

```bash
make setup DEPLOY_HOST=root@你的服务器 IP DOMAIN=你的域名
```

脚本自动完成以下操作：

- 创建 `neptune` 系统用户
- 创建 `/opt/neptune/` 目录结构
- 安装并配置 Nginx（反向代理 + 限流）
- 安装 systemd 服务（开机自启）
- 上传编译好的二进制和迁移文件
- 创建 `.env` 模板

运行后确认输出中没有报错。

## 第三步：配置服务器上的 secrets

SSH 到服务器，编辑配置文件：

```bash
sudo nano /opt/neptune/.env
```

填入 `BOT_TOKEN`、`BOT_USERNAME`、`HERMES_API_KEY`、`GITHUB_WEBHOOK_SECRET` 等。

## 第四步：启动服务

```bash
sudo systemctl start neptune
sudo systemctl status neptune    # 确认状态为 active (running)
sudo journalctl -u neptune -f    # 查看实时日志
```

## 第五步：配置 HTTPS

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot certonly --nginx -d 你的域名
```

Certbot 自动添加续期定时任务。Nginx 配置已包含 HTTP→HTTPS 跳转。

## 第六步：注册 Telegram Webhook

```bash
make webhook DOMAIN=你的域名 WEBHOOK_SECRET=你的密钥
```

此命令向 Telegram 注册 webhook 地址，并同步所有命令到 BotFather。

## 第七步：验证部署

```bash
make e2e BASE_URL=https://你的域名
```

然后在 Telegram 给机器人发送 `/ping`，应收到 "Pong!" 回复。

---

## 日常更新

### 方式一：自动部署（推荐）

推送到 `main` 分支即自动部署。需在 GitHub 仓库 Settings → Secrets and variables → Actions 配置：

| Secret | 说明 |
|--------|------|
| `DEPLOY_HOST` | 服务器 IP（如 `66.154.101.126`） |
| `DEPLOY_USER` | SSH 用户名（如 `root`） |
| `DEPLOY_PASSWORD` | SSH 密码 |

### 方式二：手动部署

```bash
make deploy DEPLOY_HOST=user@你的服务器 IP
```

---

## 常用运维命令

```bash
# 查看服务状态
sudo systemctl status neptune

# 查看实时日志
sudo journalctl -u neptune -f

# 重启服务
sudo systemctl restart neptune

# 检查 Nginx 配置语法
sudo nginx -t

# 重新加载 Nginx
sudo systemctl reload nginx

# 续期 SSL 证书（通常自动续期）
sudo certbot renew
```

---

## 目录结构（服务器）

```
/opt/neptune/
├── neptune              # 二进制
├── .env                 # 配置文件（权限 600）
├── data/
│   ├── neptune.db       # SQLite 数据库
│   └── captcha/         # 验证码图片
├── migrations/          # SQL 迁移文件
├── deploy/              # 部署脚本
└── static/              # 静态文件
```

---

## 常见问题

**发送消息后机器人没反应？**

检查日志 `sudo journalctl -u neptune -f`，确认 webhook 已注册且服务正常运行。

**Nginx 返回 502？**

确认 neptune 服务正在运行：`sudo systemctl status neptune`

**如何重新注册 webhook？**

```bash
make webhook DOMAIN=你的域名 WEBHOOK_SECRET=你的密钥
```

**如何查看数据库？**

```bash
sudo -u neptune sqlite3 /opt/neptune/data/neptune.db
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

```bash
cp deploy/hermes/SOUL.md ~/.hermes/SOUL.md
```

### 注册为 systemd 服务

```bash
sudo cp deploy/hermes/hermes.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable hermes
sudo systemctl start hermes
sudo systemctl status hermes
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
