#!/bin/sh
set -e

echo "bonsai: running pre-commit checks..."
docker compose run --rm dev sh -c "go vet ./... && golangci-lint run ./..."
echo "bonsai: all checks passed"
