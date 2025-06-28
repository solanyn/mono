#!/bin/bash
set -euo pipefail

# Usage: ./tools/containers/build.sh <container> [target] [extra-args]
# Examples:
#   ./tools/containers/build.sh airflow                    # Local build
#   ./tools/containers/build.sh airflow image-all "--set *.platform=linux/amd64,linux/arm64"  # CI build

CONTAINER="$1"
TARGET="${2:-image-local}"
EXTRA_ARGS="${3:-}"

# Check if container directory exists and has docker-bake.hcl
CONTAINER_DIR="containers/$CONTAINER"
if [[ ! -d "$CONTAINER_DIR" ]]; then
    echo "Error: Container directory '$CONTAINER_DIR' not found"
    exit 1
fi

if [[ ! -f "$CONTAINER_DIR/docker-bake.hcl" ]]; then
    echo "Skipping '$CONTAINER' - no docker-bake.hcl found"
    exit 0
fi

# Find the workspace root by looking for MODULE.bazel
WORKSPACE_ROOT="$(pwd)"
while [[ ! -f "$WORKSPACE_ROOT/MODULE.bazel" ]] && [[ "$WORKSPACE_ROOT" != "/" ]]; do
    WORKSPACE_ROOT="$(dirname "$WORKSPACE_ROOT")"
done

if [[ ! -f "$WORKSPACE_ROOT/MODULE.bazel" ]]; then
    echo "Error: Could not find workspace root (MODULE.bazel not found)"
    exit 1
fi

echo "Building container: $CONTAINER"
cd "$WORKSPACE_ROOT/containers/$CONTAINER"

if [[ "$TARGET" == "image-all" && -n "$EXTRA_ARGS" ]]; then
    # CI build with caching and multi-platform
    echo "CI build with target: $TARGET"
    docker buildx bake "$TARGET" $EXTRA_ARGS
else
    # Local build
    echo "Local build"
    docker buildx bake --load
fi