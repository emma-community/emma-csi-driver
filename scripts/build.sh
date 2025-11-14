#!/usr/bin/env bash

# Build script for Emma CSI Driver
# This script builds both controller and node binaries

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Build variables
VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
BINARY_DIR="${ROOT_DIR}/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Create binary directory
mkdir -p "${BINARY_DIR}"

log_info "Building Emma CSI Driver binaries..."
log_info "Version: ${VERSION}"
log_info "Commit: ${COMMIT}"
log_info "Build Date: ${BUILD_DATE}"

# Build controller
log_info "Building controller binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE} -w -s" \
    -o "${BINARY_DIR}/emma-csi-controller" \
    "${ROOT_DIR}/cmd/controller"

if [ $? -eq 0 ]; then
    log_info "Controller binary built successfully: ${BINARY_DIR}/emma-csi-controller"
else
    log_error "Failed to build controller binary"
    exit 1
fi

# Build node
log_info "Building node binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE} -w -s" \
    -o "${BINARY_DIR}/emma-csi-node" \
    "${ROOT_DIR}/cmd/node"

if [ $? -eq 0 ]; then
    log_info "Node binary built successfully: ${BINARY_DIR}/emma-csi-node"
else
    log_error "Failed to build node binary"
    exit 1
fi

log_info "Build complete!"
