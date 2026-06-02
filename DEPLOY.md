# Neptune 部署指南

## 前置条件

- 一台 Debian/Ubuntu VPS（推荐 1C1G 以上）
- 一个域名，已解析到该服务器的 IP
- 本地已安装 Go 1.26+（用于编译）

## 首次部署（从零开始）

### 第一步：准备配置文件

在本地项目目录：

```bash
cp .env.example .env
```

编辑 `.env`，填入以下必填项：

| 变量 | 获取方式 |
|------|----------|
| `BOT_TOKEN` | 在 Telegram 找 [@BotFather](https://t.me/BotFather)，发送 `/newbot` 创建机器人 |
| `BOT_USERNAME` | BotFather 创建时分配的用户名（不含 `@`） |
| `MIMO_API_KEY` | 小米 MiMo API 密钥（AI 聊天功能） |
| `GITHUB_WEBHOOK_SECRET` | 自行生成：`openssl rand -hex 32` |

### 第二步：初始化服务器

一条命令搞定（创建用户、目录、Nginx、systemd 服务、上传二进制）：

```bash
make setup DEPLOY_HOST=root@你的服务器 IP DOMAIN=你的域名
```

脚本会自动：
- 创建 `neptune` 系统用户
- 创建 `/opt/neptune/` 目录结构
- 安装并配置 Nginx（反向代理 + 限流）
- 安装 systemd 服务（开机自启）
- 上传编译好的二进制和迁移文件
- 创建 `.env` 模板

### 第三步：配置服务器上的 secrets

SSH 到服务器，编辑配置文件：

```bash
sudo nano /opt/neptune/.env
```

填入 `BOT_TOKEN`、`BOT_USERNAME`、`MIMO_API_KEY`、`GITHUB_WEBHOOK_SECRET` 等。

### 第四步：启动服务

```bash
sudo systemctl start neptune
sudo systemctl status neptune    # 确认运行正常
sudo journalctl -u neptune -f    # 查看实时日志
```

### 第五步：配置 HTTPS

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot certonly --nginx -d 你的域名
```

证书会自动续期。Nginx 配置已包含 HTTP→HTTPS 跳转。

### 第六步：注册 Telegram Webhook

```bash
make webhook DOMAIN=你的域名 WEBHOOK_SECRET=你的密钥
```

这会：
- 向 Telegram 注册 webhook 地址
- 同步所有 21 个命令到 BotFather

### 第七步：验证部署

```bash
make e2e BASE_URL=https://你的域名
```

然后在 Telegram 给机器人发送 `/ping`，应该收到 "Pong!" 回复。

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

# 查看 Nginx 配置
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

**Q: 发送消息后机器人没反应？**
检查日志 `sudo journalctl -u neptune -f`，确认 webhook 已注册且服务正常运行。

**Q: Nginx 返回 502？**
确认 neptune 服务正在运行：`sudo systemctl status neptune`

**Q: 如何重新注册 webhook？**
```bash
make webhook DOMAIN=你的域名 WEBHOOK_SECRET=你的密钥
```

**Q: 如何查看数据库？**
```bash
sudo -u neptune sqlite3 /opt/neptune/data/neptune.db
```
