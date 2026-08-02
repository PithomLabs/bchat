#!/bin/bash
# =============================================================================
# Fly.io Secrets Setup for CockroachDB Deployment
# =============================================================================
# Sets all required Fly secrets for deploying bchat with a CockroachDB
# backend (application data + LanceDB vector store) in one interactive session.
#
# Usage:
#   chmod +x scripts/fly-cockroach-secrets.sh
#   ./scripts/fly-cockroach-secrets.sh
#
# Prerequisites:
#   - fly CLI installed and authenticated
#   - Fly app already created (fly apps create bchat-crdb)
#   - CockroachDB cluster provisioned with a connection string
#
# Secrets set:
#   COCKROACH_DSN           - CockroachDB connection string
#   OPENROUTER_API_KEY      - LLM and embedding API key
#   ENCRYPTION_MASTER_KEY   - Tenant API key encryption (auto-generated)
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}  bchat Fly.io Secrets Setup (CockroachDB)${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""

# =============================================================================
# Step 1: Check fly CLI
# =============================================================================
echo -e "${YELLOW}[1/5] Checking flyctl...${NC}"
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
echo -e "${YELLOW}[2/5] Checking authentication...${NC}"
if ! fly auth whoami &>/dev/null; then
    echo "Not logged in. Opening browser for authentication..."
    fly auth login
fi
echo -e "${GREEN}  Logged in as: $(fly auth whoami)${NC}"
echo ""

# =============================================================================
# Step 3: App name
# =============================================================================
echo -e "${YELLOW}[3/5] Fly app name...${NC}"
read -p "  Enter app name [bchat-crdb]: " APP_NAME
APP_NAME="${APP_NAME:-bchat-crdb}"

if ! fly status --app "$APP_NAME" &>/dev/null; then
    echo -e "${RED}  App '$APP_NAME' not found. Create it first:${NC}"
    echo "    fly apps create $APP_NAME"
    echo "    fly launch -c fly_cockroach.toml --app $APP_NAME --no-deploy"
    exit 1
fi
echo -e "${GREEN}  App: $APP_NAME${NC}"
echo ""

# =============================================================================
# Step 4: COCKROACH_DSN
# =============================================================================
echo -e "${YELLOW}[4/5] CockroachDB connection string...${NC}"
echo "  Get this from: CockroachDB Cloud Console → Connect"
echo "  Format: postgresql://user:password@host:26257/bchat?sslmode=require"
echo ""
read -p "  COCKROACH_DSN: " COCKROACH_DSN

if [[ -z "$COCKROACH_DSN" ]]; then
    echo -e "${RED}  COCKROACH_DSN is required.${NC}"
    exit 1
fi

if [[ "$COCKROACH_DSN" != *"sslmode="* ]]; then
    echo -e "${YELLOW}  Warning: sslmode not found in URL. CockroachDB Cloud requires sslmode=require.${NC}"
    read -p "  Append sslmode=require? (y/n): " fix_ssl
    if [[ "$fix_ssl" == "y" ]]; then
        COCKROACH_DSN="${COCKROACH_DSN}&sslmode=require"
        echo -e "${GREEN}  Updated: $COCKROACH_DSN${NC}"
    fi
fi
echo -e "${GREEN}  COCKROACH_DSN set${NC}"
echo ""

# =============================================================================
# Step 5: Set all secrets
# =============================================================================
echo -e "${YELLOW}[5/5] Setting Fly secrets...${NC}"
echo ""

# Auto-generate encryption key
ENCRYPTION_KEY=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)

echo "  OPENROUTER_API_KEY (sk-or-v1-...): "
read -s -p "  " OPENROUTER_KEY
echo ""

if [[ -z "$OPENROUTER_KEY" ]]; then
    echo -e "${RED}  OPENROUTER_API_KEY is required.${NC}"
    exit 1
fi

fly secrets set \
    COCKROACH_DSN="$COCKROACH_DSN" \
    OPENROUTER_API_KEY="$OPENROUTER_KEY" \
    ENCRYPTION_MASTER_KEY="$ENCRYPTION_KEY" \
    --app "$APP_NAME"

echo -e "${GREEN}  COCKROACH_DSN set${NC}"
echo -e "${GREEN}  OPENROUTER_API_KEY set${NC}"
echo -e "${GREEN}  ENCRYPTION_MASTER_KEY set${NC}"
echo ""

# =============================================================================
# Summary
# =============================================================================
echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}  Secrets Configured!${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""
echo -e "${GREEN}App:${NC}            $APP_NAME"
echo -e "${GREEN}Database:${NC}       CockroachDB (app + vectors)"
echo ""
echo -e "${GREEN}Secrets:${NC}"
fly secrets list --app "$APP_NAME"
echo ""
echo -e "${GREEN}Next steps:${NC}"
echo "  1. Validate migration parity + CockroachDB compatibility:"
echo "     task validate:parity"
echo "     bash scripts/validate-cockroach-compat.sh"
echo ""
echo "  2. Deploy:"
echo "     fly -a $APP_NAME deploy -c fly_cockroach.toml"
echo ""
echo "  3. Verify:"
echo "     curl https://$APP_NAME.fly.dev/healthz"
echo "     fly -a $APP_NAME logs"
echo ""
