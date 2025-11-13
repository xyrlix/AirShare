#!/usr/bin/env bash
set -e
docker buildx create --use || true
docker buildx build --platform linux/amd64,linux/arm64 -t airshare:latest --push .
