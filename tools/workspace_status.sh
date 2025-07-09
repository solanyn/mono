#!/bin/bash
set -euo pipefail

# Output stable status (causes rebuilds when changed)
echo "STABLE_VERSION $(./tools/get-version.sh .)"
echo "STABLE_GIT_COMMIT $(git rev-parse HEAD)"

# Output volatile status (doesn't cause rebuilds)
echo "BUILD_TIMESTAMP $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "BUILD_USER $(whoami)"