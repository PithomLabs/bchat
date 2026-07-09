#!/bin/bash
# =============================================================================
# Fly.io Secrets Setup for Neon Postgres + S3 Deployment
# =============================================================================
# Sets all required Fly secrets for deploying bchat with Neon Postgres and
# Tigrisdata S3 LanceDB storage in one interactive session.
#
# Usage:
#   chmod +x scripts/fly-pg-secrets.sh
#   ./scripts/fly-pg-secrets.sh
#
# Prerequisites:
#   - fly CLI installed and authenticated
#   - Fly app already created (fly launch or fly apps create)
#   - Neon database created (https://console.neon.tech)
#
# Secrets set:
#   DATABASE_URL             - Neon Postgres connection string
#   OPENROUTER_API_KEY       - LLM and embedding API key
#   ENCRYPTION_MASTER_KEY    - Tenant API key encryption (auto-generated)
#   LANCEDB_S3_BUCKET        - Tigrisdata S3 bucket name
#   AWS_ACCESS_KEY_ID        - Tigrisdata credential (auto-set by fly storage)
#   AWS_SECRET_ACCESS_KEY    - Tigrisdata credential (auto-set by fly storage)
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}  bchat Fly.io Secrets Setup (Neon + S3)${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""

# =============================================================================
# Step 1: Check fly CLI
# =============================================================================
echo -e "${YELLOW}[1/7] Checking flyctl...${NC}"
if ! command -v fly &>/dev/null; then
    echo -e "${RED}flyctl is not installed.${NC}"
    echo "Install: curl -L https://fly.io/install.sh | sh"
    exit 1
fi
echo -e "${GREEN}  flyctl $(fly version)${NC}"
echo ""

# =============================================================================
# Step 2: Check authentication
# =============================================================================
echo -e "${YELLOW}[2/7] Checking authentication...${NC}"
if ! fly auth whoami &>/dev/null; then
    echo "Not logged in. Opening browser for authentication..."
    fly auth login
fi
echo -e "${GREEN}  Logged in as: $(fly auth whoami)${NC}"
echo ""

# =============================================================================
# Step 3: App name
# =============================================================================
echo -e "${YELLOW}[3/7] Fly app name...${NC}"
read -p "  Enter app name [bchat-pg]: " APP_NAME
APP_NAME="${APP_NAME:-bchat-pg}"

if ! fly status --app "$APP_NAME" &>/dev/null; then
    echo -e "${RED}  App '$APP_NAME' not found. Create it first:${NC}"
    echo "    fly apps create $APP_NAME"
    echo "    fly launch -c fly_pg.toml --app $APP_NAME --no-deploy"
    exit 1
fi
echo -e "${GREEN}  App: $APP_NAME${NC}"
echo ""

# =============================================================================
# Step 4: Neon DATABASE_URL
# =============================================================================
echo -e "${YELLOW}[4/7] Neon connection string...${NC}"
echo "  Get this from: https://console.neon.tech → Connection Details"
echo "  Format: postgresql://user:password@ep-xxx.us-east-2.aws.neon.tech/neondb?sslmode=require"
echo ""
read -p "  DATABASE_URL: " DATABASE_URL

if [[ -z "$DATABASE_URL" ]]; then
    echo -e "${RED}  DATABASE_URL is required.${NC}"
    exit 1
fi

if [[ "$DATABASE_URL" != *"sslmode=require"* ]]; then
    echo -e "${YELLOW}  Warning: sslmode=require not found in URL. Neon requires it.${NC}"
    read -p "  Append sslmode=require? (y/n): " fix_ssl
    if [[ "$fix_ssl" == "y" ]]; then
        DATABASE_URL="${DATABASE_URL}&sslmode=require"
        echo -e "${GREEN}  Updated: $DATABASE_URL${NC}"
    fi
fi
echo -e "${GREEN}  DATABASE_URL set${NC}"
echo ""

# =============================================================================
# Step 5: OPENROUTER_API_KEY
# =============================================================================
echo -e "${YELLOW}[5/7] OpenRouter API key...${NC}"
echo "  Get yours at: https://openrouter.ai/keys"
read -s -p "  OPENROUTER_API_KEY (sk-or-v1-...): " OPENROUTER_KEY
echo ""

if [[ -z "$OPENROUTER_KEY" ]]; then
    echo -e "${RED}  OPENROUTER_API_KEY is required.${NC}"
    exit 1
fi
echo -e "${GREEN}  OPENROUTER_API_KEY set${NC}"
echo ""

# =============================================================================
# Step 6: S3 storage (Tigrisdata)
# =============================================================================
echo -e "${YELLOW}[6/7] S3 storage (Tigrisdata for LanceDB)...${NC}"

# Check for existing storage (no --app flag — fly storage list shows all org buckets)
STORAGE_OUTPUT=$(fly storage list 2>/dev/null || true)
STORAGE_LINES=$(echo "$STORAGE_OUTPUT" | tail -n +3 | grep -v "^$" || true)

if [[ -n "$STORAGE_LINES" ]]; then
    echo -e "${GREEN}  Existing buckets:${NC}"
    echo "$STORAGE_OUTPUT" | sed 's/^/    /'
    echo ""
    read -p "  Enter the BUCKET_NAME to use: " BUCKET_NAME
else
    echo "  No S3 storage found."
    echo ""
    read -p "  Enter an existing BUCKET_NAME, or leave blank to create new: " BUCKET_NAME

    if [[ -z "$BUCKET_NAME" ]]; then
        echo "  Creating Tigrisdata bucket..."
        echo ""
        if fly storage create; then
            echo ""
            echo -e "${YELLOW}  Note the BUCKET_NAME from the output above.${NC}"
            read -p "  Enter the BUCKET_NAME: " BUCKET_NAME
        else
            echo ""
            echo -e "${YELLOW}  Creation failed (name may be taken). Enter an existing bucket name:${NC}"
            read -p "  BUCKET_NAME: " BUCKET_NAME
        fi
    fi
fi

if [[ -z "$BUCKET_NAME" ]]; then
    echo -e "${YELLOW}  Skipping S3 (bucket name empty). You can set it later:${NC}"
    echo "    fly secrets set LANCEDB_S3_BUCKET=your-bucket --app $APP_NAME"
    BUCKET_NAME=""
fi
echo ""

# =============================================================================
# Step 7: Set all secrets
# =============================================================================
echo -e "${YELLOW}[7/7] Setting Fly secrets...${NC}"
echo ""

# Auto-generate encryption key
ENCRYPTION_KEY=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)

# Set secrets
fly secrets set \
    DATABASE_URL="$DATABASE_URL" \
    OPENROUTER_API_KEY="$OPENROUTER_KEY" \
    ENCRYPTION_MASTER_KEY="$ENCRYPTION_KEY" \
    --app "$APP_NAME"

echo -e "${GREEN}  DATABASE_URL set${NC}"
echo -e "${GREEN}  OPENROUTER_API_KEY set${NC}"
echo -e "${GREEN}  ENCRYPTION_MASTER_KEY set${NC}"

# Set S3 bucket if provided
if [[ -n "$BUCKET_NAME" ]]; then
    fly secrets set LANCEDB_S3_BUCKET="$BUCKET_NAME" --app "$APP_NAME"
    echo -e "${GREEN}  LANCEDB_S3_BUCKET set${NC}"
fi

echo ""

# =============================================================================
# Summary
# =============================================================================
echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}  Secrets Configured!${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""
echo -e "${GREEN}App:${NC}            $APP_NAME"
echo -e "${GREEN}Database:${NC}       Neon Postgres"
echo -e "${GREEN}LanceDB storage:${NC} S3/Tigrisdata"
echo ""
echo -e "${GREEN}Secrets:${NC}"
fly secrets list --app "$APP_NAME"
echo ""
echo -e "${GREEN}Next steps:${NC}"
echo "  1. Validate migrations:"
echo "     task -t Taskfile_pg.yml validate:migrations"
echo ""
echo "  2. Deploy:"
echo "     fly -a $APP_NAME deploy -c fly_pg.toml"
echo ""
echo "  3. Verify:"
echo "     curl https://$APP_NAME.fly.dev/healthz"
echo "     fly -a $APP_NAME logs"
echo ""
