#!/bin/bash
set -e

OUTPUT="lsm-linux"
echo "Building Linux executable (Standard)..."
# Native build (assuming running on Linux)
go build -o "$OUTPUT" main.go

echo "Success! Binary created at: $(pwd)/$OUTPUT"
