# Emma CSI Driver API Reference

This document provides detailed information about the Emma API integration, error handling, and retry behavior in the Emma CSI Driver.

## Table of Contents

- [Emma API Overview](#emma-api-overview)
- [Authentication](#authentication)
- [Volume Operations](#volume-operations)
- [VM Operations](#vm-operations)
- [Configuration Operations](#configuration-operations)
- [Error Handling](#error-handling)
- [Retry Behavior](#retry-behavior)
- [Rate Limiting](#rate-limiting)

## Emma API Overview

The Emma CSI Driver integrates with the Emma.ms REST API to manage block storage volumes. All API communication uses HTTPS and requires Bearer token authentication.

**Base URL**: `https://api.emma.ms/external`

**API Version**: v1

**Content Type**: `application/json`

### API Client Configuration

The driver's Emma API client is configured with the following defaults:

```go
type ClientConfig struct {
    BaseURL        string        // Default: "https://api.emma.ms/external"
    Timeout        time.Duration // Default: 30s
    MaxRetries     int           // Default: 5
    InitialBackoff time.Duration // Default: 1s
    MaxBackoff     time.Duration // Default: 30s
}
```

## Authentication

### Token Issuance

The driver authenticates using OAuth2-style client credentials flow.

**Endpoint**: `POST /v1/issue-token`

**Request**:
```json
{
  "clientId": "your-client-id",
  "clientSecret": "your-client-secret"
}
```

**Response** (200 OK):
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 3600
}
```

**Fields**:
- `accessToken`: JWT token for API authentication (valid for 1 hour)
- `refreshToken`: Token for refreshing access token (valid for 24 hours)
- `expiresIn`: Access token lifetime in seconds

**Error Responses**:
- `400 Bad Request`: Missing or invalid credentials
- `401 Unauthorized`: Invalid client ID or secret
- `429 Too Many Requests`: Rate limit exceeded

### Token Refresh

The driver automatically refreshes access tokens before expiration.

**Endpoint**: `POST /v1/refresh-token`

**Request**:
```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response** (200 OK):
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 3600
}
```

**Refresh Strategy**:
- Tokens are refreshed 5 minutes before expiration
- If refresh fails, driver re-authenticates with client credentials
- Token refresh is thread-safe (uses mutex)

### Authorization Header

All authenticated requests include the Bearer token:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## Volume Operations

### Create Volume

Creates a new block storage volume.

**Endpoint**: `POST /v1/volumes`

**Request**:
```json
{
  "name": "pvc-abc123-def456",
  "volumeGb": 10,
  "volumeType": "ssd",
  "dataCenterId": "aws-eu-west-2"
}
```

**Request Fields**:
- `name` (string, required): Volume name (max 255 characters)
- `volumeGb` (integer, required): Volume size in GB (must match available configs)
- `volumeType` (string, required): Volume type - `ssd`, `ssd-plus`, or `hdd`
- `dataCenterId` (string, required): Emma datacenter identifier

**Response** (200 OK):
```json
{
  "volumeId": 12345,
  "name": "pvc-abc123-def456",
  "volumeGb": 10,
  "volumeType": "ssd",
  "status": "DRAFT",
  "dataCenterId": "aws-eu-west-2",
  "createdAt": "2025-01-15T10:30:45Z"
}
```

**Response Fields**:
- `volumeId` (integer): Unique volume identifier
- `status` (string): Volume state - `DRAFT`, `BUSY`, `AVAILABLE`, `ACTIVE`, `FAILED`, `DELETED`
- Other fields echo request parameters

**Volume Status Lifecycle**:
1. `DRAFT`: Volume created, provisioning in progress
2. `BUSY`: Volume being modified (resize, attach, detach)
3. `AVAILABLE`: Volume ready for attachment
4. `ACTIVE`: Volume attached to a VM
5. `FAILED`: Provisioning or operation failed
6. `DELETED`: Volume deleted

**Error Responses**:
- `400 Bad Request`: Invalid parameters (e.g., unsupported size)
- `401 Unauthorized`: Invalid or expired token
- `403 Forbidden`: Insufficient permissions
- `422 Unprocessable Entity`: Invalid field values
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Emma platform error
- `503 Service Unavailable`: Emma platform temporarily unavailable

**Driver Behavior**:
- Polls volume status until `AVAILABLE` (max 5 minutes)
- Retries on 5xx errors with exponential backoff
- Returns error if volume reaches `FAILED` status

### Get Volume

Retrieves volume details.

**Endpoint**: `GET /v1/volumes/{volumeId}`

**Path Parameters**:
- `volumeId` (integer): Volume identifier

**Response** (200 OK):
```json
{
  "volumeId": 12345,
  "name": "pvc-abc123-def456",
  "volumeGb": 10,
  "volumeType": "ssd",
  "status": "ACTIVE",
  "dataCenterId": "aws-eu-west-2",
  "attachedToVmId": 67890,
  "createdAt": "2025-01-15T10:30:45Z",
  "updatedAt": "2025-01-15T10:35:20Z"
}
```

**Additional Fields**:
- `attachedToVmId` (integer, nullable): VM ID if volume is attached
- `updatedAt` (string): Last modification timestamp

**Error Responses**:
- `404 Not Found`: Volume doesn't exist
- `401 Unauthorized`: Invalid or expired token

### List Volumes

Lists all volumes in the project.

**Endpoint**: `GET /v1/volumes`

**Query Parameters**:
- `page` (integer, optional): Page number (default: 1)
- `perPage` (integer, optional): Items per page (default: 50, max: 100)

**Response** (200 OK):
```json
{
  "volumes": [
    {
      "volumeId": 12345,
      "name": "pvc-abc123-def456",
      "volumeGb": 10,
      "volumeType": "ssd",
      "status": "ACTIVE",
      "dataCenterId": "aws-eu-west-2",
      "attachedToVmId": 67890,
      "createdAt": "2025-01-15T10:30:45Z"
    },
    {
      "volumeId": 12346,
      "name": "pvc-xyz789-uvw012",
      "volumeGb": 20,
      "volumeType": "hdd",
      "status": "AVAILABLE",
      "dataCenterId": "aws-eu-west-2",
      "attachedToVmId": null,
      "createdAt": "2025-01-15T11:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "perPage": 50,
    "total": 2,
    "totalPages": 1
  }
}
```

**Driver Behavior**:
- Used by `ListVolumes` CSI method
- Fetches all pages if total > perPage

### Delete Volume

Deletes a volume.

**Endpoint**: `DELETE /v1/volumes/{volumeId}`

**Path Parameters**:
- `volumeId` (integer): Volume identifier

**Response** (204 No Content): Empty body

**Error Responses**:
- `404 Not Found`: Volume doesn't exist (treated as success by driver)
- `409 Conflict`: Volume is attached (must detach first)
- `401 Unauthorized`: Invalid or expired token

**Driver Behavior**:
- Ensures volume is detached before deletion
- Treats 404 as successful deletion (idempotent)
- Retries on 5xx errors

### Resize Volume

Expands volume capacity.

**Endpoint**: `POST /v1/volumes/{volumeId}/actions`

**Path Parameters**:
- `volumeId` (integer): Volume identifier

**Request**:
```json
{
  "action": "edit",
  "volumeGb": 20
}
```

**Request Fields**:
- `action` (string, required): Must be `"edit"`
- `volumeGb` (integer, required): New size in GB (must be larger than current)

**Response** (200 OK):
```json
{
  "volumeId": 12345,
  "name": "pvc-abc123-def456",
  "volumeGb": 20,
  "volumeType": "ssd",
  "status": "BUSY",
  "dataCenterId": "aws-eu-west-2",
  "attachedToVmId": 67890,
  "updatedAt": "2025-01-15T12:00:00Z"
}
```

**Error Responses**:
- `400 Bad Request`: New size smaller than current size
- `422 Unprocessable Entity`: Invalid size (not in volume configs)
- `404 Not Found`: Volume doesn't exist

**Driver Behavior**:
- Validates new size against volume configs
- Polls volume status until `AVAILABLE` or `ACTIVE`
- Triggers filesystem expansion on node after volume resize

## VM Operations

### Attach Volume

Attaches a volume to a virtual machine.

**Endpoint**: `POST /v1/vms/{vmId}/actions`

**Path Parameters**:
- `vmId` (integer): Virtual machine identifier

**Request**:
```json
{
  "action": "attach",
  "volumeId": 12345
}
```

**Request Fields**:
- `action` (string, required): Must be `"attach"`
- `volumeId` (integer, required): Volume to attach

**Response** (200 OK):
```json
{
  "vmId": 67890,
  "name": "k8s-worker-1",
  "status": "ACTIVE",
  "attachedVolumes": [12345],
  "dataCenterId": "aws-eu-west-2"
}
```

**Error Responses**:
- `404 Not Found`: VM or volume doesn't exist
- `409 Conflict`: Volume already attached elsewhere
- `422 Unprocessable Entity`: Maximum volumes per VM exceeded (16)

**Driver Behavior**:
- Polls volume status until `ACTIVE` (attached)
- Polls VM to verify volume in `attachedVolumes` list
- Timeout after 2 minutes
- Retries on 5xx errors

### Detach Volume

Detaches a volume from a virtual machine.

**Endpoint**: `POST /v1/vms/{vmId}/actions`

**Path Parameters**:
- `vmId` (integer): Virtual machine identifier

**Request**:
```json
{
  "action": "detach",
  "volumeId": 12345
}
```

**Request Fields**:
- `action` (string, required): Must be `"detach"`
- `volumeId` (integer, required): Volume to detach

**Response** (200 OK):
```json
{
  "vmId": 67890,
  "name": "k8s-worker-1",
  "status": "ACTIVE",
  "attachedVolumes": [],
  "dataCenterId": "aws-eu-west-2"
}
```

**Error Responses**:
- `404 Not Found`: VM or volume doesn't exist (treated as success)
- `409 Conflict`: Volume not attached to this VM

**Driver Behavior**:
- Polls volume status until `AVAILABLE` (detached)
- Polls VM to verify volume removed from `attachedVolumes`
- Timeout after 2 minutes
- Retries with exponential backoff (up to 5 attempts)
- Treats 404 as successful detachment (idempotent)

### Get VM

Retrieves virtual machine details.

**Endpoint**: `GET /v1/vms/{vmId}`

**Path Parameters**:
- `vmId` (integer): Virtual machine identifier

**Response** (200 OK):
```json
{
  "vmId": 67890,
  "name": "k8s-worker-1",
  "status": "ACTIVE",
  "attachedVolumes": [12345, 12346],
  "dataCenterId": "aws-eu-west-2",
  "osType": "linux",
  "createdAt": "2025-01-10T08:00:00Z"
}
```

**Response Fields**:
- `vmId` (integer): Unique VM identifier
- `status` (string): VM state - `ACTIVE`, `STOPPED`, `BUSY`, etc.
- `attachedVolumes` (array): List of attached volume IDs
- `dataCenterId` (string): VM location

**Error Responses**:
- `404 Not Found`: VM doesn't exist

**Driver Usage**:
- Used to verify volume attachment/detachment
- Used to get node VM ID from metadata

## Configuration Operations

### Get Data Centers

Lists available Emma datacenters.

**Endpoint**: `GET /v1/data-centers`

**Response** (200 OK):
```json
{
  "dataCenters": [
    {
      "dataCenterId": "aws-eu-west-2",
      "name": "AWS Europe (London)",
      "provider": "aws",
      "region": "eu-west-2",
      "available": true
    },
    {
      "dataCenterId": "gcp-us-central1",
      "name": "GCP US Central",
      "provider": "gcp",
      "region": "us-central1",
      "available": true
    }
  ]
}
```

**Response Fields**:
- `dataCenterId` (string): Unique datacenter identifier
- `name` (string): Human-readable name
- `provider` (string): Cloud provider - `aws`, `gcp`, `azure`
- `region` (string): Provider-specific region code
- `available` (boolean): Whether datacenter accepts new resources

**Driver Usage**:
- Validates `dataCenterId` in StorageClass parameters
- Used during driver initialization

### Get Volume Configs

Lists available volume configurations (sizes and types).

**Endpoint**: `GET /v1/volume-configs`

**Query Parameters**:
- `dataCenterId` (string, optional): Filter by datacenter
- `volumeType` (string, optional): Filter by type (`ssd`, `ssd-plus`, `hdd`)

**Response** (200 OK):
```json
{
  "volumeConfigs": [
    {
      "providerId": 1,
      "providerName": "aws",
      "locationId": 10,
      "locationName": "eu-west-2",
      "dataCenterId": "aws-eu-west-2",
      "dataCenterName": "AWS Europe (London)",
      "volumeGb": 10,
      "volumeType": "ssd",
      "cost": {
        "amount": 0.10,
        "currency": "USD",
        "period": "hour"
      }
    },
    {
      "providerId": 1,
      "providerName": "aws",
      "locationId": 10,
      "locationName": "eu-west-2",
      "dataCenterId": "aws-eu-west-2",
      "dataCenterName": "AWS Europe (London)",
      "volumeGb": 20,
      "volumeType": "ssd",
      "cost": {
        "amount": 0.20,
        "currency": "USD",
        "period": "hour"
      }
    }
  ]
}
```

**Response Fields**:
- `volumeGb` (integer): Available volume size
- `volumeType` (string): Volume performance tier
- `dataCenterId` (string): Datacenter where config is available
- `cost` (object): Pricing information

**Driver Usage**:
- Validates requested volume size against available configs
- Validates volume type is available in datacenter
- Used during volume creation and expansion

## Error Handling

### Error Response Format

All Emma API errors follow a consistent format:

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "Volume size 15GB is not supported. Available sizes: 10, 20, 50, 100, 200, 500, 1000 GB",
    "details": {
      "field": "volumeGb",
      "value": 15,
      "allowedValues": [10, 20, 50, 100, 200, 500, 1000]
    }
  }
}
```

**Error Fields**:
- `code` (string): Machine-readable error code
- `message` (string): Human-readable error description
- `details` (object, optional): Additional error context

### Error Code Mapping

The driver maps Emma API errors to gRPC status codes:

| HTTP Status | Emma Error Code | gRPC Status | Description |
|-------------|-----------------|-------------|-------------|
| 400 | INVALID_REQUEST | InvalidArgument | Malformed request |
| 400 | INVALID_PARAMETER | InvalidArgument | Invalid parameter value |
| 401 | UNAUTHORIZED | Unauthenticated | Invalid or expired token |
| 403 | FORBIDDEN | PermissionDenied | Insufficient permissions |
| 404 | NOT_FOUND | NotFound | Resource doesn't exist |
| 409 | CONFLICT | FailedPrecondition | Resource state conflict |
| 409 | ALREADY_ATTACHED | FailedPrecondition | Volume attached elsewhere |
| 422 | UNPROCESSABLE_ENTITY | InvalidArgument | Invalid field values |
| 429 | RATE_LIMIT_EXCEEDED | ResourceExhausted | Too many requests |
| 500 | INTERNAL_ERROR | Internal | Emma platform error |
| 503 | SERVICE_UNAVAILABLE | Unavailable | Temporary outage |

### Error Handling Strategy

**Non-Retryable Errors** (fail immediately):
- 400 Bad Request
- 401 Unauthorized (triggers re-authentication)
- 403 Forbidden
- 404 Not Found (except for delete operations)
- 422 Unprocessable Entity

**Retryable Errors** (retry with backoff):
- 429 Too Many Requests
- 500 Internal Server Error
- 502 Bad Gateway
- 503 Service Unavailable
- 504 Gateway Timeout
- Network timeouts
- Connection errors

**Idempotent Operations**:
- Delete volume: 404 treated as success
- Detach volume: 404 treated as success
- Create volume: Checks if volume with same name exists

## Retry Behavior

### Exponential Backoff

The driver implements exponential backoff with jitter for retryable errors:

```
delay = min(initialDelay * (multiplier ^ attempt) * (1 ± jitter), maxDelay)
```

**Default Configuration**:
- Initial delay: 1 second
- Multiplier: 2.0
- Max delay: 30 seconds
- Jitter: ±10%
- Max retries: 5

**Example Retry Delays**:
- Attempt 1: ~1s (0.9-1.1s with jitter)
- Attempt 2: ~2s (1.8-2.2s)
- Attempt 3: ~4s (3.6-4.4s)
- Attempt 4: ~8s (7.2-8.8s)
- Attempt 5: ~16s (14.4-17.6s)

### Retry Logic Flow

```
1. Execute API request
2. If success → return result
3. If non-retryable error → return error
4. If retryable error:
   a. Increment attempt counter
   b. If attempts >= maxRetries → return error
   c. Calculate backoff delay with jitter
   d. Sleep for delay
   e. Go to step 1
```

### Operation-Specific Retry Behavior

**Volume Creation**:
- Retries on 5xx errors
- No retry on 400, 422 (invalid parameters)
- Polls status after successful creation

**Volume Attachment**:
- Retries on 5xx errors
- Retries on 409 if volume is in `BUSY` state
- No retry on 409 if volume attached elsewhere
- Polls status after successful attachment

**Volume Detachment**:
- Retries on all errors (up to 5 attempts)
- Longer backoff for detachment (up to 60s)
- Treats 404 as success

**Volume Deletion**:
- Retries on 5xx errors
- Treats 404 as success (idempotent)
- No retry on 409 (volume still attached)

### Timeout Configuration

**Per-Request Timeouts**:
- API requests: 30 seconds
- Authentication: 10 seconds

**Operation Timeouts**:
- Volume creation: 5 minutes (includes polling)
- Volume attachment: 2 minutes (includes polling)
- Volume detachment: 2 minutes (includes polling)
- Volume deletion: 1 minute
- Volume resize: 5 minutes (includes polling)

**Polling Intervals**:
- Volume status: 5 seconds
- VM status: 5 seconds

## Rate Limiting

### Emma API Rate Limits

Emma API enforces rate limits per service application:

- **Default**: 100 requests per minute
- **Burst**: 20 requests per second

**Rate Limit Headers**:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642252800
```

### Driver Rate Limit Handling

**429 Response Handling**:
1. Extract `Retry-After` header (seconds until reset)
2. Wait for specified duration
3. Retry request (counts toward max retries)

**Example**:
```
Response: 429 Too Many Requests
Retry-After: 30

Driver waits 30 seconds, then retries
```

**Rate Limit Avoidance**:
- Driver doesn't implement client-side rate limiting
- Relies on Kubernetes CSI framework to serialize operations
- Controller runs single replica to avoid concurrent requests
- Node plugins operate independently (different VMs)

### Best Practices

**For High-Volume Clusters**:
1. Request rate limit increase from Emma support
2. Use `WaitForFirstConsumer` binding mode to reduce API calls
3. Avoid frequent PVC create/delete cycles
4. Monitor `emma_csi_api_requests_total{status="429"}` metric

**For Large Volumes**:
- Attachment/detachment operations are slowest (10-30s)
- Plan for operation latency in application deployment
- Use pod disruption budgets to control rollout speed

## API Client Implementation

### Client Interface

```go
type EmmaClient interface {
    // Authentication
    IssueToken(ctx context.Context, clientID, clientSecret string) (*TokenResponse, error)
    RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
    
    // Volume Operations
    CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error)
    GetVolume(ctx context.Context, volumeID int) (*Volume, error)
    ListVolumes(ctx context.Context, page, perPage int) (*ListVolumesResponse, error)
    DeleteVolume(ctx context.Context, volumeID int) error
    ResizeVolume(ctx context.Context, volumeID int, newSizeGB int) (*Volume, error)
    
    // VM Operations
    AttachVolume(ctx context.Context, vmID, volumeID int) error
    DetachVolume(ctx context.Context, vmID, volumeID int) error
    GetVM(ctx context.Context, vmID int) (*VM, error)
    
    // Configuration
    GetDataCenters(ctx context.Context) ([]*DataCenter, error)
    GetVolumeConfigs(ctx context.Context, filters *VolumeConfigFilters) ([]*VolumeConfig, error)
}
```

### Thread Safety

The Emma API client is thread-safe:

- **Token Management**: Uses `sync.RWMutex` for token access
- **HTTP Client**: Reuses single `http.Client` (connection pooling)
- **Concurrent Requests**: Safe for concurrent use from multiple goroutines

### Context Handling

All API methods accept `context.Context`:

- **Cancellation**: Requests cancelled if context cancelled
- **Timeouts**: Requests timeout based on context deadline
- **Tracing**: Context propagates trace IDs for observability

**Example**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

volume, err := client.CreateVolume(ctx, req)
if err != nil {
    // Handle timeout or cancellation
}
```

## Debugging API Issues

### Enable Debug Logging

Set log level to `debug` to see full API request/response details:

```bash
kubectl patch configmap emma-csi-driver-config -n kube-system \
  -p '{"data":{"log-level":"debug"}}'
```

**Debug Log Example**:
```json
{
  "timestamp": "2025-01-15T10:30:45Z",
  "level": "debug",
  "component": "emma-client",
  "method": "POST",
  "url": "https://api.emma.ms/external/v1/volumes",
  "requestBody": "{\"name\":\"pvc-abc123\",\"volumeGb\":10,\"volumeType\":\"ssd\",\"dataCenterId\":\"aws-eu-west-2\"}",
  "responseStatus": 200,
  "responseBody": "{\"volumeId\":12345,\"status\":\"DRAFT\"}",
  "duration_ms": 1250
}
```

### Manual API Testing

Test API endpoints directly using curl:

```bash
# Get token
TOKEN=$(curl -X POST https://api.emma.ms/external/v1/issue-token \
  -H "Content-Type: application/json" \
  -d '{"clientId":"your-id","clientSecret":"your-secret"}' \
  | jq -r '.accessToken')

# Create volume
curl -X POST https://api.emma.ms/external/v1/volumes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-volume",
    "volumeGb": 10,
    "volumeType": "ssd",
    "dataCenterId": "aws-eu-west-2"
  }'

# Get volume
curl -H "Authorization: Bearer $TOKEN" \
  https://api.emma.ms/external/v1/volumes/12345

# Delete volume
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  https://api.emma.ms/external/v1/volumes/12345
```

### Common API Issues

**401 Unauthorized**:
- Token expired → Driver auto-refreshes
- Invalid credentials → Check secret

**422 Unprocessable Entity**:
- Invalid volume size → Check volume configs
- Invalid datacenter → Check datacenter list

**503 Service Unavailable**:
- Emma platform maintenance → Wait and retry
- Check Emma status page

## Additional Resources

- **Emma API Documentation**: https://docs.emma.ms/api
- **Emma Status Page**: https://status.emma.ms
- **CSI Specification**: https://github.com/container-storage-interface/spec
- **Driver Repository**: https://github.com/your-org/emma-csi-driver

