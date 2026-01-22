#!/usr/bin/env bash
set -euo pipefail

cd frontend

npm ci
npm run lint -- --resolve-plugins-relative-to .
git diff --exit-code
npm run check
npm run test
