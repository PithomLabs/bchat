#!/bin/bash
# setup.sh - CockroachDB setup script for hackathon demo
# NOTE: cluster creation is console-first (see bugs/057/pre_code.md §5):
# multi-region Basic clusters must be created in the Cloud Console
# (https://cockroachlabs.cloud) — `ccloud cluster create basic` accepts
# exactly ONE region. This script handles user / connection URL / allowlist
# for an existing cluster.

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-hackathon-demo}"
USER_NAME="${USER_NAME:-hackathon-user}"

echo "=== CockroachDB Setup Script ==="
echo "Cluster: $CLUSTER_NAME"
echo "User: $USER_NAME"
echo ""

# Step 1: Verify cluster exists (console-created; 2-region Basic)
echo "1. Checking cluster $CLUSTER_NAME..."
if ! ccloud cluster list 2>/dev/null | grep -q "$CLUSTER_NAME"; then
    echo "   Cluster '$CLUSTER_NAME' not found."
    echo "   Create it in the Cloud Console first (Create cluster -> Basic ->"
    echo "   AWS, 2 regions: us-east-1 primary + us-west-2)."
    echo "   NOTE: ccloud cluster create basic supports only ONE region."
    exit 1
fi
echo "   Cluster found"

# Step 2: Create user
echo "2. Creating database user..."
if ccloud cluster user create "$CLUSTER_NAME" "$USER_NAME"; then
    echo "   User created successfully"
else
    echo "   User creation failed or already exists"
fi

# Step 3: Allowlist (Basic ships with 0.0.0.0/0; keep unless hardened via task crdb:harden)
echo "3. Checking SQL allowlist..."
if ! ccloud cluster networking allowlist list "$CLUSTER_NAME" 2>/dev/null | grep -q "0.0.0.0/0"; then
    ccloud cluster networking allowlist create "$CLUSTER_NAME" 0.0.0.0/0 --sql --ui --name all || true
    echo "   Allowlist 0.0.0.0/0 added"
else
    echo "   Allowlist 0.0.0.0/0 present"
fi

# Step 4: Get connection string
echo "4. Getting connection string..."
CONNECTION_URL=$(ccloud cluster sql --connection-url "$CLUSTER_NAME" 2>/dev/null || echo "")
if [ -n "$CONNECTION_URL" ]; then
    echo "   Connection URL: $CONNECTION_URL"
    echo ""
    echo "=== Setup Complete ==="
    echo "Set COCKROACH_DSN to: $CONNECTION_URL"
    echo ""
    echo "Next steps:"
    echo "1. Set environment variables:"
    echo "   export COCKROACH_DSN=\"$CONNECTION_URL\""
    echo "   export LANCEDB_STORAGE_PROVIDER=cockroach"
    echo "   export RAG_PIPELINE_ENABLED=true"
    echo "   export EMBEDDING_PROVIDER=openrouter"
    echo "   export OPENROUTER_API_KEY=your-api-key"
    echo ""
    echo "2. Enable vector index (v25.x clusters only; no-op on v26+):"
    echo "   cockroach sql --url \"$CONNECTION_URL\" \\"
    echo "     -e \"SET CLUSTER SETTING feature.vector_index.enabled = true;\""
    echo ""
    echo "3. Run the application:"
    echo "   task run:cockroach"
else
    echo "   Failed to get connection URL"
    echo "   Try running: ccloud cluster sql --connection-url $CLUSTER_NAME"
fi
