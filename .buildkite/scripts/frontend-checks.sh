#!/usr/bin/env bash
set -euo pipefail

cd frontend

npm ci
npm run lint -- --resolve-plugins-relative-to .
if command -v git >/dev/null 2>&1; then
  git diff --exit-code
else
  echo "git not available; skipping diff check."
fi
npm run check
npm run test
