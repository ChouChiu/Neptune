#!/usr/bin/env bash
set -euo pipefail

# setup.sh — Initial server setup for Neptune deployment
#
# Run on the target VPS as root or with sudo.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/kazumi-group/neptune/main/deploy/setup.sh | sudo bash
#   # or
#   sudo ./deploy/setup.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="/opt/neptune"
SERVICE_NAME="neptune"
DOMAIN="${NEPTUNE_DOMAIN:-bot.example.com}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log() { echo -e "${GREEN}[setup]${NC} $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }
err() { echo -e "${RED}[error]${NC} $*" >&2; }

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  err "Please run as root (use sudo)"
  exit 1
fi

log "Neptune deployment setup"
log "========================"
log "Install directory: $INSTALL_DIR"
log "Domain: $DOMAIN"
echo ""

# 1. Create neptune user
if ! id -u neptune >/dev/null 2>&1; then
  log "Creating neptune user..."
  useradd --system --shell /bin/false --home-dir "$INSTALL_DIR" neptune
  log "User 'neptune' created"
else
  log "User 'neptune' already exists"
fi

# 2. Create directory structure
log "Creating directory structure..."
mkdir -p "$INSTALL_DIR"/{data/captcha,migrations,static,deploy}
chown -R neptune:neptune "$INSTALL_DIR"
chmod 755 "$INSTALL_DIR"
chmod 700 "$INSTALL_DIR/data"

# 3. Install systemd service
log "Installing systemd service..."
if [ -f "$SCRIPT_DIR/neptune.service" ]; then
  cp "$SCRIPT_DIR/neptune.service" "/etc/systemd/system/${SERVICE_NAME}.service"
else
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<'EOF'
[Unit]
Description=Neptune Telegram Bot
After=network.target

[Service]
Type=simple
User=neptune
Group=neptune
WorkingDirectory=/opt/neptune
ExecStart=/opt/neptune/neptune
Restart=always
RestartSec=5
EnvironmentFile=/opt/neptune/.env
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/neptune/data
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
EOF
fi
systemctl daemon-reload
log "Systemd service installed"

# 4. Setup Nginx
log "Setting up Nginx..."
if ! command -v nginx >/dev/null 2>&1; then
  warn "Nginx not installed. Install with: apt install nginx"
else
  if [ -f "$SCRIPT_DIR/nginx.conf" ]; then
    # Replace domain placeholder
    sed "s/bot.example.com/$DOMAIN/g" "$SCRIPT_DIR/nginx.conf" > "/etc/nginx/sites-available/${SERVICE_NAME}"
  else
    cat > "/etc/nginx/sites-available/${SERVICE_NAME}" <<EOF
server {
    listen 80;
    server_name $DOMAIN;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF
  fi

  ln -sf "/etc/nginx/sites-available/${SERVICE_NAME}" "/etc/nginx/sites-enabled/${SERVICE_NAME}"
  rm -f /etc/nginx/sites-enabled/default

  if nginx -t 2>/dev/null; then
    systemctl reload nginx
    log "Nginx configured and reloaded"
  else
    warn "Nginx config test failed — check /etc/nginx/sites-available/${SERVICE_NAME}"
  fi
fi

# 5. Setup SSL with certbot
log "Checking SSL certificate..."
if command -v certbot >/dev/null 2>&1; then
  if [ ! -d "/etc/letsencrypt/live/$DOMAIN" ]; then
    warn "SSL certificate not found for $DOMAIN"
    warn "Run: sudo certbot certonly --nginx -d $DOMAIN"
  else
    log "SSL certificate exists for $DOMAIN"
  fi
else
  warn "certbot not installed. Install with: apt install certbot python3-certbot-nginx"
fi

# 6. Create .env template
ENV_FILE="$INSTALL_DIR/.env"
if [ ! -f "$ENV_FILE" ]; then
  log "Creating .env template..."
  cat > "$ENV_FILE" <<'EOF'
# Neptune environment variables
# Fill in the values below before starting the bot.

# Telegram Bot Token (from @BotFather)
BOT_TOKEN=

# Bot username (without @)
BOT_USERNAME=

# MiMo API Key
MIMO_API_KEY=

# GitHub Webhook Secret
GITHUB_WEBHOOK_SECRET=

# Release notification channel ID (optional)
RELEASE_CHANNEL_ID=

# Reuse captcha images (true/false, optional)
REUSE_CAPTCHA=false

# Database path (default: data/neptune.db)
DB_PATH=data/neptune.db

# Listen address (default: :8080)
LISTEN_ADDR=:8080
EOF
  chmod 600 "$ENV_FILE"
  chown neptune:neptune "$ENV_FILE"
  log "Created $ENV_FILE — fill in your secrets!"
else
  log ".env file already exists"
fi

# 7. Setup log rotation
log "Setting up log rotation..."
cat > /etc/logrotate.d/neptune <<EOF
/var/log/neptune/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    missingok
    create 0640 neptune neptune
}
EOF
log "Log rotation configured"

# 8. Summary
echo ""
log "Setup complete!"
echo ""
echo -e "${CYAN}Next steps:${NC}"
echo "  1. Edit $ENV_FILE with your secrets"
echo "  2. Copy the neptune binary to $INSTALL_DIR/neptune"
echo "  3. Copy migrations to $INSTALL_DIR/migrations/"
echo "  4. Run: sudo systemctl enable $SERVICE_NAME"
echo "  5. Run: sudo systemctl start $SERVICE_NAME"
echo "  6. Set up SSL: sudo certbot certonly --nginx -d $DOMAIN"
echo "  7. Register webhook: curl https://$DOMAIN/set-webhook?token=YOUR_WEBHOOK_SECRET"
echo ""
echo -e "${CYAN}Useful commands:${NC}"
echo "  sudo systemctl status $SERVICE_NAME    # Check status"
echo "  sudo journalctl -u $SERVICE_NAME -f    # View logs"
echo "  sudo systemctl restart $SERVICE_NAME    # Restart"
