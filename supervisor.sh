#!/bin/sh
# Supervisor script to run both Qdrant and MCP server in the same container

set -e

# Start Qdrant in the background
echo "Starting Qdrant server..."
/qdrant/qdrant &
QDRANT_PID=$!

# Wait for Qdrant to be ready
echo "Waiting for Qdrant to start..."
for i in $(seq 1 30); do
  if nc -z localhost 6334 2>/dev/null; then
    echo "Qdrant is ready"
    break
  fi
  echo "Waiting for Qdrant... (attempt $i/30)"
  sleep 1
done

# Start MCP server in foreground
echo "Starting MCP server..."
exec /app/mcp-server
