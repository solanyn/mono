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

echo "✅ Building protocol buffers..."
bazel build //tldr/proto:news_proto_go

echo "✅ Building Python airflow..."
bazel build //airflow:bps

echo "✅ Testing Bazel query..."
bazel query "//tools/... + //tldr/backend/... + //tldr/proto/... + //airflow/..." > /dev/null

echo ""
echo "🎉 All working targets built successfully!"
echo ""
echo "📋 Summary:"
echo "   ✅ Shell scripts: //tools:get-version"
echo "   ✅ Go backend: //tldr/backend/cmd/server:server"
echo "   ✅ Protocol buffers: //tldr/proto:news_proto_go"
echo "   ✅ Python: //airflow:bps"
echo "   ⏳ TypeScript/React: Not yet implemented (shell toolchain issues)"
echo "   ⏳ Rust: Not yet implemented (edition2024 compatibility issues)"