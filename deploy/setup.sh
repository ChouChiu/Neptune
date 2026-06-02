#!/usr/bin/env bash
set -euo pipefail

# setup.sh — Neptune 服务器初始化脚本
#
# 由 `make setup` 调用，也可单独在服务器上运行。
# 需要 root 权限。

INSTALL_DIR="/opt/neptune"
SERVICE_NAME="neptune"
DOMAIN="${NEPTUNE_DOMAIN:-}"

# ========== 颜色输出 ==========
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✔]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✘]${NC} $*" >&2; }
step() { echo -e "\n${CYAN}${BOLD}── $* ──${NC}"; }

# ========== 前置检查 ==========
if [ "$EUID" -ne 0 ]; then
  err "请使用 root 权限运行：sudo bash setup.sh"
  exit 1
fi

if [ -z "$DOMAIN" ]; then
  err "请设置域名：NEPTUNE_DOMAIN=你的域名 sudo bash setup.sh"
  err "或通过 make setup DEPLOY_HOST=root@服务器 DOMAIN=你的域名"
  exit 1
fi

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           Neptune 服务器初始化                           ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "  域名:     $DOMAIN"
echo "  安装目录: $INSTALL_DIR"
echo ""

# ========== 1. 创建系统用户 ==========
step "1/6 创建系统用户"
if id -u neptune >/dev/null 2>&1; then
  log "用户 'neptune' 已存在"
else
  useradd --system --shell /bin/false --home-dir "$INSTALL_DIR" neptune
  log "已创建用户 'neptune'"
fi

# ========== 2. 创建目录结构 ==========
step "2/6 创建目录结构"
mkdir -p "$INSTALL_DIR"/{data/captcha,migrations,static,deploy}
chown -R neptune:neptune "$INSTALL_DIR"
chmod 755 "$INSTALL_DIR"
chmod 700 "$INSTALL_DIR/data"
log "目录已创建：$INSTALL_DIR/"

# ========== 3. 安装 systemd 服务 ==========
step "3/6 安装 systemd 服务"
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<'EOF'
[Unit]
Description=Neptune Telegram Bot
After=network.target
Documentation=https://github.com/kazumi-group/neptune

[Service]
Type=simple
User=neptune
Group=neptune
WorkingDirectory=/opt/neptune
ExecStart=/opt/neptune/neptune
Restart=always
RestartSec=5
EnvironmentFile=/opt/neptune/.env

# 安全加固
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/neptune/data
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

# 日志
StandardOutput=journal
StandardError=journal
SyslogIdentifier=neptune

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
log "systemd 服务已安装并启用"

# ========== 4. 配置 Nginx ==========
step "4/6 配置 Nginx"
if ! command -v nginx >/dev/null 2>&1; then
  warn "Nginx 未安装，正在安装..."
  apt-get update -qq && apt-get install -y -qq nginx >/dev/null 2>&1
  log "Nginx 已安装"
fi

# 生成 Nginx 配置（先用 HTTP，SSL 后续配置）
cat > "/etc/nginx/sites-available/${SERVICE_NAME}" <<EOF
# Neptune Nginx 配置
# SSL 会在运行 certbot 后自动添加

limit_req_zone \$binary_remote_addr zone=webhook:10m rate=30r/s;

server {
    listen 80;
    server_name $DOMAIN;

    # Telegram webhook（限流）
    location /webhook {
        limit_req zone=webhook burst=50 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 30s;
    }

    # GitHub webhook
    location /github-webhook {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 30s;
    }

    # 管理面板
    location /admin/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 30s;
    }

    # 健康检查
    location /health {
        proxy_pass http://127.0.0.1:8080;
        access_log off;
    }

    # Webhook 注册端点
    location /set-webhook {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # 其他请求返回 404
    location / {
        return 404;
    }

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain application/json;
}
EOF

ln -sf "/etc/nginx/sites-available/${SERVICE_NAME}" "/etc/nginx/sites-enabled/${SERVICE_NAME}"
rm -f /etc/nginx/sites-enabled/default

if nginx -t 2>/dev/null; then
  systemctl reload nginx
  log "Nginx 配置完成"
else
  warn "Nginx 配置测试失败，请检查 /etc/nginx/sites-available/${SERVICE_NAME}"
fi

# ========== 5. 创建 .env 配置文件 ==========
step "5/6 创建配置文件"
ENV_FILE="$INSTALL_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  log ".env 文件已存在，跳过创建"
else
  cat > "$ENV_FILE" <<'EOF'
# Neptune 环境变量配置
# 请填入以下必填项后启动服务

# ===== 必填 =====
BOT_TOKEN=
BOT_USERNAME=
MIMO_API_KEY=

# ===== 建议配置 =====
GITHUB_WEBHOOK_SECRET=
RELEASE_CHANNEL_ID=

# ===== 可选 =====
REUSE_CAPTCHA=false
LISTEN_ADDR=:8080
DB_PATH=data/neptune.db
DATA_DIR=./data
EOF
  chmod 600 "$ENV_FILE"
  chown neptune:neptune "$ENV_FILE"
  log "已创建 $ENV_FILE（权限 600）"
fi

# ========== 6. 安装 SSL 证书（如果 certbot 可用） ==========
step "6/6 检查 SSL 证书"
if command -v certbot >/dev/null 2>&1; then
  if [ -d "/etc/letsencrypt/live/$DOMAIN" ]; then
    log "SSL 证书已存在：$DOMAIN"
  else
    warn "SSL 证书尚未配置"
    echo ""
    echo "  服务启动后，请运行以下命令获取证书："
    echo "    sudo certbot certonly --nginx -d $DOMAIN"
    echo ""
  fi
else
  warn "certbot 未安装"
  echo ""
  echo "  安装 certbot："
  echo "    sudo apt install certbot python3-certbot-nginx"
  echo ""
  echo "  然后获取 SSL 证书："
  echo "    sudo certbot certonly --nginx -d $DOMAIN"
  echo ""
fi

# ========== 完成 ==========
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           服务器初始化完成！                              ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "接下来请完成以下步骤："
echo ""
echo "  ① 编辑配置文件，填入 secrets："
echo "     sudo nano $ENV_FILE"
echo ""
echo "  ② 将 neptune 二进制上传到服务器（make setup 会自动完成）："
echo "     $INSTALL_DIR/neptune"
echo ""
echo "  ③ 启动服务："
echo "     sudo systemctl start $SERVICE_NAME"
echo ""
echo "  ④ 查看运行状态："
echo "     sudo systemctl status $SERVICE_NAME"
echo "     sudo journalctl -u $SERVICE_NAME -f    # 查看日志"
echo ""
echo "  ⑤ 配置 SSL 证书："
echo "     sudo certbot certonly --nginx -d $DOMAIN"
echo ""
echo "  ⑥ 注册 Telegram webhook："
echo "     make webhook DOMAIN=$DOMAIN WEBHOOK_SECRET=你的密钥"
echo ""
