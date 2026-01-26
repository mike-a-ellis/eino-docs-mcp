#!/bin/sh
# Supervisor script to run both Qdrant and MCP server in the same container

set -e

# Trap SIGTERM and SIGINT to gracefully shutdown
trap 'echo "Shutting down..."; kill $QDRANT_PID; exit 0' TERM INT

# Start Qdrant in the background with explicit storage path
# Uses /qdrant/storage which is mounted as a Fly.io volume for persistence
echo "Starting Qdrant server..."
export QDRANT__STORAGE__STORAGE_PATH=/qdrant/storage
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

# Start MCP server (health endpoint only - stdio server will exit immediately)
echo "Starting MCP server health endpoint..."
/app/mcp-server &

# Give it a moment to start the health endpoint
sleep 2

# Keep container alive - wait for Qdrant process
echo "Services running, waiting..."
wait $QDRANT_PID
