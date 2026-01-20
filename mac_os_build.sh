#!/bin/bash
set -e

OUTPUT="lsm-linux"
echo "Building Linux (amd64) executable: $OUTPUT ..."

# CGO_ENABLED=0: Static binary (no C dependency)
# GOOS=linux: Target OS
# GOARCH=amd64: Target Architecture (modify to arm64 for Raspberry Pi)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$OUTPUT" main.go

echo "Success! Binary created at: $(pwd)/$OUTPUT"
echo "You can now upload it:"
