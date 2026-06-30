#!/bin/bash
# =============================================================================
# Fix environment chain by syncing .env -> Dockerfile + fly.toml
# Companion to validate-env-chain.sh
# =============================================================================

set -e

# Configuration (same as validate script)
SENSITIVE_VARS="OPENROUTER_API_KEY ENCRYPTION_MASTER_KEY"
LOCAL_ONLY_VARS="LANCEDB_LOCAL_PATH"
S3_ONLY_VARS="LANCEDB_S3_ENDPOINT LANCEDB_S3_REGION LANCEDB_S3_BUCKET"
SKIP_IN_ENV="MEMOS_MODE MEMOS_PORT TZ LLM_VERIFIER_ENABLED"  # Runtime-only, not in .env
BUILD_ONLY_VARS="CGO_ENABLED CGO_CFLAGS CGO_LDFLAGS"  # Build-time only, not runtime

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FIXES=0

echo "=== Environment Chain Fix ==="
echo ""

# -----------------------------------------------------------------------------
# Step 1: Read .env vars (source of truth)
# -----------------------------------------------------------------------------
echo "Step 1: Reading .env (source of truth)..."
if [ ! -f .env ]; then
    echo -e "${YELLOW}ERROR: .env file not found${NC}"
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
    echo -e "${YELLOW}ERROR: fly.toml not found${NC}"
    exit 1
fi

DOCKERFILE=$(grep -E '^\s*dockerfile\s*=' fly.toml | head -1 | sed -E "s/^\s*dockerfile\s*=\s*['\"]//; s/['\"]\s*$//")
if [ -z "$DOCKERFILE" ]; then
    echo -e "${YELLOW}ERROR: No dockerfile specified in fly.toml [build]${NC}"
    exit 1
fi
echo "   Using: $DOCKERFILE"

# Determine storage type from Dockerfile name
STORAGE_TYPE="local"
if [[ "$DOCKERFILE" == *"s3"* ]]; then
    STORAGE_TYPE="s3"
fi
echo "   Storage type: $STORAGE_TYPE"

if [ ! -f "$DOCKERFILE" ]; then
    echo -e "${YELLOW}ERROR: $DOCKERFILE not found${NC}"
    exit 1
fi

# -----------------------------------------------------------------------------
# Step 3: Read current Dockerfile ENV vars
# -----------------------------------------------------------------------------
echo ""
echo "Step 3: Reading $DOCKERFILE ENV declarations..."
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
# Step 4: Read current fly.toml [env] vars
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
# Step 5: Fix missing/mismatched vars
# -----------------------------------------------------------------------------
echo ""
echo "=== Applying Fixes ==="

# Find the EXPOSE line number in Dockerfile for insertion point
EXPOSE_LINE=$(grep -n "^EXPOSE" "$DOCKERFILE" | head -1 | cut -d: -f1)
if [ -z "$EXPOSE_LINE" ]; then
    echo -e "${YELLOW}WARNING: No EXPOSE line found in Dockerfile, appending to end${NC}"
    EXPOSE_LINE=$(wc -l < "$DOCKERFILE")
fi

# Find the end of [env] section in fly.toml for insertion point
# We'll find the line before the next section after [env]
ENV_SECTION_END=""
IN_ENV_SECTION=0
LINE_NUM=0
while IFS= read -r line; do
    LINE_NUM=$((LINE_NUM + 1))
    if [[ "$line" =~ ^\[env\] ]]; then
        IN_ENV_SECTION=1
        continue
    fi
    if [[ "$line" =~ ^\[.+\] ]] && [ $IN_ENV_SECTION -eq 1 ]; then
        ENV_SECTION_END=$((LINE_NUM - 1))
        break
    fi
done < fly.toml

# If no section found after [env], use end of file
if [ -z "$ENV_SECTION_END" ]; then
    ENV_SECTION_END=$(wc -l < fly.toml)
fi

for key in "${!ENV_VARS[@]}"; do
    env_val="${ENV_VARS[$key]}"

    # Skip sensitive vars (should only be in fly secrets)
    if [[ " $SENSITIVE_VARS " =~ " $key " ]]; then
        continue
    fi
    # Skip build-only vars (CGO_*)
    if [[ " $BUILD_ONLY_VARS " =~ " $key " ]]; then
        continue
    fi
    # Skip context-inappropriate storage vars
    if [ "$STORAGE_TYPE" = "local" ] && [[ " $S3_ONLY_VARS " =~ " $key " ]]; then
        continue
    fi
    if [ "$STORAGE_TYPE" = "s3" ] && [[ " $LOCAL_ONLY_VARS " =~ " $key " ]]; then
        continue
    fi

    # ----- Fix Dockerfile -----
    if [ -z "${DOCKER_VARS[$key]+x}" ]; then
        # Missing: insert before EXPOSE line
        echo -e "  ${GREEN}ADD${NC} to Dockerfile: $key=\"$env_val\""
        sed -i "${EXPOSE_LINE}i ENV ${key}=\"${env_val}\"" "$DOCKERFILE"
        EXPOSE_LINE=$((EXPOSE_LINE + 1))  # Shift EXPOSE line down
        FIXES=$((FIXES + 1))
    elif [ "${DOCKER_VARS[$key]}" != "$env_val" ]; then
        # Mismatch: update in place
        echo -e "  ${GREEN}UPDATE${NC} Dockerfile: $key=\"${DOCKER_VARS[$key]}\" -> \"$env_val\""
        # Escape special characters for sed
        escaped_old=$(printf '%s\n' "${DOCKER_VARS[$key]}" | sed 's/[&/\]/\\&/g')
        escaped_new=$(printf '%s\n' "$env_val" | sed 's/[&/\]/\\&/g')
        sed -i "s|^ENV ${key}=\"${escaped_old}\"|ENV ${key}=\"${escaped_new}\"|" "$DOCKERFILE"
        FIXES=$((FIXES + 1))
    fi

    # ----- Fix fly.toml -----
    if [ -z "${TOML_VARS[$key]+x}" ]; then
        # Missing: append to [env] section
        echo -e "  ${GREEN}ADD${NC} to fly.toml: $key = \"$env_val\""
        sed -i "${ENV_SECTION_END}a\\  ${key} = \"${env_val}\"" fly.toml
        ENV_SECTION_END=$((ENV_SECTION_END + 1))  # Shift section end down
        FIXES=$((FIXES + 1))
    elif [ "${TOML_VARS[$key]}" != "$env_val" ]; then
        # Mismatch: update in place
        echo -e "  ${GREEN}UPDATE${NC} fly.toml: $key = \"${TOML_VARS[$key]}\" -> \"$env_val\""
        # Escape special characters for sed
        escaped_old=$(printf '%s\n' "${TOML_VARS[$key]}" | sed 's/[&/\]/\\&/g')
        escaped_new=$(printf '%s\n' "$env_val" | sed 's/[&/\]/\\&/g')
        sed -i "s|^[[:space:]]*${key}[[:space:]]*=[[:space:]]*['\"]${escaped_old}['\"]|  ${key} = \"${escaped_new}\"|" fly.toml
        FIXES=$((FIXES + 1))
    fi
done

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
echo "=== Summary ==="
if [ $FIXES -gt 0 ]; then
    echo -e "${GREEN}Applied $FIXES fix(es)${NC}"
    echo ""
    echo "Review changes with:"
    echo "  git diff $DOCKERFILE fly.toml"
    echo ""
    echo "Verify with:"
    echo "  task fly:check"
else
    echo -e "${GREEN}No fixes needed - all in sync!${NC}"
fi
