#!/bin/bash
# Wrapper script to find and execute the hermetic protoc binary

set -euo pipefail

# Find the protoc binary in the runfiles
RUNFILES_DIR="${RUNFILES_DIR:-$0.runfiles}"
WORKSPACE_NAME="${BUILD_WORKSPACE_DIRECTORY##*/}"

# Try to find protoc in runfiles
PROTOC_PATH=""
if [[ -f "${RUNFILES_DIR}/_main/external/toolchains_protoc++protoc+toolchains_protoc_hub.osx_aarch_64/bin/protoc" ]]; then
    PROTOC_PATH="${RUNFILES_DIR}/_main/external/toolchains_protoc++protoc+toolchains_protoc_hub.osx_aarch_64/bin/protoc"
elif [[ -f "${RUNFILES_DIR}/_main/external/toolchains_protoc++protoc+toolchains_protoc_hub.linux_x86_64/bin/protoc" ]]; then
    PROTOC_PATH="${RUNFILES_DIR}/_main/external/toolchains_protoc++protoc+toolchains_protoc_hub.linux_x86_64/bin/protoc"
fi

if [[ -n "$PROTOC_PATH" ]]; then
    exec "$PROTOC_PATH" "$@"
else
    echo "Error: Could not find hermetic protoc binary" >&2
    exit 1
fi