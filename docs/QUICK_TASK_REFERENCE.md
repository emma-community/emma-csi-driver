# Quick Task Reference

## Installation

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
```

## Most Used Commands

```bash
# List all tasks
task --list

# Format, test, and build
task fmt test build

# Run CI pipeline
task ci

# Build and push to Docker Hub
task docker:build docker:push

# Full release
task release
```

## Your Workflow

```bash
# 1. Make changes to code

# 2. Format and test
task fmt test

# 3. Build and push
task docker:build docker:push

# 4. Deploy to Kubernetes
export KUBECONFIG=~/.kube/config-emmacsi
task k8s:deploy k8s:rollout

# 5. Check logs
task k8s:logs:node
```

## Quick Tasks

| Command | Description |
|---------|-------------|
| `task lint` | Run all linting |
| `task test` | Run all tests |
| `task build` | Build binaries |
| `task docker:build` | Build Docker images |
| `task docker:push` | Push to registry |
| `task ci` | Lint + test + build |
| `task release` | Full release pipeline |
| `task clean` | Clean artifacts |

## Custom Configuration

```bash
# Use custom registry
REGISTRY=ghcr.io task docker:build

# Use custom version
VERSION=v2.0.0 task build

# Combine
REGISTRY=ghcr.io VERSION=v2.0.0 task release
```

## Troubleshooting

```bash
# See what will run
task --dry build

# Verbose output
task --verbose build

# Force run
task --force build
```

## Help

```bash
# Show task info
task info

# Show version
task version

# Read full guide
cat TASKFILE_GUIDE.md
```
