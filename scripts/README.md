# Build Scripts

This directory contains build and release scripts for the Emma CSI Driver.

## Scripts

### build.sh

Builds the controller and node binaries for Linux/amd64.

**Usage:**

```bash
./scripts/build.sh
```

**Environment Variables:**

- `VERSION` - Version string (default: `dev`)
- `COMMIT` - Git commit hash (default: auto-detected)
- `BUILD_DATE` - Build timestamp (default: current UTC time)

**Example:**

```bash
VERSION=v1.0.0 ./scripts/build.sh
```

### docker-build.sh

Builds Docker images for the controller and node components.

**Usage:**

```bash
./scripts/docker-build.sh [OPTIONS]
```

**Options:**

- `--controller-only` - Build only the controller image
- `--node-only` - Build only the node image
- `--help` - Show help message

**Environment Variables:**

- `IMAGE_REGISTRY` - Docker registry (default: `docker.io`)
- `IMAGE_NAMESPACE` - Image namespace (default: `emma`)
- `VERSION` - Version tag (default: `dev`)
- `IMAGE_TAG` - Image tag (default: `$VERSION`)
- `COMMIT` - Git commit hash (default: auto-detected)
- `BUILD_DATE` - Build timestamp (default: current UTC time)

**Examples:**

```bash
# Build both images with default settings
./scripts/docker-build.sh

# Build with custom version
VERSION=v1.0.0 ./scripts/docker-build.sh

# Build only controller image
./scripts/docker-build.sh --controller-only

# Build with custom registry and namespace
IMAGE_REGISTRY=ghcr.io IMAGE_NAMESPACE=myorg VERSION=v1.0.0 ./scripts/docker-build.sh
```

### docker-push.sh

Pushes Docker images to a registry.

**Usage:**

```bash
./scripts/docker-push.sh [OPTIONS]
```

**Options:**

- `--controller-only` - Push only the controller image
- `--node-only` - Push only the node image
- `--no-latest` - Don't push 'latest' tag
- `--help` - Show help message

**Environment Variables:**

- `IMAGE_REGISTRY` - Docker registry (default: `docker.io`)
- `IMAGE_NAMESPACE` - Image namespace (default: `emma`)
- `VERSION` - Version tag (default: `dev`)
- `IMAGE_TAG` - Image tag (default: `$VERSION`)

**Examples:**

```bash
# Push both images with default settings
./scripts/docker-push.sh

# Push with custom version
VERSION=v1.0.0 ./scripts/docker-push.sh

# Push only controller image
./scripts/docker-push.sh --controller-only

# Push without 'latest' tag
./scripts/docker-push.sh --no-latest

# Push to custom registry
IMAGE_REGISTRY=ghcr.io IMAGE_NAMESPACE=myorg VERSION=v1.0.0 ./scripts/docker-push.sh
```

## Makefile Targets

The project Makefile provides convenient targets for building and releasing:

### Build Targets

```bash
# Build binaries
make build

# Build controller binary only
make controller

# Build node binary only
make node

# Build Docker images
make docker-build

# Build controller Docker image only
make docker-build-controller

# Build node Docker image only
make docker-build-node
```

### Push Targets

```bash
# Push Docker images
make docker-push

# Push controller Docker image only
make docker-push-controller

# Push node Docker image only
make docker-push-node
```

### Release Target

```bash
# Build and push images in one command
make release VERSION=v1.0.0
```

### Other Targets

```bash
# Run tests
make test

# Format code
make fmt

# Run go vet
make vet

# Download dependencies
make deps

# Clean build artifacts
make clean
```

## CI/CD Workflows

The project includes GitHub Actions workflows for automated testing and releases:

### CI Workflow (`.github/workflows/ci.yml`)

Runs on every push and pull request to `main` and `develop` branches:

- Lints code with `go fmt`, `go vet`, and `golangci-lint`
- Runs tests with race detection and coverage
- Builds binaries
- Uploads artifacts

### Docker Build Workflow (`.github/workflows/docker-build.yml`)

Runs on pull requests when Docker-related files change:

- Builds Docker images without pushing
- Validates Dockerfile changes

### Release Workflow (`.github/workflows/release.yml`)

Runs when a version tag is pushed (e.g., `v1.0.0`):

- Builds and pushes Docker images to registry
- Tags images with version and `latest`
- Creates GitHub release with release notes

**To create a release:**

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Docker Registry Authentication

Before pushing images, authenticate with your Docker registry:

```bash
# Docker Hub
docker login

# GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Other registries
docker login <registry-url>
```

## Multi-Architecture Builds

To build for multiple architectures, use Docker Buildx:

```bash
# Create a new builder
docker buildx create --name multiarch --use

# Build for multiple platforms
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --target controller \
  -t emma/emma-csi-controller:v1.0.0 \
  --push \
  .
```
