#!/bin/bash
set -e

# Emma CSI Driver - Build and Push Script
# This script builds Docker images and pushes them to a container registry

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Default values
REGISTRY="${REGISTRY:-docker.io}"
IMAGE_NAME="${IMAGE_NAME:-emma-csi-driver}"
VERSION="${VERSION:-latest}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -i|--image)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -u|--username)
            USERNAME="$2"
            shift 2
            ;;
        --no-push)
            NO_PUSH=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -r, --registry REGISTRY    Container registry (default: docker.io)"
            echo "  -i, --image IMAGE          Image name (default: emma-csi-driver)"
            echo "  -v, --version VERSION      Image version/tag (default: latest)"
            echo "  -u, --username USERNAME    Registry username (for docker.io)"
            echo "  --no-push                  Build only, don't push"
            echo "  -h, --help                 Show this help message"
            echo ""
            echo "Examples:"
            echo "  # Build and push to Docker Hub"
            echo "  $0 --registry docker.io --username myuser --version v1.0.0"
            echo ""
            echo "  # Build and push to GitHub Container Registry"
            echo "  $0 --registry ghcr.io --username myuser --version v1.0.0"
            echo ""
            echo "  # Build only (local development)"
            echo "  $0 --no-push"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Construct full image name
if [ -n "$USERNAME" ]; then
    FULL_IMAGE="${REGISTRY}/${USERNAME}/${IMAGE_NAME}:${VERSION}"
else
    FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${VERSION}"
fi

print_info "Building Emma CSI Driver"
print_info "Image: $FULL_IMAGE"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    print_error "Docker is not running. Please start Docker Desktop."
    print_info "On macOS: Open Docker Desktop from Applications"
    exit 1
fi

# Check if Dockerfile exists
if [ ! -f "Dockerfile" ]; then
    print_error "Dockerfile not found in current directory"
    exit 1
fi

# Build the image
print_info "Building Docker image..."
docker build -t "$FULL_IMAGE" -f Dockerfile . || {
    print_error "Docker build failed"
    exit 1
}

print_info "✓ Image built successfully: $FULL_IMAGE"

# Tag as latest if not already
if [ "$VERSION" != "latest" ]; then
    LATEST_IMAGE="${FULL_IMAGE%:*}:latest"
    print_info "Tagging as latest: $LATEST_IMAGE"
    docker tag "$FULL_IMAGE" "$LATEST_IMAGE"
fi

# Push to registry
if [ "$NO_PUSH" != "true" ]; then
    print_info "Pushing image to registry..."
    
    # Check if logged in (for docker.io)
    if [ "$REGISTRY" = "docker.io" ] && [ -n "$USERNAME" ]; then
        if ! docker info 2>/dev/null | grep -q "Username: $USERNAME"; then
            print_warn "Not logged in to Docker Hub. Attempting login..."
            docker login docker.io -u "$USERNAME" || {
                print_error "Docker login failed"
                exit 1
            }
        fi
    fi
    
    docker push "$FULL_IMAGE" || {
        print_error "Docker push failed"
        print_info "Make sure you're logged in to the registry:"
        print_info "  docker login $REGISTRY"
        exit 1
    }
    
    print_info "✓ Image pushed successfully"
    
    if [ "$VERSION" != "latest" ]; then
        docker push "$LATEST_IMAGE" || {
            print_warn "Failed to push latest tag"
        }
    fi
else
    print_info "Skipping push (--no-push specified)"
fi

echo ""
print_info "Build complete!"
echo ""
echo "To use this image in your cluster, update the image in deploy/ files:"
echo "  image: $FULL_IMAGE"
echo ""
echo "Or use sed to update all deployment files:"
echo "  sed -i '' 's|image: .*emma.*csi-driver.*|image: $FULL_IMAGE|g' deploy/*.yaml"
echo ""
