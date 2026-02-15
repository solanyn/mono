#!/usr/bin/env bash
set -euo pipefail

BUCKET="${R2_BUCKET:-assets}"
SOURCE_DIR="${1:-website/assets}"

if ! command -v wrangler &> /dev/null; then
  echo "error: wrangler not found. Run: bazelisk run //:wrangler" >&2
  exit 1
fi

if [ ! -d "$SOURCE_DIR" ]; then
  echo "error: $SOURCE_DIR does not exist" >&2
  exit 1
fi

find "$SOURCE_DIR" -type f \( -name "*.jpg" -o -name "*.jpeg" -o -name "*.png" -o -name "*.webp" -o -name "*.gif" -o -name "*.svg" \) | while read -r file; do
  key="${file#"$SOURCE_DIR"/}"
  echo "uploading: $key"
  wrangler r2 object put "$BUCKET/$key" --file "$file"
done

echo "done"
