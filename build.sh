#!/usr/bin/env bash
# Cross-platform build script for StackGet.
# Outputs all binaries into ./dist/

set -e

export PATH="/c/Program Files/Go/bin:$PATH"

mkdir -p dist

echo "==> go mod tidy..."
go mod tidy

echo "==> Building for all platforms..."

GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/stackget-windows-x64.exe .
echo "    windows/x64    -> dist/stackget-windows-x64.exe"

GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o dist/stackget-windows-arm64.exe .
echo "    windows/arm64  -> dist/stackget-windows-arm64.exe"

GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/stackget-macos-x64 .
echo "    macos/x64      -> dist/stackget-macos-x64"

GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/stackget-macos-arm64 .
echo "    macos/arm64    -> dist/stackget-macos-arm64"

GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/stackget-linux-x64 .
echo "    linux/x64      -> dist/stackget-linux-x64"

GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/stackget-linux-arm64 .
echo "    linux/arm64    -> dist/stackget-linux-arm64"

echo ""
echo "Build complete! Binaries are in ./dist/"
