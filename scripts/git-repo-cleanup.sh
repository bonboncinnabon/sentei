#!/bin/bash
if command -v sentei >/dev/null 2>&1; then
    exec sentei cleanup "$@"
else
    echo "sentei not found. Install: go install github.com/bonboncinnabon/sentei@latest" >&2
    exit 1
fi
