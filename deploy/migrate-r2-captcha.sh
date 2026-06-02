#!/usr/bin/env bash
set -euo pipefail

# migrate-r2-captcha.sh — Download captcha images from R2 to local filesystem
#
# Prerequisites:
#   - wrangler CLI installed and authenticated
#   - rclone or aws CLI configured for R2 (optional, for bulk download)
#
# Usage:
#   ./deploy/migrate-r2-captcha.sh [output_dir]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${1:-$PROJECT_DIR/data/captcha}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[captcha]${NC} $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }
err() { echo -e "${RED}[error]${NC} $*" >&2; }

# Get R2 bucket name from wrangler.toml
R2_BUCKET=""
if [ -f "$PROJECT_DIR/wrangler.toml" ]; then
  R2_BUCKET=$(grep -oP 'bucket_name\s*=\s*"\K[^"]+' "$PROJECT_DIR/wrangler.toml" || true)
fi

if [ -z "$R2_BUCKET" ]; then
  err "Could not find R2 bucket_name in wrangler.toml"
  err "Please set R2_BUCKET environment variable or configure wrangler.toml"
  exit 1
fi

log "R2 bucket: $R2_BUCKET"
log "Output directory: $OUTPUT_DIR"

mkdir -p "$OUTPUT_DIR"

# Method 1: Try rclone (preferred for bulk download)
if command -v rclone >/dev/null 2>&1; then
  log "Using rclone for download..."

  # Configure rclone for R2 if not already configured
  RCLONE_CONFIG="${RCLONE_CONFIG:-$HOME/.config/rclone/rclone.conf}"
  if ! rclone listremotes 2>/dev/null | grep -q "^r2:"; then
    warn "rclone R2 remote not configured."
    warn "Run: rclone config"
    warn "Or set R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY env vars"
    warn ""
    warn "Falling back to wrangler..."

    # Fall through to wrangler method
  else
    log "Downloading captcha files from R2..."
    rclone copy "r2:$R2_BUCKET/captcha/" "$OUTPUT_DIR/" --progress --transfers 4
    log "Download complete!"
    log "Files saved to: $OUTPUT_DIR"
    exit 0
  fi
fi

# Method 2: Use wrangler (slower, but no extra dependencies)
log "Using wrangler for download..."
warn "This may be slow for large numbers of files."

# List objects in R2 bucket under captcha/ prefix
OBJECTS_FILE=$(mktemp)
trap "rm -f $OBJECTS_FILE" EXIT

log "Listing captcha files in R2..."
wrangler r2 object list --bucket "$R2_BUCKET" --prefix "captcha/" > "$OBJECTS_FILE" 2>/dev/null || {
  # Try alternative wrangler syntax
  wrangler r2 bucket list --bucket "$R2_BUCKET" --prefix "captcha/" > "$OBJECTS_FILE" 2>/dev/null || {
    err "Failed to list R2 objects. Check your wrangler configuration."
    err ""
    err "Manual download steps:"
    err "  1. Install rclone: https://rclone.org/install/"
    err "  2. Configure R2: rclone config"
    err "  3. Run: rclone copy r2:$R2_BUCKET/captcha/ $OUTPUT_DIR/"
    exit 1
  }
}

# Parse object keys and download each
DOWNLOADED=0
FAILED=0
while IFS= read -r key; do
  [ -z "$key" ] && continue

  # Create local directory structure
  local_path="$OUTPUT_DIR/${key#captcha/}"
  mkdir -p "$(dirname "$local_path")"

  # Download object
  if wrangler r2 object get --bucket "$R2_BUCKET" --key "$key" --file "$local_path" 2>/dev/null; then
    DOWNLOADED=$((DOWNLOADED + 1))
  else
    FAILED=$((FAILED + 1))
    warn "Failed to download: $key"
  fi
done < <(grep -oP 'captcha/[^ ]+' "$OBJECTS_FILE" || true)

log "Download complete!"
log "Downloaded: $DOWNLOADED, Failed: $FAILED"
log "Files saved to: $OUTPUT_DIR"

# List directory structure
log "Directory structure:"
find "$OUTPUT_DIR" -type f -name "*.bmp" | head -20 | while read -r f; do
  echo "  $f"
done
TOTAL=$(find "$OUTPUT_DIR" -type f -name "*.bmp" | wc -l)
if [ "$TOTAL" -gt 20 ]; then
  log "  ... and $((TOTAL - 20)) more files"
fi
