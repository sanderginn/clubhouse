#!/usr/bin/env bash
set -euo pipefail

cd backend

git ls-files '*.go' | xargs -r gofmt -w
git diff --exit-code

export GOBIN=/tmp/bin
export PATH="${GOBIN}:${PATH}"

go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

golangci-lint run ./...
