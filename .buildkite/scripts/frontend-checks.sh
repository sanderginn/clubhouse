#!/usr/bin/env bash
set -euo pipefail

if ! command -v git >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y git
fi

cd frontend

npm ci
npm run lint -- --resolve-plugins-relative-to .
git diff --exit-code
npm run check
npm run test
