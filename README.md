# Emma CSI Driver

A Container Storage Interface (CSI) driver for Kubernetes that enables dynamic provisioning and management of persistent volumes using the Emma.ms cloud platform.

## Overview

The Emma CSI Driver provides similar functionality to the AWS EBS CSI driver but integrates with Emma.ms infrastructure. It allows Kubernetes clusters to dynamically provision, attach, detach, and manage block storage volumes through the Emma.ms API.

## Features

- Dynamic volume provisioning
- Volume attachment and detachment
- Volume expansion
- Multiple volume types (SSD, SSD-Plus, HDD)
- Multi-datacenter support
- Automatic retry with exponential backoff
- Comprehensive logging and metrics

## Project Structure

```
.
├── cmd/
│   ├── controller/     # Controller service binary
│   └── node/          # Node plugin binary
├── pkg/
│   └── driver/        # CSI driver implementation
├── deploy/            # Kubernetes deployment manifests
├── go.mod
└── README.md
```

## Requirements

- Kubernetes 1.20+
- Emma.ms account with API access
- Go 1.21+ (for building from source)

## Installation

Installation instructions will be provided once the driver is fully implemented.

## Development

### Building

```bash
# Build controller binary
go build -o bin/emma-csi-controller ./cmd/controller

# Build node binary
go build -o bin/emma-csi-node ./cmd/node
```

### Testing

```bash
go test ./...
```

## License

TBD
