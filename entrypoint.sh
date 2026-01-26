#!/bin/sh
# Entrypoint script to handle different process types in Fly.io

# The first argument determines which process to run
PROCESS_TYPE="${1:-web}"

case "$PROCESS_TYPE" in
  "/qdrant/qdrant")
    echo "Starting Qdrant server..."
    exec /qdrant/qdrant
    ;;
  "/app/mcp-server"|"web")
    echo "Starting MCP server..."
    exec /app/mcp-server
    ;;
  *)
    echo "Unknown process type: $PROCESS_TYPE"
    echo "Usage: $0 [/qdrant/qdrant|/app/mcp-server]"
    exit 1
    ;;
esac
