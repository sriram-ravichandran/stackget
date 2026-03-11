#!/usr/bin/env bash
# Quick build helper for StackGet.
# Run this once after cloning / creating the project.

set -e

echo "==> go mod tidy  (downloads dependencies)..."
go mod tidy

echo "==> go build     (compiles binary)..."
go build -ldflags="-s -w" -o stackget.exe .

echo ""
echo "Build complete!  Try:"
echo "  ./stackget.exe scan"
echo "  ./stackget.exe scan --installed"
echo "  ./stackget.exe scan --only languages"
echo "  ./stackget.exe scan --json"
echo "  ./stackget.exe generate"
echo "  ./stackget.exe check stackget.yaml"
echo "  ./stackget.exe diff env1.yaml env2.yaml"
