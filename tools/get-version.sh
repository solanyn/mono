#!/bin/bash
set -euo pipefail

# Return the current release version in YYYY.MM.N format.
latest_tag=$(git tag --list 'v20*' --sort=-version:refname | head -n1 || true)
if [[ -n "$latest_tag" ]]; then
  echo "${latest_tag#v}"
else
  echo "0.0.0"
fi
