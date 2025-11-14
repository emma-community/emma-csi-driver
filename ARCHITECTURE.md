# Emma CSI Driver Architecture

## Project Structure

```
emma-csi-driver/
├── cmd/                    # Main applications
│   ├── controller/        # Controller service entry point
│   │   └── main.go       # Controller binary main
│   └── node/             # Node plugin entry point
│       └── main.go       # Node binary main
│
├── pkg/                   # Library code
│   └── driver/           # CSI driver implementation
│       ├── driver.go     # Main driver struct and initialization
│       ├── server.go     # gRPC server implementation
│       ├── identity.go   # CSI Identity service
│       ├── controller.go # CSI Controller service
│       └── node.go       # CSI Node service
│
├── deploy/               # Kubernetes manifests
│   └── (to be added)    # YAML files for deployment
│
├── go.mod               # Go module definition
├── Makefile            # Build automation
└── README.md           # Project documentation
```

## Components

### Controller Service (`cmd/controller/`)

The controller service runs as a StatefulSet in Kubernetes and handles:
- Volume lifecycle operations (create, delete, expand)
- Volume attachment/detachment to nodes
- Communication with Emma.ms API

**Command-line flags:**
- `--endpoint`: CSI socket endpoint (default: unix:///var/lib/csi/sockets/pluginproxy/csi.sock)
- `--emma-api-url`: Emma API base URL (default: https://api.emma.ms/external)
- `--client-id`: Emma API client ID (required)
- `--client-secret`: Emma API client secret (required)
- `--datacenter-id`: Default datacenter ID
- `--log-level`: Log level (debug, info, warn, error)

### Node Plugin (`cmd/node/`)

The node plugin runs as a DaemonSet on each Kubernetes worker node and handles:
- Volume staging (formatting and preparing volumes)
- Volume publishing (mounting to pod directories)
- Filesystem operations
- Volume statistics

**Command-line flags:**
- `--endpoint`: CSI socket endpoint (default: unix:///csi/csi.sock)
- `--node-id`: Node ID (VM ID in Emma, can be set via NODE_ID env var)
- `--log-level`: Log level (debug, info, warn, error)

### Driver Package (`pkg/driver/`)

Core CSI driver implementation:

#### `driver.go`
- Main Driver struct
- Driver initialization and lifecycle management
- Service registration

#### `server.go`
- Non-blocking gRPC server
- Request logging and interceptors
- Socket management

#### `identity.go`
- CSI Identity Service implementation
- Plugin information and capabilities
- Health checks

#### `controller.go`
- CSI Controller Service implementation
- Volume CRUD operations
- Volume attachment/detachment
- Volume expansion

#### `node.go`
- CSI Node Service implementation
- Volume staging/unstaging
- Volume publishing/unpublishing
- Filesystem operations

## CSI Specification Compliance

The driver implements CSI specification v1.5.0 with the following capabilities:

### Plugin Capabilities
- CONTROLLER_SERVICE
- VOLUME_ACCESSIBILITY_CONSTRAINTS

### Controller Capabilities
- CREATE_DELETE_VOLUME
- PUBLISH_UNPUBLISH_VOLUME
- EXPAND_VOLUME
- LIST_VOLUMES

### Node Capabilities
- STAGE_UNSTAGE_VOLUME
- EXPAND_VOLUME
- GET_VOLUME_STATS

## Build System

The Makefile provides the following targets:

- `make build`: Build both controller and node binaries
- `make controller`: Build only the controller binary
- `make node`: Build only the node binary
- `make test`: Run all tests
- `make fmt`: Format Go code
- `make vet`: Run go vet
- `make clean`: Clean build artifacts
- `make deps`: Download and tidy dependencies

## Next Steps

The following components need to be implemented:

1. Emma API client (authentication, volume operations, VM operations)
2. Complete CSI method implementations
3. Mount utilities for node plugin
4. Kubernetes deployment manifests
5. Integration tests
6. Documentation

See the tasks.md file in `.kiro/specs/emma-csi-driver/` for the detailed implementation plan.
