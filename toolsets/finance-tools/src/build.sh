#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${SAM_TOOL_BUILD_OUT:-dist}"
NAME="${SAM_TOOL_NAME:-finance-tools}"
: "${SAM_TOOL_TARGET_OS:?must be set by sam config apply, or exported for a manual build (e.g. linux)}"
: "${SAM_TOOL_TARGET_ARCH:?must be set by sam config apply, or exported for a manual build (e.g. amd64)}"
TARGET_OS="$SAM_TOOL_TARGET_OS"
TARGET_ARCH="$SAM_TOOL_TARGET_ARCH"

mkdir -p "$OUT_DIR"

CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" \
  go build -o "$OUT_DIR/$NAME" .

cp manifest.yaml "$OUT_DIR/manifest.yaml"
