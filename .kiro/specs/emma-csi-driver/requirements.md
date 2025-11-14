# Requirements Document

## Introduction

This document specifies the requirements for an Emma CSI (Container Storage Interface) Driver that enables Kubernetes clusters to dynamically provision and manage persistent volumes using the Emma.ms cloud platform API. The driver SHALL provide similar functionality to the AWS EBS CSI driver but SHALL integrate with Emma.ms infrastructure instead.

## Glossary

- **CSI_Driver**: The Container Storage Interface driver implementation that interfaces between Kubernetes and Emma.ms API
- **Emma_API**: The RESTful API provided by Emma.ms for managing cloud infrastructure resources
- **Volume**: A block storage resource in Emma.ms that can be attached to virtual machines
- **PersistentVolume**: A Kubernetes resource representing storage in the cluster
- **PersistentVolumeClaim**: A Kubernetes resource representing a user's request for storage
- **StorageClass**: A Kubernetes resource that defines different classes of storage
- **Node**: A Kubernetes worker node (virtual machine) where pods run
- **Controller**: The CSI controller component that handles volume lifecycle operations
- **Node_Plugin**: The CSI node plugin component that handles volume attachment and mounting on nodes

## Requirements

### Requirement 1

**User Story:** As a Kubernetes administrator, I want to deploy the Emma CSI driver to my cluster, so that I can use Emma.ms volumes for persistent storage.

#### Acceptance Criteria

1. THE CSI_Driver SHALL be deployable as a Kubernetes DaemonSet for the Node_Plugin component
2. THE CSI_Driver SHALL be deployable as a Kubernetes StatefulSet for the Controller component
3. WHEN deploying THE CSI_Driver, THE system SHALL require Emma.ms API credentials (clientId and clientSecret)
4. THE CSI_Driver SHALL authenticate with Emma_API using Bearer token authentication
5. THE CSI_Driver SHALL automatically refresh access tokens before expiration

### Requirement 2

**User Story:** As a Kubernetes user, I want to dynamically provision volumes through PersistentVolumeClaims, so that my applications can request storage without manual intervention.

#### Acceptance Criteria

1. WHEN a PersistentVolumeClaim is created with an Emma StorageClass, THE Controller SHALL create a Volume via Emma_API
2. THE Controller SHALL support volume size specifications in gigabytes
3. THE Controller SHALL support volume type parameters (ssd, ssd-plus, hdd)
4. THE Controller SHALL select an appropriate data center based on StorageClass parameters
5. WHEN volume creation succeeds, THE Controller SHALL create a corresponding PersistentVolume in Kubernetes

### Requirement 3

**User Story:** As a Kubernetes user, I want volumes to be automatically attached to nodes when pods are scheduled, so that my applications can access their persistent data.

#### Acceptance Criteria

1. WHEN a pod using an Emma volume is scheduled to a Node, THE Controller SHALL attach the Volume to that Node via Emma_API
2. THE Controller SHALL wait for the Volume status to become ACTIVE before reporting attachment success
3. THE Controller SHALL handle attachment failures with appropriate error messages
4. THE Controller SHALL support only one pod accessing a volume at a time (ReadWriteOnce access mode)
5. WHEN attachment completes, THE Node_Plugin SHALL make the volume available to the pod

### Requirement 4

**User Story:** As a Kubernetes user, I want volumes to be automatically detached when pods are deleted, so that volumes can be reused by other pods.

#### Acceptance Criteria

1. WHEN a pod using an Emma volume is deleted, THE Controller SHALL detach the Volume from the Node via Emma_API
2. THE Controller SHALL wait for detachment to complete before marking the operation successful
3. IF detachment fails, THE Controller SHALL retry the operation with exponential backoff
4. THE Controller SHALL handle cases where the Volume is already detached gracefully
5. WHEN detachment completes, THE Volume SHALL be available for attachment to other Nodes

### Requirement 5

**User Story:** As a Kubernetes user, I want to delete PersistentVolumeClaims and have the underlying volumes cleaned up, so that I don't incur unnecessary storage costs.

#### Acceptance Criteria

1. WHEN a PersistentVolumeClaim with reclaimPolicy Delete is removed, THE Controller SHALL delete the Volume via Emma_API
2. THE Controller SHALL ensure the Volume is detached before attempting deletion
3. IF the Volume is attached, THE Controller SHALL detach it before deletion
4. THE Controller SHALL handle deletion of non-existent volumes gracefully
5. WHEN deletion completes, THE Controller SHALL remove the PersistentVolume from Kubernetes

### Requirement 6

**User Story:** As a Kubernetes user, I want to expand existing volumes when my storage needs grow, so that I can increase capacity without recreating volumes.

#### Acceptance Criteria

1. WHEN a PersistentVolumeClaim size is increased, THE Controller SHALL resize the Volume via Emma_API
2. THE Controller SHALL only allow volume size increases (not decreases)
3. THE Controller SHALL verify the new size is supported by the Emma_API volume configurations
4. WHEN resize completes, THE Node_Plugin SHALL expand the filesystem on the volume
5. THE Controller SHALL update the PersistentVolume capacity in Kubernetes

### Requirement 7

**User Story:** As a Kubernetes user, I want volumes to be formatted and mounted on nodes, so that my applications can read and write data.

#### Acceptance Criteria

1. WHEN a volume is attached to a Node, THE Node_Plugin SHALL format the volume with the specified filesystem type
2. THE Node_Plugin SHALL support ext4 and xfs filesystem types
3. THE Node_Plugin SHALL mount the formatted volume to the pod's mount path
4. WHEN a pod is deleted, THE Node_Plugin SHALL unmount the volume from the Node
5. THE Node_Plugin SHALL handle mount failures with appropriate error messages

### Requirement 8

**User Story:** As a Kubernetes administrator, I want the CSI driver to provide detailed logs and metrics, so that I can troubleshoot issues and monitor performance.

#### Acceptance Criteria

1. THE CSI_Driver SHALL log all API requests to Emma_API with request and response details
2. THE CSI_Driver SHALL log volume lifecycle events (create, attach, detach, delete, resize)
3. THE CSI_Driver SHALL expose Prometheus metrics for operation counts and latencies
4. THE CSI_Driver SHALL log errors with sufficient context for troubleshooting
5. THE CSI_Driver SHALL support configurable log levels (debug, info, warn, error)

### Requirement 9

**User Story:** As a Kubernetes administrator, I want the CSI driver to handle API failures gracefully, so that temporary issues don't cause permanent failures.

#### Acceptance Criteria

1. WHEN Emma_API returns a 5xx error, THE CSI_Driver SHALL retry the operation with exponential backoff
2. WHEN Emma_API returns a 429 rate limit error, THE CSI_Driver SHALL wait and retry the operation
3. THE CSI_Driver SHALL implement a maximum retry count of 5 attempts
4. WHEN all retries are exhausted, THE CSI_Driver SHALL return an error to Kubernetes
5. THE CSI_Driver SHALL handle network timeouts with appropriate retry logic

### Requirement 10

**User Story:** As a Kubernetes administrator, I want to configure the CSI driver with Emma.ms data center preferences, so that volumes are created in the appropriate regions.

#### Acceptance Criteria

1. THE CSI_Driver SHALL support data center ID specification in StorageClass parameters
2. THE CSI_Driver SHALL validate that the specified data center exists via Emma_API
3. WHEN no data center is specified, THE CSI_Driver SHALL use a default data center from configuration
4. THE CSI_Driver SHALL support multiple StorageClasses for different data centers
5. THE CSI_Driver SHALL ensure volumes are created in the same data center as the requesting Node
