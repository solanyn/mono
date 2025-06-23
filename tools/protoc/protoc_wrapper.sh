#!/bin/bash
# Hermetic protoc wrapper for Rust build scripts

set -euo pipefail

# Find the hermetic protoc binary
SCRIPT_DIR=$(dirname "$(readlink -f "$0" 2>/dev/null || echo "$0")")
PROTOC_BIN="$SCRIPT_DIR/protoc_wrapper.runfiles/_main/tools/protoc/protoc"

# Try alternative runfiles path
if [[ ! -f "$PROTOC_BIN" ]]; then
    PROTOC_BIN="$SCRIPT_DIR/protoc"
fi

# Fallback: look for protoc in same directory
if [[ ! -f "$PROTOC_BIN" ]]; then
    PROTOC_BIN="$SCRIPT_DIR/../protoc/protoc"
fi

if [[ ! -f "$PROTOC_BIN" ]]; then
    echo "Error: Could not find hermetic protoc binary at $PROTOC_BIN" >&2
    exit 1
fi

# Execute the hermetic protoc
exec "$PROTOC_BIN" "$@"