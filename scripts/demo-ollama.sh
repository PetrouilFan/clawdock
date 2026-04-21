#!/usr/bin/env bash

# Demo script to configure a remote Ollama provider
# Usage: ./scripts/demo-ollama.sh https://mainframeollama.petrouil.com

set -e

BASE_URL="${1:-https://mainframeollama.petrouil.com}"
API_KEY="${2:-}"

echo "=== Clawdock ModelRouter Demo Setup ==="
echo "Configuring Ollama provider at: $BASE_URL"

# Check if server is running
if ! curl -s http://localhost:11436/healthz > /dev/null; then
  echo "Error: Clawdock server does not appear to be running on http://localhost:11436"
  echo "Start the server first: go run ./cmd/server --config /etc/openclaw-manager/config.yaml"
  exit 1
fi

# Create provider
echo "Creating provider 'Remote Ollama'..."
PROVIDER_DATA='{
  "display_name": "Remote Ollama",
  "base_url": "'"$BASE_URL"'",
  "auth_type": "none",
  "enabled": true,
  "supports_model_discovery": true
}'

RESPONSE=$(curl -s -X POST http://localhost:11436/api/providers \
  -H "Content-Type: application/json" \
  -d "$PROVIDER_DATA")

echo "Provider created: $RESPONSE"

# Extract provider ID (should be auto-generated slug based on display name? Actually ID is UUID)
# For demo, we'll list providers to get the ID
echo "Fetching provider list..."
PROVIDERS=$(curl -s http://localhost:11436/api/providers)
echo "Available providers: $PROVIDERS"

# Find the provider ID (simple grep)
PROVIDER_ID=$(echo "$PROVIDERS" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$PROVIDER_ID" ]; then
  echo "Could not determine provider ID. Exiting."
  exit 1
fi

echo "Found provider ID: $PROVIDER_ID"

# Refresh models
echo "Refreshing models from provider..."
REFRESH=$(curl -s -X POST http://localhost:11436/api/providers/$PROVIDER_ID/refresh-models)
echo "Refresh result: $REFRESH"

# Set as default model (optional)
DEFAULT_MODEL="llama3"
echo "Setting default model to: $DEFAULT_MODEL"
curl -s -X PUT http://localhost:11436/api/settings/default_model \
  -H "Content-Type: application/json" \
  -d "{\"default_model\":\"$DEFAULT_MODEL\"}"

echo ""
echo "=== Demo Setup Complete ==="
echo "1. Provider 'Remote Ollama' created"
echo "2. Models discovered and enabled"
echo "3. Default model set to '$DEFAULT_MODEL'"
echo ""
echo "Next steps:"
echo "- Open http://localhost:11436 in your browser"
echo "- Go to Providers to see discovered models"
echo "- Create an agent using the provider"
echo "- Try chat proxy: curl -X POST http://localhost:11436/v1/chat/completions -H 'Content-Type: application/json' -d '{\"model\":\"llama3\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}]}'"
