#!/bin/bash

# Build verification script for Bazel migration
# This script should be run from the repository root with: ./scripts/test-build.sh

set -e

echo "🏗️  Testing Bazel build targets..."
echo ""

echo "✅ Building shell script tools..."
bazel build //tools:get-version

echo "✅ Building Go backend server..."
bazel build //tldr/backend/cmd/server:server

echo "✅ Testing Bazel query..."
bazel query "//tools/... + //tldr/backend/..." > /dev/null

echo ""
echo "🎉 All working targets built successfully!"
echo ""
echo "📋 Summary:"
echo "   ✅ Shell scripts: //tools:get-version"
echo "   ✅ Go backend: //tldr/backend/cmd/server:server"
echo "   ⏳ TypeScript/React: Not yet implemented"
echo "   ⏳ Python: Not yet implemented"  
echo "   ⏳ Rust: Not yet implemented"
echo "   ⏳ Protocol buffers: Not yet implemented"