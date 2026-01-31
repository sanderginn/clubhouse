#!/usr/bin/env bash
set -euo pipefail

cd backend

if command -v git >/dev/null 2>&1; then
  git ls-files '*.go' | xargs -r gofmt -w
  git diff --exit-code
else
  echo "git not available; skipping gofmt diff check."
  find . -name '*.go' -print0 | xargs -0 -r gofmt -w
fi

export GOBIN=/tmp/bin
export PATH="${GOBIN}:${PATH}"

go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

golangci-lint run ./...
