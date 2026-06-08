#!/usr/bin/env bash
set -euo pipefail

# setup.sh — Neptune Docker 部署初始化脚本
#
# 由 `make setup` 调用，也可单独在服务器上运行。
# 需要 root 权限。

INSTALL_DIR="/opt/neptune"
REPO_URL="https://github.com/ChouChiu/Neptune.git"

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

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           Neptune Docker 部署初始化                      ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ========== 1. 安装 Docker ==========
step "1/4 检查 Docker"
if command -v docker >/dev/null 2>&1; then
  log "Docker 已安装：$(docker --version)"
else
  warn "Docker 未安装，正在安装..."
  curl -fsSL https://get.docker.com | sh
  log "Docker 已安装：$(docker --version)"
fi

if docker compose version >/dev/null 2>&1; then
  log "Docker Compose 已安装：$(docker compose version --short)"
else
  warn "Docker Compose 插件未安装，请手动安装"
fi

# ========== 2. 克隆仓库 ==========
step "2/4 克隆仓库"
if [ -d "$INSTALL_DIR/.git" ]; then
  log "仓库已存在，跳过克隆"
else
  git clone "$REPO_URL" "$INSTALL_DIR"
  log "仓库已克隆到 $INSTALL_DIR"
fi

# ========== 3. 创建配置文件 ==========
step "3/4 创建配置文件"
ENV_FILE="$INSTALL_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  log ".env 文件已存在，跳过创建"
else
  cat > "$ENV_FILE" <<'EOF'
# Neptune 环境变量配置

# ===== 必填 =====
BOT_TOKEN=
BOT_USERNAME=
HERMES_API_URL=http://host.docker.internal:8642/v1
HERMES_API_KEY=

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
  log "已创建 $ENV_FILE（权限 600）"
fi

# ========== 4. 创建数据目录 ==========
step "4/4 创建数据目录"
mkdir -p "$INSTALL_DIR/data"
log "数据目录已就绪"

# ========== 完成 ==========
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           初始化完成！                                    ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "接下来请完成以下步骤："
echo ""
echo "  ① 编辑配置文件，填入 secrets："
echo "     nano $ENV_FILE"
echo ""
echo "  ② 构建并启动容器："
echo "     cd $INSTALL_DIR"
echo "     docker compose up --build -d"
echo ""
echo "  ③ 查看运行状态："
echo "     docker compose ps"
echo "     docker compose logs -f"
echo ""
echo "  ④ 配置外部反向代理（Nginx/Caddy）指向 127.0.0.1:8080"
echo ""
echo "  ⑤ 注册 Telegram webhook："
echo "     ./deploy/register-webhook.sh 你的域名 你的密钥"
echo ""
