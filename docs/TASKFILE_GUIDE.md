# Taskfile Guide

This project uses [Task](https://taskfile.dev) as a task runner for common development operations.

## Installation

### Install Task

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

# Or using Go
go install github.com/go-task/task/v3/cmd/task@latest
```

### Verify Installation

```bash
task --version
```

## Quick Start

```bash
# List all available tasks
task --list

# Run CI pipeline (lint + test + build)
task ci

# Build and push release
task release
```

## Available Tasks

### Linting

```bash
# Run all linting checks
task lint

# Individual linting tasks
task lint:fmt        # Check code formatting
task lint:vet        # Run go vet
task lint:golangci   # Run golangci-lint (if installed)
```

### Formatting

```bash
# Format Go code
task fmt
```

### Testing

```bash
# Run all tests
task test

# Individual test tasks
task test:unit              # Run unit tests
task test:coverage          # Run tests with coverage
task test:coverage:html     # Generate HTML coverage report
```

### Building

```bash
# Build both binaries
task build

# Build individual binaries
task build:controller   # Build controller binary
task build:node        # Build node binary
```

### Docker

```bash
# Build Docker images
task docker:build

# Build individual images
task docker:build:controller
task docker:build:node

# Push images to registry
task docker:push

# Push individual images
task docker:push:controller
task docker:push:node
```

### Combined Tasks

```bash
# CI pipeline: lint + test + build
task ci

# Full release: lint + test + build + push
task release
```

### Kubernetes

```bash
# Deploy to Kubernetes
task k8s:deploy

# Wait for rollout to complete
task k8s:rollout

# View logs
task k8s:logs:controller
task k8s:logs:node
```

### Cleanup

```bash
# Clean build artifacts
task clean

# Clean everything including Docker images
task clean:all
```

### Development

```bash
# Setup development environment
task dev:setup

# Watch for changes and rebuild
task dev:watch
```

### Information

```bash
# Show build information
task info

# Show version
task version
```

## Configuration

### Environment Variables

You can override default values using environment variables:

```bash
# Custom registry
REGISTRY=ghcr.io task docker:build

# Custom namespace
NAMESPACE=myorg/myproject task docker:build

# Custom version
VERSION=v2.0.0 task build

# Combine multiple variables
REGISTRY=ghcr.io NAMESPACE=myorg VERSION=v2.0.0 task release
```

### Using .env File

Create a `.env` file in the project root:

```bash
REGISTRY=ghcr.io
NAMESPACE=myorg/myproject
VERSION=v1.2.0
```

Task will automatically load these variables.

## Common Workflows

### Development Workflow

```bash
# 1. Setup environment
task dev:setup

# 2. Make changes to code

# 3. Format code
task fmt

# 4. Run tests
task test

# 5. Build binaries
task build

# 6. Build Docker images
task docker:build
```

### CI/CD Workflow

```bash
# Run full CI pipeline
task ci

# If CI passes, release
task release
```

### Quick Test and Build

```bash
# Test and build in one command
task test build
```

### Deploy to Kubernetes

```bash
# 1. Build and push images
task docker:build docker:push

# 2. Deploy to Kubernetes
task k8s:deploy

# 3. Wait for rollout
task k8s:rollout

# 4. Check logs
task k8s:logs:controller
```

## Examples

### Build with Custom Version

```bash
VERSION=v1.2.0 task build
```

### Push to Different Registry

```bash
REGISTRY=ghcr.io NAMESPACE=myorg task docker:push
```

### Run Specific Tests

```bash
# Run tests for specific package
go test -v ./pkg/driver/...

# Or use task for all tests
task test:unit
```

### Generate Coverage Report

```bash
task test:coverage:html
open coverage.html
```

### Watch and Rebuild

```bash
# Requires inotifywait (Linux) or fswatch (macOS)
task dev:watch
```

## Task Dependencies

Some tasks have dependencies that run automatically:

```bash
# This will run docker:build first, then push
task docker:push

# This will run test:coverage first, then generate HTML
task test:coverage:html

# This will run lint, test, build, docker:build, then docker:push
task release
```

## Parallel Execution

Run multiple tasks in parallel:

```bash
# Build both binaries in parallel
task build:controller build:node

# Run linting and tests in parallel
task lint test
```

## Tips and Tricks

### Dry Run

See what commands will be executed without running them:

```bash
task --dry build
```

### Verbose Output

Show all commands being executed:

```bash
task --verbose build
```

### Force Execution

Force task to run even if up-to-date:

```bash
task --force build
```

### List Tasks with Description

```bash
task --list
```

### Run Task from Subdirectory

Task will automatically find the Taskfile in parent directories:

```bash
cd pkg/driver
task build  # Works!
```

## Integration with Make

If you prefer Make, you can create a simple Makefile wrapper:

```makefile
.PHONY: all
all:
	task ci

.PHONY: build
build:
	task build

.PHONY: test
test:
	task test

.PHONY: release
release:
	task release
```

## Troubleshooting

### Task Not Found

```bash
# Make sure Task is installed
task --version

# Make sure you're in the project root
ls Taskfile.yml
```

### Docker Build Fails

```bash
# Make sure Docker is running
docker ps

# Make sure buildx is available
docker buildx version
```

### golangci-lint Not Found

```bash
# Install golangci-lint
task dev:setup

# Or manually
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Permission Denied

```bash
# Make sure Docker doesn't require sudo
sudo usermod -aG docker $USER
newgrp docker
```

## Comparison with Makefile

| Feature | Taskfile | Makefile |
|---------|----------|----------|
| Syntax | YAML | Make syntax |
| Cross-platform | ✅ Yes | ⚠️ Limited |
| Variables | Easy | Complex |
| Dependencies | Clear | Implicit |
| Parallel execution | ✅ Built-in | ⚠️ Limited |
| Documentation | ✅ Built-in | Manual |

## Additional Resources

- [Task Documentation](https://taskfile.dev)
- [Task GitHub](https://github.com/go-task/task)
- [Task Examples](https://github.com/go-task/task/tree/main/docs/docs)

## Summary

**Quick commands:**

```bash
# Development
task fmt test build

# CI
task ci

# Release
task release

# Deploy
task docker:push k8s:deploy k8s:rollout
```

**Your specific workflow:**

```bash
# Build and push to your registry
task docker:build docker:push

# Deploy to Kubernetes
export KUBECONFIG=~/.kube/config-emmacsi
task k8s:deploy k8s:rollout
```
