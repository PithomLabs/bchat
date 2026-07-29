#!/bin/bash
# setup.sh - CockroachDB setup script for hackathon demo
# Uses ccloud CLI to create and configure a CockroachDB cluster

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-hackathon-demo}"
REGION="${REGION:-us-east-1}"
USER_NAME="${USER_NAME:-hackathon-user}"

echo "=== CockroachDB Setup Script ==="
echo "Cluster: $CLUSTER_NAME"
echo "Region: $REGION"
echo "User: $USER_NAME"
echo ""

# Step 1: Create cluster
echo "1. Creating CockroachDB cluster..."
if ccloud cluster create basic "$CLUSTER_NAME" "$REGION" --cloud AWS --spend-limit 0; then
    echo "   Cluster created successfully"
else
    echo "   Cluster creation failed or already exists"
fi

# Step 2: Create user
echo "2. Creating database user..."
if ccloud cluster user create "$CLUSTER_NAME" "$USER_NAME"; then
    echo "   User created successfully"
else
    echo "   User creation failed or already exists"
fi

# Step 3: Get connection string
echo "3. Getting connection string..."
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
    echo "   export VECTOR_DB_PROVIDER=cockroach"
    echo "   export RAG_PIPELINE_ENABLED=true"
    echo "   export EMBEDDING_PROVIDER=openrouter"
    echo "   export OPENROUTER_API_KEY=your-api-key"
    echo ""
    echo "2. Run the application:"
    echo "   task run:cockroach"
else
    echo "   Failed to get connection URL"
    echo "   Try running: ccloud cluster sql --connection-url $CLUSTER_NAME"
fi
