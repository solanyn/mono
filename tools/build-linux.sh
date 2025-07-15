#!/bin/bash
# Build targets using Linux container for dbx_build_tools compatibility
# Usage: ./tools/build-linux.sh [bazel_command] [target] [additional_args...]

set -euo pipefail

# Default to build if no command provided
BAZEL_CMD="${1:-build}"
shift 2>/dev/null || true

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not available. Please install Docker or use remote execution:"
    echo "  bazelisk build --config=dev:remote //target:name"
    exit 1
fi

# Use Debian slim with bazelisk
BAZEL_IMAGE="debian:bookworm-slim"

echo "Running Bazel in Linux container..."
echo "Command: bazelisk ${BAZEL_CMD} $*"

# Mount workspace and run Bazel command with bazelisk installation
docker run --rm \
    -v "$(pwd):/workspace" \
    -w /workspace \
    --platform linux/amd64 \
    "${BAZEL_IMAGE}" \
    bash -c "
        # Install bazelisk with minimal dependencies
        if ! command -v bazelisk &> /dev/null; then
            apt-get update -qq
            apt-get install -y --no-install-recommends curl ca-certificates
            curl -LO https://github.com/bazelbuild/bazelisk/releases/latest/download/bazelisk-linux-amd64
            chmod +x bazelisk-linux-amd64
            mv bazelisk-linux-amd64 /usr/local/bin/bazelisk
        fi
        # Run the bazelisk command with LLVM toolchain
        bazelisk ${BAZEL_CMD} --extra_toolchains=@llvm_toolchain//:cc-toolchain-x86_64-linux $*
    "