#!/usr/bin/env bash

# Docker build script for Emma CSI Driver
# This script builds Docker images for both controller and node components

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Docker variables
IMAGE_REGISTRY="${IMAGE_REGISTRY:-docker.io}"
IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-emma}"
VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
IMAGE_TAG="${IMAGE_TAG:-${VERSION}}"

CONTROLLER_IMAGE="${IMAGE_REGISTRY}/${IMAGE_NAMESPACE}/emma-csi-controller"
NODE_IMAGE="${IMAGE_REGISTRY}/${IMAGE_NAMESPACE}/emma-csi-node"

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

# Parse command line arguments
BUILD_CONTROLLER=true
BUILD_NODE=true

while [[ $# -gt 0 ]]; do
    case $1 in
        --controller-only)
            BUILD_NODE=false
            shift
            ;;
        --node-only)
            BUILD_CONTROLLER=false
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --controller-only    Build only the controller image"
            echo "  --node-only          Build only the node image"
            echo "  --help               Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  IMAGE_REGISTRY       Docker registry (default: docker.io)"
            echo "  IMAGE_NAMESPACE      Image namespace (default: emma)"
            echo "  VERSION              Version tag (default: dev)"
            echo "  IMAGE_TAG            Image tag (default: \$VERSION)"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

log_info "Building Emma CSI Driver Docker images..."
log_info "Registry: ${IMAGE_REGISTRY}"
log_info "Namespace: ${IMAGE_NAMESPACE}"
log_info "Version: ${VERSION}"
log_info "Commit: ${COMMIT}"
log_info "Build Date: ${BUILD_DATE}"
log_info "Image Tag: ${IMAGE_TAG}"

cd "${ROOT_DIR}"

# Build controller image
if [ "${BUILD_CONTROLLER}" = true ]; then
    log_info "Building controller Docker image..."
    docker build \
        --target controller \
        --build-arg VERSION="${VERSION}" \
        --build-arg COMMIT="${COMMIT}" \
        --build-arg BUILD_DATE="${BUILD_DATE}" \
        -t "${CONTROLLER_IMAGE}:${IMAGE_TAG}" \
        -t "${CONTROLLER_IMAGE}:latest" \
        -f Dockerfile .
    
    if [ $? -eq 0 ]; then
        log_info "Controller image built successfully:"
        log_info "  ${CONTROLLER_IMAGE}:${IMAGE_TAG}"
        log_info "  ${CONTROLLER_IMAGE}:latest"
    else
        log_error "Failed to build controller image"
        exit 1
    fi
fi

# Build node image
if [ "${BUILD_NODE}" = true ]; then
    log_info "Building node Docker image..."
    docker build \
        --target node \
        --build-arg VERSION="${VERSION}" \
        --build-arg COMMIT="${COMMIT}" \
        --build-arg BUILD_DATE="${BUILD_DATE}" \
        -t "${NODE_IMAGE}:${IMAGE_TAG}" \
        -t "${NODE_IMAGE}:latest" \
        -f Dockerfile .
    
    if [ $? -eq 0 ]; then
        log_info "Node image built successfully:"
        log_info "  ${NODE_IMAGE}:${IMAGE_TAG}"
        log_info "  ${NODE_IMAGE}:latest"
    else
        log_error "Failed to build node image"
        exit 1
    fi
fi

log_info "Docker build complete!"
