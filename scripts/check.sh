#!/usr/bin/env bash
# Go vet + tests, then the frontend's typecheck, lint and tests.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/go/bin:$PATH"

# go:embed needs frontend/dist to exist; the bindings are generated, not committed.
mkdir -p frontend/dist && [ -n "$(ls -A frontend/dist)" ] || touch frontend/dist/.keep
wails3 generate bindings -clean=true -ts -i >/dev/null
go vet . ./internal/...
go test . ./internal/...
npm --prefix frontend run check
