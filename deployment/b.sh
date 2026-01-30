#!/bin/bash
# =============================================================================
# bchat fly.io Deployment Setup Script
# =============================================================================
# This script sets up all prerequisites for deploying bchat to fly.io.
# Run this BEFORE running 'fly deploy'.
#
# Usage:
#   chmod +x bchat-fly-setup.sh
#   ./bchat-fly-setup.sh
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project directory
PROJECT_DIR="/home/chaschel/Documents/ibm/ai/bchat"

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}  bchat fly.io Deployment Setup${NC}"
echo -e "${BLUE}=========================================${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 1: Check flyctl installation
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Step 1/7] Checking flyctl installation...${NC}"
if ! command -v fly &> /dev/null; then
    echo -e "${RED}flyctl is not installed.${NC}"
    echo "Install it with:"
    echo "  curl -L https://fly.io/install.sh | sh"
    echo ""
    read -p "Install flyctl now? (y/n): " install_fly
    if [[ "$install_fly" == "y" ]]; then
        curl -L https://fly.io/install.sh | sh
        export PATH="$HOME/.fly/bin:$PATH"
    else
        echo -e "${RED}Cannot proceed without flyctl. Exiting.${NC}"
        exit 1
    fi
fi
echo -e "${GREEN}✓ flyctl is installed: $(fly version)${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 2: Login to fly.io
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Step 2/7] Checking fly.io authentication...${NC}"
if ! fly auth whoami &> /dev/null; then
    echo "Not logged in. Opening browser for authentication..."
    fly auth login
fi
echo -e "${GREEN}✓ Logged in as: $(fly auth whoami)${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 3: Choose storage type
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Step 3/7] Choose storage type for LanceDB:${NC}"
echo ""
echo "  1) LOCAL  - Store LanceDB on fly volume (simpler, single volume)"
echo "  2) S3     - Store LanceDB on Tigrisdata S3 (recommended for production)"
echo ""
read -p "Enter choice (1 or 2): " storage_choice

if [[ "$storage_choice" == "2" ]]; then
    STORAGE_TYPE="s3"
    FLY_TOML="fly.s3.toml"
    DOCKERFILE="Dockerfile.s3.fly"
else
    STORAGE_TYPE="local"
    FLY_TOML="fly.local.toml"
    DOCKERFILE="Dockerfile.local.fly"
fi
echo -e "${GREEN}✓ Selected: ${STORAGE_TYPE} storage${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 4: Create/Launch fly app
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Step 4/7] Creating fly.io app...${NC}"
cd "$PROJECT_DIR"

# Copy the appropriate fly.toml template
echo "Copying ${FLY_TOML} to fly.toml..."
cp "$FLY_TOML" fly.toml

# Check if app already exists
if fly status &> /dev/null; then
    echo -e "${GREEN}✓ App 'bchat' already exists${NC}"
else
    echo "Creating new fly app..."
    fly launch --no-deploy --copy-config --name bchat --region ord
fi
echo ""

# -----------------------------------------------------------------------------
# Step 5: Create Tigrisdata storage (S3 only)
# -----------------------------------------------------------------------------
if [[ "$STORAGE_TYPE" == "s3" ]]; then
    echo -e "${YELLOW}[Step 5/7] Creating Tigrisdata S3 storage...${NC}"

    # Check if storage already exists
    if fly storage list 2>/dev/null | grep -q "bchat"; then
        echo -e "${GREEN}✓ Tigrisdata storage already exists${NC}"
        echo ""
        echo "If you need the bucket name, run: fly storage list"
        read -p "Enter your existing LANCEDB_S3_BUCKET name: " bucket_name
    else
        echo "Creating new Tigrisdata storage bucket..."
        fly storage create
        echo ""
        echo -e "${YELLOW}IMPORTANT: Note the BUCKET_NAME from the output above!${NC}"
        read -p "Enter the BUCKET_NAME from above: " bucket_name
    fi

    # Set the bucket name secret
    echo "Setting LANCEDB_S3_BUCKET secret..."
    fly secrets set LANCEDB_S3_BUCKET="$bucket_name"
    echo -e "${GREEN}✓ S3 storage configured${NC}"
else
    echo -e "${YELLOW}[Step 5/7] Skipping S3 storage (using local)...${NC}"
    echo -e "${GREEN}✓ Skipped (local storage selected)${NC}"
fi
echo ""

# -----------------------------------------------------------------------------
# Step 6: Set required secrets
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Step 6/7] Setting required secrets...${NC}"
echo ""

# OpenRouter API Key
echo "OpenRouter API Key is required for LLM and embeddings."
echo "Get your key at: https://openrouter.ai/keys"
read -p "Enter your OPENROUTER_API_KEY (sk-or-v1-...): " openrouter_key

if [[ -n "$openrouter_key" ]]; then
    fly secrets set OPENROUTER_API_KEY="$openrouter_key"
    echo -e "${GREEN}✓ OPENROUTER_API_KEY set${NC}"
else
    echo -e "${RED}Warning: OPENROUTER_API_KEY not set. You must set it before deployment.${NC}"
fi
echo ""

# Encryption Master Key
echo "ENCRYPTION_MASTER_KEY is used for tenant API key encryption."
echo "You can generate one with: uuidgen"
read -p "Enter your ENCRYPTION_MASTER_KEY: " encryption_key

if [[ -n "$encryption_key" ]]; then
    fly secrets set ENCRYPTION_MASTER_KEY="$encryption_key"
    echo -e "${GREEN}✓ ENCRYPTION_MASTER_KEY set${NC}"
else
    echo -e "${RED}Warning: ENCRYPTION_MASTER_KEY not set. You must set it before deployment.${NC}"
fi
echo ""

# Show all secrets
echo "Current secrets:"
fly secrets list
echo ""

# -----------------------------------------------------------------------------
# Step 7: Create volume for SQLite
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[Step 7/7] Creating fly volume for SQLite database...${NC}"

# Check if volume already exists
if fly volumes list 2>/dev/null | grep -q "memos_data"; then
    echo -e "${GREEN}✓ Volume 'memos_data' already exists${NC}"
else
    echo "Creating 1GB volume in sjc region..."
    fly volumes create memos_data --size 1 --region sjc
    echo -e "${GREEN}✓ Volume created${NC}"
fi
echo ""

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}  Setup Complete!${NC}"
echo -e "${BLUE}=========================================${NC}"
echo ""
echo -e "${GREEN}Configuration Summary:${NC}"
echo "  Storage Type:  ${STORAGE_TYPE}"
echo "  Dockerfile:    ${DOCKERFILE}"
echo "  fly.toml:      copied from ${FLY_TOML}"
echo ""
echo -e "${GREEN}Next Steps:${NC}"
echo "  1. cd $PROJECT_DIR"
echo "  2. fly deploy"
echo ""
echo -e "${GREEN}Useful Commands:${NC}"
echo "  fly status      - Check app status"
echo "  fly logs        - View application logs"
echo "  fly open        - Open app in browser"
echo "  fly ssh console - SSH into container"
echo ""
echo -e "${YELLOW}Documentation: ${PROJECT_DIR}/docs/DOCS_DEPLOYMENT.MD${NC}"
