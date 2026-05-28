#!/bin/bash
# =============================================================================
# Pre-deployment environment chain validation
# Validates: .env → Dockerfile → fly.toml → fly secrets
# =============================================================================

set -e

# Configuration
SENSITIVE_VARS="OPENROUTER_API_KEY ENCRYPTION_MASTER_KEY"
LOCAL_ONLY_VARS="LANCEDB_LOCAL_PATH"
S3_ONLY_VARS="LANCEDB_S3_ENDPOINT LANCEDB_S3_REGION LANCEDB_S3_BUCKET"
SKIP_IN_ENV="MEMOS_MODE MEMOS_PORT TZ LLM_VERIFIER_ENABLED"  # Runtime-only, not in .env
BUILD_ONLY_VARS="CGO_ENABLED CGO_CFLAGS CGO_LDFLAGS"  # Build-time only, not runtime

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

echo "=== Pre-Deployment Environment Chain Check ==="
echo ""

# -----------------------------------------------------------------------------
# Step 1: Read .env vars (source of truth)
# -----------------------------------------------------------------------------
echo "Step 1: Reading .env (source of truth)..."
if [ ! -f .env ]; then
    echo -e "${RED}ERROR: .env file not found${NC}"
    exit 1
fi

declare -A ENV_VARS
while IFS='=' read -r key value; do
    # Skip comments and empty lines
    [[ "$key" =~ ^#.*$ ]] && continue
    [[ -z "$key" ]] && continue
    # Trim whitespace
    key=$(echo "$key" | xargs)
    # Remove quotes from value
    value=$(echo "$value" | xargs | sed 's/^"//' | sed 's/"$//')
    ENV_VARS["$key"]="$value"
done < .env
echo "   Found ${#ENV_VARS[@]} variables"

# -----------------------------------------------------------------------------
# Step 2: Determine which Dockerfile to check from fly.toml
# -----------------------------------------------------------------------------
echo ""
echo "Step 2: Determining Dockerfile from fly.toml..."
if [ ! -f fly.toml ]; then
    echo -e "${RED}ERROR: fly.toml not found${NC}"
    exit 1
fi

DOCKERFILE=$(grep -E '^\s*dockerfile\s*=' fly.toml | head -1 | sed -E "s/^\s*dockerfile\s*=\s*['\"]//; s/['\"]\s*$//")
if [ -z "$DOCKERFILE" ]; then
    echo -e "${RED}ERROR: No dockerfile specified in fly.toml [build]${NC}"
    exit 1
fi
echo "   Using: $DOCKERFILE"

# Determine storage type from Dockerfile name
STORAGE_TYPE="local"
if [[ "$DOCKERFILE" == *"s3"* ]]; then
    STORAGE_TYPE="s3"
fi
echo "   Storage type: $STORAGE_TYPE"

# -----------------------------------------------------------------------------
# Step 3: Read Dockerfile ENV declarations
# -----------------------------------------------------------------------------
echo ""
echo "Step 3: Reading $DOCKERFILE ENV declarations..."
if [ ! -f "$DOCKERFILE" ]; then
    echo -e "${RED}ERROR: $DOCKERFILE not found${NC}"
    exit 1
fi

declare -A DOCKER_VARS
while IFS= read -r line; do
    # Match ENV KEY="value" or ENV KEY=value patterns
    if [[ "$line" =~ ^ENV[[:space:]]+([A-Z_][A-Z0-9_]*)=\"([^\"]*)\" ]]; then
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
        DOCKER_VARS["$key"]="$value"
    elif [[ "$line" =~ ^ENV[[:space:]]+([A-Z_][A-Z0-9_]*)=([^[:space:]]*) ]]; then
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
        DOCKER_VARS["$key"]="$value"
    fi
done < "$DOCKERFILE"
echo "   Found ${#DOCKER_VARS[@]} ENV declarations"

# -----------------------------------------------------------------------------
# Step 4: Read fly.toml [env] vars
# -----------------------------------------------------------------------------
echo ""
echo "Step 4: Reading fly.toml [env] section..."
declare -A TOML_VARS
IN_ENV_SECTION=0
while IFS= read -r line; do
    if [[ "$line" =~ ^\[env\] ]]; then
        IN_ENV_SECTION=1
        continue
    fi
    # Stop when we hit another section
    if [[ "$line" =~ ^\[.+\] ]] && [ $IN_ENV_SECTION -eq 1 ]; then
        break
    fi
    if [ $IN_ENV_SECTION -eq 1 ]; then
        # Match KEY = "value" or KEY = 'value' pattern (TOML style)
        if [[ "$line" =~ ^[[:space:]]*([A-Z_][A-Z0-9_]*)[[:space:]]*=[[:space:]]*[\'\"](.*)[\'\"] ]]; then
            key="${BASH_REMATCH[1]}"
            value="${BASH_REMATCH[2]}"
            TOML_VARS["$key"]="$value"
        fi
    fi
done < fly.toml
echo "   Found ${#TOML_VARS[@]} variables"

# -----------------------------------------------------------------------------
# Step 5: Get fly secrets
# -----------------------------------------------------------------------------
echo ""
echo "Step 5: Reading fly secrets..."
declare -A FLY_SECRETS
if command -v fly &> /dev/null; then
    while IFS= read -r secret; do
        [ -n "$secret" ] && FLY_SECRETS["$secret"]=1
    done < <(fly secrets list 2>/dev/null | tail -n +2 | awk '{print $1}')
    echo "   Found ${#FLY_SECRETS[@]} secrets"
else
    echo -e "   ${YELLOW}(flyctl not installed - skipping secrets check)${NC}"
fi

# -----------------------------------------------------------------------------
# Step 6: Validate chain
# -----------------------------------------------------------------------------
echo ""
echo "=== Validation Results ==="

# -----------------------------------------------------------------------------
# Check 6a: Non-sensitive .env vars should be in Dockerfile AND fly.toml with matching values
# -----------------------------------------------------------------------------
echo ""
echo "Check: .env -> Dockerfile + fly.toml (non-sensitive vars)"
ENV_CHECK_ERRORS=0
for key in "${!ENV_VARS[@]}"; do
    env_val="${ENV_VARS[$key]}"

    # Skip sensitive vars (should only be in fly secrets)
    if [[ " $SENSITIVE_VARS " =~ " $key " ]]; then
        continue
    fi
    # Skip context-inappropriate storage vars
    if [ "$STORAGE_TYPE" = "local" ] && [[ " $S3_ONLY_VARS " =~ " $key " ]]; then
        continue
    fi
    if [ "$STORAGE_TYPE" = "s3" ] && [[ " $LOCAL_ONLY_VARS " =~ " $key " ]]; then
        continue
    fi

    # Check Dockerfile
    if [ -z "${DOCKER_VARS[$key]+x}" ]; then
        echo -e "  ${RED}MISSING in Dockerfile: $key${NC}"
        ERRORS=$((ERRORS + 1))
        ENV_CHECK_ERRORS=$((ENV_CHECK_ERRORS + 1))
    elif [ "${DOCKER_VARS[$key]}" != "$env_val" ]; then
        echo -e "  ${RED}VALUE MISMATCH (.env vs Dockerfile): $key${NC}"
        echo "    .env:       $env_val"
        echo "    Dockerfile: ${DOCKER_VARS[$key]}"
        ERRORS=$((ERRORS + 1))
        ENV_CHECK_ERRORS=$((ENV_CHECK_ERRORS + 1))
    fi

    # Check fly.toml
    if [ -z "${TOML_VARS[$key]+x}" ]; then
        echo -e "  ${RED}MISSING in fly.toml: $key${NC}"
        ERRORS=$((ERRORS + 1))
        ENV_CHECK_ERRORS=$((ENV_CHECK_ERRORS + 1))
    elif [ "${TOML_VARS[$key]}" != "$env_val" ]; then
        echo -e "  ${RED}VALUE MISMATCH (.env vs fly.toml): $key${NC}"
        echo "    .env:     $env_val"
        echo "    fly.toml: ${TOML_VARS[$key]}"
        ERRORS=$((ERRORS + 1))
        ENV_CHECK_ERRORS=$((ENV_CHECK_ERRORS + 1))
    fi
done
[ $ENV_CHECK_ERRORS -eq 0 ] && echo -e "  ${GREEN}All .env vars properly reflected in Dockerfile and fly.toml${NC}"

# -----------------------------------------------------------------------------
# Check 6b: Sensitive vars must be in fly secrets
# -----------------------------------------------------------------------------
echo ""
echo "Check: Sensitive vars in fly secrets"
if [ ${#FLY_SECRETS[@]} -gt 0 ]; then
    for var in $SENSITIVE_VARS; do
        if [ -n "${ENV_VARS[$var]+x}" ]; then
            if [ -z "${FLY_SECRETS[$var]+x}" ]; then
                echo -e "  ${RED}MISSING in fly secrets: $var${NC}"
                ERRORS=$((ERRORS + 1))
            else
                echo -e "  ${GREEN}OK: $var in fly secrets${NC}"
            fi
        fi
    done
else
    echo -e "  ${YELLOW}Skipped (flyctl not installed)${NC}"
fi

# -----------------------------------------------------------------------------
# Check 6c: Sensitive vars must NOT be in Dockerfile or fly.toml (security)
# -----------------------------------------------------------------------------
echo ""
echo "Check: Sensitive vars NOT exposed (security)"
SECURITY_OK=1
for var in $SENSITIVE_VARS; do
    if [ -n "${DOCKER_VARS[$var]+x}" ]; then
        echo -e "  ${RED}SECURITY: $var exposed in Dockerfile!${NC}"
        ERRORS=$((ERRORS + 1))
        SECURITY_OK=0
    fi
    if [ -n "${TOML_VARS[$var]+x}" ]; then
        echo -e "  ${RED}SECURITY: $var exposed in fly.toml!${NC}"
        ERRORS=$((ERRORS + 1))
        SECURITY_OK=0
    fi
done
[ $SECURITY_OK -eq 1 ] && echo -e "  ${GREEN}No sensitive vars exposed${NC}"

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
echo "=== Summary ==="
if [ $ERRORS -gt 0 ]; then
    echo -e "${RED}$ERRORS error(s), $WARNINGS warning(s)${NC}"
    echo -e "${RED}Fix errors before deploying!${NC}"
    exit 1
elif [ $WARNINGS -gt 0 ]; then
    echo -e "${YELLOW}$WARNINGS warning(s) - review before deploying${NC}"
    exit 0
else
    echo -e "${GREEN}All checks passed!${NC}"
    exit 0
fi
