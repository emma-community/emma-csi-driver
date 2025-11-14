#!/usr/bin/env bash

# Docker push script for Emma CSI Driver
# This script pushes Docker images to a registry

set -euo pipefail

# Docker variables
IMAGE_REGISTRY="${IMAGE_REGISTRY:-docker.io}"
IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-emma}"
VERSION="${VERSION:-dev}"
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
PUSH_CONTROLLER=true
PUSH_NODE=true
PUSH_LATEST=true

while [[ $# -gt 0 ]]; do
    case $1 in
        --controller-only)
            PUSH_NODE=false
            shift
            ;;
        --node-only)
            PUSH_CONTROLLER=false
            shift
            ;;
        --no-latest)
            PUSH_LATEST=false
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --controller-only    Push only the controller image"
            echo "  --node-only          Push only the node image"
            echo "  --no-latest          Don't push 'latest' tag"
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

log_info "Pushing Emma CSI Driver Docker images..."
log_info "Registry: ${IMAGE_REGISTRY}"
log_info "Namespace: ${IMAGE_NAMESPACE}"
log_info "Image Tag: ${IMAGE_TAG}"

# Push controller image
if [ "${PUSH_CONTROLLER}" = true ]; then
    log_info "Pushing controller image: ${CONTROLLER_IMAGE}:${IMAGE_TAG}"
    docker push "${CONTROLLER_IMAGE}:${IMAGE_TAG}"
    
    if [ $? -eq 0 ]; then
        log_info "Controller image pushed successfully: ${CONTROLLER_IMAGE}:${IMAGE_TAG}"
    else
        log_error "Failed to push controller image"
        exit 1
    fi
    
    if [ "${PUSH_LATEST}" = true ]; then
        log_info "Pushing controller image: ${CONTROLLER_IMAGE}:latest"
        docker push "${CONTROLLER_IMAGE}:latest"
        
        if [ $? -eq 0 ]; then
            log_info "Controller image pushed successfully: ${CONTROLLER_IMAGE}:latest"
        else
            log_error "Failed to push controller image (latest)"
            exit 1
        fi
    fi
fi

# Push node image
if [ "${PUSH_NODE}" = true ]; then
    log_info "Pushing node image: ${NODE_IMAGE}:${IMAGE_TAG}"
    docker push "${NODE_IMAGE}:${IMAGE_TAG}"
    
    if [ $? -eq 0 ]; then
        log_info "Node image pushed successfully: ${NODE_IMAGE}:${IMAGE_TAG}"
    else
        log_error "Failed to push node image"
        exit 1
    fi
    
    if [ "${PUSH_LATEST}" = true ]; then
        log_info "Pushing node image: ${NODE_IMAGE}:latest"
        docker push "${NODE_IMAGE}:latest"
        
        if [ $? -eq 0 ]; then
            log_info "Node image pushed successfully: ${NODE_IMAGE}:latest"
        else
            log_error "Failed to push node image (latest)"
            exit 1
        fi
    fi
fi

log_info "Docker push complete!"
