#!/bin/bash
# Hermetic protoc wrapper for Rust build scripts
# This script forwards all arguments to the hermetic protoc binary

set -euo pipefail

# Find the protoc binary relative to this script
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")
PROTOC_BIN="$SCRIPT_DIR/protoc_wrapper.runfiles/_main/external/protobuf+/protoc"

# If runfiles path doesn't exist, try alternative paths
if [[ ! -f "$PROTOC_BIN" ]]; then
    # Try direct runfiles path
    PROTOC_BIN="$SCRIPT_DIR/protoc_wrapper.runfiles/protobuf+/protoc"
fi

if [[ ! -f "$PROTOC_BIN" ]]; then
    # Try finding it in the data dependencies
    for candidate in "$SCRIPT_DIR"/../../../*/external/protobuf+/protoc; do
        if [[ -f "$candidate" ]]; then
            PROTOC_BIN="$candidate"
            break
        fi
    done
fi

if [[ ! -f "$PROTOC_BIN" ]]; then
    echo "Error: Could not find hermetic protoc binary" >&2
    exit 1
fi

# Execute the hermetic protoc with all provided arguments
exec "$PROTOC_BIN" "$@"