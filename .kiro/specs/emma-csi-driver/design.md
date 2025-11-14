# Emma CSI Driver Design Document

## Overview

The Emma CSI Driver is a Kubernetes Container Storage Interface (CSI) implementation that enables dynamic provisioning and management of persistent volumes using the Emma.ms cloud platform. The driver follows the CSI specification v1.5.0 and provides similar functionality to the AWS EBS CSI driver, adapted for the Emma.ms API.

The driver consists of two main components:
- **Controller Service**: Handles volume lifecycle operations (create, delete, attach, detach, resize)
- **Node Service**: Handles volume staging, publishing, and filesystem operations on worker nodes

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                       │
│                                                              │
│  ┌────────────────┐         ┌──────────────────┐           │
│  │  Controller    │         │   Node Plugin    │           │
│  │  (StatefulSet) │         │   (DaemonSet)    │           │
│  │                │         │                  │           │
│  │  - CSI         │         │  - CSI Node      │           │
│  │    Controller  │         │    Service       │           │
│  │  - Emma API    │         │  - Mount/Unmount │           │
│  │    Client      │         │  - Format        │           │
│  └────────┬───────┘         └────────┬─────────┘           │
│           │                          │                      │
└───────────┼──────────────────────────┼──────────────────────┘
            │                          │
            │                          │
            ▼                          ▼
    ┌───────────────────────────────────────┐
    │         Emma.ms API                    │
    │  https://api.emma.ms/external/v1/     │
    │                                        │
    │  - /volumes (GET, POST, DELETE)       │
    │  - /volumes/{id}/actions (POST)       │
    │  - /vms/{id}/actions (POST)           │
    │  - /data-centers (GET)                │
    │  - /volume-configs (GET)              │
    └───────────────────────────────────────┘
```

### Component Architecture

```
Controller Pod:
┌──────────────────────────────────────────┐
│  CSI Controller Service                   │
│  ┌────────────────────────────────────┐  │
│  │  gRPC Server (unix socket)         │  │
│  │  - CreateVolume                    │  │
│  │  - DeleteVolume                    │  │
│  │  - ControllerPublishVolume         │  │
│  │  - ControllerUnpublishVolume       │  │
│  │  - ControllerExpandVolume          │  │
│  │  - ValidateVolumeCapabilities      │  │
│  │  - ListVolumes                     │  │
│  └────────────────────────────────────┘  │
│                                           │
│  ┌────────────────────────────────────┐  │
│  │  Emma API Client                   │  │
│  │  - Authentication Manager          │  │
│  │  - Volume Operations               │  │
│  │  - VM Operations                   │  │
│  │  - Retry Logic                     │  │
│  │  - Rate Limiting                   │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘

Node Pod (on each worker):
┌──────────────────────────────────────────┐
│  CSI Node Service                         │
│  ┌────────────────────────────────────┐  │
│  │  gRPC Server (unix socket)         │  │
│  │  - NodeStageVolume                 │  │
│  │  - NodeUnstageVolume               │  │
│  │  - NodePublishVolume               │  │
│  │  - NodeUnpublishVolume             │  │
│  │  - NodeExpandVolume                │  │
│  │  - NodeGetCapabilities             │  │
│  │  - NodeGetInfo                     │  │
│  └────────────────────────────────────┘  │
│                                           │
│  ┌────────────────────────────────────┐  │
│  │  Mount/Filesystem Operations       │  │
│  │  - Device Discovery                │  │
│  │  - Filesystem Formatting           │  │
│  │  - Mount/Unmount                   │  │
│  │  - Filesystem Resize               │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

## Components and Interfaces

### 1. CSI Controller Service

The controller service implements the CSI Controller Service gRPC interface and manages volume lifecycle operations.

**Key Responsibilities:**
- Create and delete volumes via Emma API
- Attach and detach volumes to/from VMs
- Expand volume capacity
- Validate volume capabilities
- List volumes

**Interface Methods:**

```go
type ControllerService interface {
    CreateVolume(context.Context, *CreateVolumeRequest) (*CreateVolumeResponse, error)
    DeleteVolume(context.Context, *DeleteVolumeRequest) (*DeleteVolumeResponse, error)
    ControllerPublishVolume(context.Context, *ControllerPublishVolumeRequest) (*ControllerPublishVolumeResponse, error)
    ControllerUnpublishVolume(context.Context, *ControllerUnpublishVolumeRequest) (*ControllerUnpublishVolumeResponse, error)
    ValidateVolumeCapabilities(context.Context, *ValidateVolumeCapabilitiesRequest) (*ValidateVolumeCapabilitiesResponse, error)
    ListVolumes(context.Context, *ListVolumesRequest) (*ListVolumesResponse, error)
    ControllerExpandVolume(context.Context, *ControllerExpandVolumeRequest) (*ControllerExpandVolumeResponse, error)
    ControllerGetCapabilities(context.Context, *ControllerGetCapabilitiesRequest) (*ControllerGetCapabilitiesResponse, error)
}
```

### 2. CSI Node Service

The node service implements the CSI Node Service gRPC interface and handles volume operations on individual nodes.

**Key Responsibilities:**
- Stage volumes (format and prepare for use)
- Publish volumes (mount to pod directories)
- Unpublish volumes (unmount from pod directories)
- Unstage volumes (cleanup after detachment)
- Expand filesystem after volume resize
- Report node capabilities and topology

**Interface Methods:**

```go
type NodeService interface {
    NodeStageVolume(context.Context, *NodeStageVolumeRequest) (*NodeStageVolumeResponse, error)
    NodeUnstageVolume(context.Context, *NodeUnstageVolumeRequest) (*NodeUnstageVolumeResponse, error)
    NodePublishVolume(context.Context, *NodePublishVolumeRequest) (*NodePublishVolumeResponse, error)
    NodeUnpublishVolume(context.Context, *NodeUnpublishVolumeRequest) (*NodeUnpublishVolumeResponse, error)
    NodeGetVolumeStats(context.Context, *NodeGetVolumeStatsRequest) (*NodeGetVolumeStatsResponse, error)
    NodeExpandVolume(context.Context, *NodeExpandVolumeRequest) (*NodeExpandVolumeResponse, error)
    NodeGetCapabilities(context.Context, *NodeGetCapabilitiesRequest) (*NodeGetCapabilitiesResponse, error)
    NodeGetInfo(context.Context, *NodeGetInfoRequest) (*NodeGetInfoResponse, error)
}
```

### 3. Emma API Client

The Emma API client provides a Go interface to the Emma.ms REST API with authentication, retry logic, and error handling.

**Key Components:**

```go
type EmmaClient struct {
    baseURL      string
    httpClient   *http.Client
    authManager  *AuthManager
    rateLimiter  *RateLimiter
}

type AuthManager struct {
    clientID     string
    clientSecret string
    accessToken  string
    refreshToken string
    tokenExpiry  time.Time
    mutex        sync.RWMutex
}

// Volume Operations
func (c *EmmaClient) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error)
func (c *EmmaClient) DeleteVolume(ctx context.Context, volumeID int) error
func (c *EmmaClient) GetVolume(ctx context.Context, volumeID int) (*Volume, error)
func (c *EmmaClient) ListVolumes(ctx context.Context) ([]*Volume, error)
func (c *EmmaClient) ResizeVolume(ctx context.Context, volumeID int, newSizeGB int) error

// VM Operations (for attach/detach)
func (c *EmmaClient) AttachVolume(ctx context.Context, vmID int, volumeID int) error
func (c *EmmaClient) DetachVolume(ctx context.Context, vmID int, volumeID int) error
func (c *EmmaClient) GetVM(ctx context.Context, vmID int) (*VM, error)

// Configuration Operations
func (c *EmmaClient) GetDataCenters(ctx context.Context) ([]*DataCenter, error)
func (c *EmmaClient) GetVolumeConfigs(ctx context.Context, filters *VolumeConfigFilters) ([]*VolumeConfig, error)
```

### 4. Identity Service

The identity service provides driver information and capabilities.

```go
type IdentityService interface {
    GetPluginInfo(context.Context, *GetPluginInfoRequest) (*GetPluginInfoResponse, error)
    GetPluginCapabilities(context.Context, *GetPluginCapabilitiesRequest) (*GetPluginCapabilitiesResponse, error)
    Probe(context.Context, *ProbeRequest) (*ProbeResponse, error)
}
```

## Data Models

### Volume

```go
type Volume struct {
    ID           int
    Name         string
    SizeGB       int
    Type         string // "ssd", "ssd-plus", "hdd"
    Status       string // "DRAFT", "BUSY", "AVAILABLE", "ACTIVE", "FAILED", "DELETED"
    AttachedToID *int   // VM ID if attached
    DataCenterID string
    CreatedAt    time.Time
}
```

### VolumeConfig

```go
type VolumeConfig struct {
    ProviderID     int
    ProviderName   string
    LocationID     int
    LocationName   string
    DataCenterID   string
    DataCenterName string
    VolumeGB       int
    VolumeType     string
    Cost           Cost
}
```

### StorageClass Parameters

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: emma-ssd
provisioner: csi.emma.ms
parameters:
  type: ssd              # Volume type: ssd, ssd-plus, hdd
  dataCenterId: aws-eu-west-2  # Emma data center ID
  fsType: ext4           # Filesystem type: ext4, xfs
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
reclaimPolicy: Delete
```

## Error Handling

### Retry Strategy

The driver implements exponential backoff with jitter for retryable errors:

```go
type RetryConfig struct {
    MaxRetries     int           // Default: 5
    InitialDelay   time.Duration // Default: 1s
    MaxDelay       time.Duration // Default: 30s
    Multiplier     float64       // Default: 2.0
    JitterFraction float64       // Default: 0.1
}
```

**Retryable Errors:**
- HTTP 5xx errors (server errors)
- HTTP 429 (rate limit exceeded)
- Network timeouts
- Connection errors

**Non-Retryable Errors:**
- HTTP 400 (bad request)
- HTTP 401 (unauthorized)
- HTTP 403 (forbidden)
- HTTP 404 (not found)
- HTTP 422 (unprocessable entity)

### Error Mapping

Emma API errors are mapped to appropriate gRPC status codes:

| Emma API Error | HTTP Code | gRPC Status | Description |
|----------------|-----------|-------------|-------------|
| Bad Request | 400 | InvalidArgument | Invalid parameters |
| Unauthorized | 401 | Unauthenticated | Authentication failed |
| Forbidden | 403 | PermissionDenied | Insufficient permissions |
| Not Found | 404 | NotFound | Resource doesn't exist |
| Conflict | 409 | FailedPrecondition | Resource state conflict |
| Unprocessable | 422 | InvalidArgument | Invalid field values |
| Server Error | 500 | Internal | Emma API internal error |

## Testing Strategy

### Unit Tests

- Test each CSI method with mocked Emma API client
- Test authentication manager token refresh logic
- Test retry logic with various error scenarios
- Test volume state transitions
- Test error handling and mapping

### Integration Tests

- Test against Emma API sandbox environment
- Test complete volume lifecycle (create, attach, mount, unmount, detach, delete)
- Test volume expansion workflow
- Test concurrent operations
- Test failure recovery scenarios

### End-to-End Tests

- Deploy driver to test Kubernetes cluster
- Create PVCs and verify volume provisioning
- Deploy pods with PVCs and verify mounting
- Test volume expansion
- Test pod rescheduling with volume reattachment
- Test cleanup on PVC deletion

### Performance Tests

- Measure volume creation latency
- Measure attach/detach latency
- Test concurrent volume operations
- Test rate limiting behavior
- Measure API call overhead

## Security Considerations

### Authentication

- Emma API credentials (clientId, clientSecret) stored as Kubernetes Secret
- Access tokens refreshed automatically before expiration
- Tokens stored in memory only, never logged
- Support for token rotation without driver restart

### Authorization

- Driver requires Emma API "Manage" access level
- Principle of least privilege for Kubernetes RBAC
- Controller service account limited to PV/PVC operations
- Node service account limited to node-specific operations

### Network Security

- All API communication over HTTPS
- TLS certificate validation enabled
- Support for custom CA certificates
- Network policies to restrict egress traffic

### Data Security

- Volumes encrypted at rest (Emma platform feature)
- No sensitive data logged
- Secure deletion of volumes
- Support for volume snapshots (future enhancement)

## Deployment

### Prerequisites

- Kubernetes cluster version 1.20+
- Emma.ms account with API access
- Service application created in Emma.ms with "Manage" access level
- Worker nodes must be Emma.ms VMs in the same project

### Installation Steps

1. Create namespace for CSI driver
2. Create Secret with Emma API credentials
3. Deploy CSI driver components:
   - CSIDriver object
   - Controller StatefulSet
   - Node DaemonSet
   - RBAC resources (ServiceAccounts, ClusterRoles, ClusterRoleBindings)
4. Create StorageClass(es) for different volume types/regions
5. Verify driver health with test PVC

### Configuration

**Driver Configuration (ConfigMap):**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: emma-csi-driver-config
  namespace: kube-system
data:
  emma-api-url: "https://api.emma.ms/external"
  default-datacenter-id: "aws-eu-west-2"
  default-volume-type: "ssd"
  log-level: "info"
  max-volumes-per-node: "16"
```

**Credentials (Secret):**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: emma-api-credentials
  namespace: kube-system
type: Opaque
stringData:
  client-id: "your-client-id"
  client-secret: "your-client-secret"
```

## Monitoring and Observability

### Metrics

The driver exposes Prometheus metrics on port 8080:

- `emma_csi_operations_total{operation, status}` - Total operations count
- `emma_csi_operation_duration_seconds{operation}` - Operation latency histogram
- `emma_csi_api_requests_total{method, endpoint, status}` - API request count
- `emma_csi_api_request_duration_seconds{method, endpoint}` - API request latency
- `emma_csi_volumes_total{status}` - Total volumes by status
- `emma_csi_volume_attach_duration_seconds` - Volume attach latency
- `emma_csi_volume_detach_duration_seconds` - Volume detach latency

### Logging

Structured logging with configurable levels:

- **DEBUG**: Detailed operation traces, API request/response bodies
- **INFO**: Operation lifecycle events, API calls
- **WARN**: Retryable errors, degraded performance
- **ERROR**: Operation failures, non-retryable errors

Log format (JSON):

```json
{
  "timestamp": "2025-01-15T10:30:45Z",
  "level": "info",
  "component": "controller",
  "operation": "CreateVolume",
  "volumeId": "12345",
  "message": "Volume created successfully",
  "duration_ms": 2500
}
```

### Health Checks

- Liveness probe: gRPC health check endpoint
- Readiness probe: Emma API connectivity check
- Startup probe: Initial authentication verification

## Limitations and Future Enhancements

### Current Limitations

- ReadWriteOnce access mode only (single node attachment)
- No volume snapshot support
- No volume cloning support
- No topology-aware scheduling (volumes created in default datacenter)
- Maximum 16 volumes per node (Emma platform limit)

### Future Enhancements

- Volume snapshots and restore
- Volume cloning for faster provisioning
- Topology-aware scheduling based on datacenter
- ReadWriteMany support via NFS gateway
- Volume encryption key management
- Performance optimization with caching
- Support for volume tags and labels
- Integration with Emma.ms backup policies
