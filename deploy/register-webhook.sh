#!/usr/bin/env bash
set -euo pipefail

# register-webhook.sh — Register Telegram webhook for Neptune
#
# Usage:
#   ./deploy/register-webhook.sh <bot_domain> <webhook_secret>
#   ./deploy/register-webhook.sh bot.example.com my-secret-token

DOMAIN="${1:-}"
WEBHOOK_SECRET="${2:-}"
BOT_TOKEN="${BOT_TOKEN:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[webhook]${NC} $*"; }
err() { echo -e "${RED}[error]${NC} $*" >&2; }

if [ -z "$DOMAIN" ]; then
  err "Usage: $0 <bot_domain> <webhook_secret>"
  err "  bot_domain: Domain where Neptune is hosted (e.g., bot.example.com)"
  err "  webhook_secret: Secret token for webhook authentication"
  exit 1
fi

if [ -z "$BOT_TOKEN" ]; then
  err "BOT_TOKEN environment variable is required"
  err "Export it: export BOT_TOKEN=your-bot-token"
  exit 1
fi

WEBHOOK_URL="https://$DOMAIN/webhook"
SET_WEBHOOK_URL="https://$DOMAIN/set-webhook?token=$WEBHOOK_SECRET"

log "Registering webhook..."
log "  Domain: $DOMAIN"
log "  Webhook URL: $WEBHOOK_URL"
echo ""

# Method 1: Via Neptune's set-webhook endpoint (recommended)
log "Method 1: Via Neptune set-webhook endpoint..."
RESPONSE=$(curl -s "$SET_WEBHOOK_URL" 2>/dev/null || echo "")
if echo "$RESPONSE" | grep -q "Webhook set to"; then
  log "Webhook registered via Neptune endpoint!"
  log "  Response: $RESPONSE"
else
  log "Neptune endpoint response: $RESPONSE"
  log ""
  log "Method 2: Direct Telegram API..."
fi

# Method 2: Direct Telegram API (fallback)
log "Verifying via Telegram API..."
WEBHOOK_INFO=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getWebhookInfo" 2>/dev/null)

if echo "$WEBHOOK_INFO" | jq -e '.ok' >/dev/null 2>&1; then
  CURRENT_URL=$(echo "$WEBHOOK_INFO" | jq -r '.result.url')
  PENDING=$(echo "$WEBHOOK_INFO" | jq -r '.result.pending_update_count // 0')
  LAST_ERROR=$(echo "$WEBHOOK_INFO" | jq -r '.result.last_error_message // "none"')
  MAX_CONN=$(echo "$WEBHOOK_INFO" | jq -r '.result.max_connections // "default"')

  echo ""
  log "Webhook Info:"
  echo "  URL: $CURRENT_URL"
  echo "  Pending updates: $PENDING"
  echo "  Last error: $LAST_ERROR"
  echo "  Max connections: $MAX_CONN"

  if [ "$CURRENT_URL" = "$WEBHOOK_URL" ]; then
    echo ""
    log "Webhook is correctly registered!"
  else
    echo ""
    warn "Webhook URL mismatch!"
    warn "  Expected: $WEBHOOK_URL"
    warn "  Current:  $CURRENT_URL"
    echo ""
    log "Setting webhook via Telegram API..."
    RESULT=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/setWebhook?url=$WEBHOOK_URL&allowed_updates=[\"message\",\"chat_member\",\"callback_query\"]" 2>/dev/null)
    if echo "$RESULT" | jq -e '.ok' >/dev/null 2>&1; then
      log "Webhook set successfully!"
    else
      err "Failed to set webhook: $RESULT"
    fi
  fi
else
  err "Failed to get webhook info: $WEBHOOK_INFO"
fi

# Also set commands
log ""
log "Syncing bot commands..."
COMMANDS='[
  {"command":"help","description":"显示帮助信息"},
  {"command":"ping","description":"检查机器人是否在线"},
  {"command":"id","description":"获取当前群组 ID"},
  {"command":"connect","description":"绑定私聊与群组"},
  {"command":"switch","description":"切换管理的群组"},
  {"command":"setwelcome","description":"设置欢迎消息"},
  {"command":"enablewelcome","description":"启用入群欢迎"},
  {"command":"disablewelcome","description":"禁用入群欢迎"},
  {"command":"rule","description":"设置群规"},
  {"command":"setverifybutton","description":"设置认证按钮文案"},
  {"command":"setverifytimeout","description":"设置认证超时时间"},
  {"command":"testverify","description":"测试验证消息"},
  {"command":"addkeyword","description":"添加关键词规则"},
  {"command":"addregex","description":"添加正则规则"},
  {"command":"listkeywords","description":"列出所有规则"},
  {"command":"removekeyword","description":"删除规则"},
  {"command":"enablevotekick","description":"启用投票踢人"},
  {"command":"disablevotekick","description":"禁用投票踢人"},
  {"command":"kick","description":"发起踢人投票"},
  {"command":"report","description":"举报违规消息"},
  {"command":"warn","description":"警告用户"}
]'

RESULT=$(curl -s -X POST "https://api.telegram.org/bot$BOT_TOKEN/setMyCommands" \
  -H "Content-Type: application/json" \
  -d "{\"commands\":$COMMANDS}" 2>/dev/null)

if echo "$RESULT" | jq -e '.ok' >/dev/null 2>&1; then
  log "Bot commands synced (21 commands)"
else
  err "Failed to sync commands: $RESULT"
fi

echo ""
log "Done!"
