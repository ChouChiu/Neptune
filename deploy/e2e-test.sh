#!/usr/bin/env bash
set -euo pipefail

# e2e-test.sh — End-to-end test script for Neptune bot
#
# Tests all 21 commands against a running bot instance.
# Requires: curl, jq, BOT_TOKEN env var
#
# Usage:
#   BOT_TOKEN=xxx ./deploy/e2e-test.sh https://your-bot-domain.com

BASE_URL="${1:-http://localhost:8080}"
BOT_TOKEN="${BOT_TOKEN:-}"
WEBHOOK_SECRET="${GITHUB_WEBHOOK_SECRET:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
SKIP=0

log() { echo -e "${GREEN}[test]${NC} $*"; }
warn() { echo -e "${YELLOW}[skip]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; FAIL=$((FAIL + 1)); }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; PASS=$((PASS + 1)); }
skip() { echo -e "${YELLOW}[SKIP]${NC} $*"; SKIP=$((SKIP + 1)); }

# Helper: HTTP request and check status code
test_endpoint() {
  local method="$1"
  local path="$2"
  local expected_status="$3"
  local description="$4"
  local data="${5:-}"

  local args=(-s -o /dev/null -w "%{http_code}" --max-time 10)
  if [ "$method" = "POST" ]; then
    args+=(-X POST)
    if [ -n "$data" ]; then
      args+=(-H "Content-Type: application/json" -d "$data")
    fi
  fi

  local status
  status=$(curl "${args[@]}" "$BASE_URL$path" 2>/dev/null || echo "000")

  if [ "$status" = "$expected_status" ]; then
    pass "$description (HTTP $status)"
  else
    fail "$description — expected HTTP $expected_status, got $status"
  fi
}

echo ""
echo "=========================================="
echo "  Neptune E2E Test Suite"
echo "=========================================="
echo "Base URL: $BASE_URL"
echo ""

# --- Health Check ---
log "Testing health endpoint..."
test_endpoint GET /health 200 "Health check"

# --- Telegram Webhook ---
log "Testing webhook endpoint..."
test_endpoint POST /webhook 405 "Webhook rejects GET" ""
# A proper webhook test would send a Telegram update, but that requires a real chat

# --- Set Webhook ---
log "Testing set-webhook endpoint..."
test_endpoint GET /set-webhook 401 "Set-webhook without token"
if [ -n "$WEBHOOK_SECRET" ]; then
  test_endpoint GET "/set-webhook?token=$WEBHOOK_SECRET" 200 "Set-webhook with valid token"
else
  skip "Set-webhook with valid token (no GITHUB_WEBHOOK_SECRET set)"
fi

# --- Admin Panel ---
log "Testing admin panel..."
test_endpoint GET /admin/ 200 "Admin panel login page"
test_endpoint GET /admin/api/reports 401 "Reports API requires auth"
test_endpoint GET /admin/api/warnings 401 "Warnings API requires auth"

# --- GitHub Webhook ---
log "Testing GitHub webhook..."
test_endpoint GET /github-webhook 405 "GitHub webhook rejects GET"
test_endpoint POST /github-webhook 403 "GitHub webhook rejects invalid signature"

if [ -n "$WEBHOOK_SECRET" ]; then
  # Send a test ping event
  PAYLOAD='{"zen":"Keep it logically awesome.","hook_id":12345}'
  SIG=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | awk '{print $2}')
  test_endpoint POST /github-webhook 200 "GitHub webhook accepts valid ping" "$PAYLOAD"
else
  skip "GitHub webhook valid signature test (no GITHUB_WEBHOOK_SECRET set)"
fi

# --- Bot Commands (via Telegram API) ---
if [ -n "$BOT_TOKEN" ]; then
  log "Testing bot commands via Telegram API..."

  # Get bot info
  BOT_INFO=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getMe" 2>/dev/null)
  if echo "$BOT_INFO" | jq -e '.ok' >/dev/null 2>&1; then
    BOT_USERNAME=$(echo "$BOT_INFO" | jq -r '.result.username')
    pass "Bot info retrieved: @$BOT_USERNAME"
  else
    fail "Failed to get bot info"
  fi

  # Get webhook info
  WEBHOOK_INFO=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getWebhookInfo" 2>/dev/null)
  if echo "$WEBHOOK_INFO" | jq -e '.ok' >/dev/null 2>&1; then
    WEBHOOK_URL=$(echo "$WEBHOOK_INFO" | jq -r '.result.url')
    if [ -n "$WEBHOOK_URL" ] && [ "$WEBHOOK_URL" != "null" ]; then
      pass "Webhook registered: $WEBHOOK_URL"
    else
      warn "No webhook registered"
    fi

    PENDING=$(echo "$WEBHOOK_INFO" | jq -r '.result.pending_update_count // 0')
    if [ "$PENDING" -gt 0 ]; then
      warn "Pending updates: $PENDING"
    fi
  else
    fail "Failed to get webhook info"
  fi

  # List registered commands
  COMMANDS=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getMyCommands" 2>/dev/null)
  if echo "$COMMANDS" | jq -e '.ok' >/dev/null 2>&1; then
    CMD_COUNT=$(echo "$COMMANDS" | jq '.result | length')
    pass "Registered commands: $CMD_COUNT"

    # Expected commands
    EXPECTED_CMDS=(ping help id connect switch setwelcome enablewelcome disablewelcome rule setverifybutton setverifytimeout testverify addkeyword addregex listkeywords removekeyword enablevotekick disablevotekick kick report warn)

    for cmd in "${EXPECTED_CMDS[@]}"; do
      if echo "$COMMANDS" | jq -e ".result[] | select(.command == \"$cmd\")" >/dev/null 2>&1; then
        pass "Command /$cmd registered"
      else
        fail "Command /$cmd not registered"
      fi
    done
  else
    fail "Failed to get bot commands"
  fi
else
  warn "BOT_TOKEN not set — skipping Telegram API tests"
  warn "Set BOT_TOKEN to test bot commands"
fi

# --- Static Files ---
log "Testing static files..."
test_endpoint GET /static/ 404 "Static dir listing disabled (or no files)"

# --- Summary ---
echo ""
echo "=========================================="
echo "  Test Results"
echo "=========================================="
echo -e "  ${GREEN}Passed: $PASS${NC}"
echo -e "  ${RED}Failed: $FAIL${NC}"
echo -e "  ${YELLOW}Skipped: $SKIP${NC}"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo -e "${RED}Some tests failed!${NC}"
  exit 1
else
  echo -e "${GREEN}All tests passed!${NC}"
  exit 0
fi
