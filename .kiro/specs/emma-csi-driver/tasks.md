# Implementation Plan

- [x] 1. Set up project structure and core interfaces
  - Create Go module with appropriate dependencies (CSI spec, gRPC, HTTP client)
  - Define project directory structure (cmd/, pkg/, deploy/)
  - Create main entry points for controller and node binaries
  - _Requirements: 1.1, 1.2_

- [x] 2. Implement Emma API client
  - [x] 2.1 Create authentication manager
    - Implement token issuance via /v1/issue-token endpoint
    - Implement token refresh via /v1/refresh-token endpoint
    - Add automatic token refresh before expiration
    - Add thread-safe token storage
    - _Requirements: 1.4, 1.5_

  - [x] 2.2 Implement HTTP client with retry logic
    - Create base HTTP client with timeout configuration
    - Implement exponential backoff retry mechanism
    - Add retry logic for 5xx and 429 errors
    - Implement maximum retry limit
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5_

  - [x] 2.3 Implement volume operations
    - Implement CreateVolume (POST /v1/volumes)
    - Implement GetVolume (GET /v1/volumes/{volumeId})
    - Implement ListVolumes (GET /v1/volumes)
    - Implement DeleteVolume (DELETE /v1/volumes/{volumeId})
    - Implement ResizeVolume (POST /v1/volumes/{volumeId}/actions with action=edit)
    - _Requirements: 2.1, 2.2, 2.3, 5.1, 6.1_

  - [x] 2.4 Implement VM operations for attach/detach
    - Implement AttachVolume (POST /v1/vms/{vmId}/actions with action=attach)
    - Implement DetachVolume (POST /v1/vms/{vmId}/actions with action=detach)
    - Implement GetVM (GET /v1/vms/{vmId})
    - Add volume attachment status polling
    - _Requirements: 3.1, 3.2, 4.1, 4.2_

  - [x] 2.5 Implement configuration operations
    - Implement GetDataCenters (GET /v1/data-centers)
    - Implement GetVolumeConfigs (GET /v1/volume-configs)
    - Add datacenter validation
    - _Requirements: 10.1, 10.2, 10.3_

- [x] 3. Implement CSI Identity Service
  - [x] 3.1 Create identity service implementation
    - Implement GetPluginInfo method
    - Implement GetPluginCapabilities method
    - Implement Probe method with Emma API health check
    - _Requirements: 1.1, 1.2_

- [x] 4. Implement CSI Controller Service
  - [x] 4.1 Implement CreateVolume
    - Parse StorageClass parameters (type, dataCenterId, fsType)
    - Validate volume size and type
    - Call Emma API to create volume
    - Wait for volume to become AVAILABLE
    - Return volume ID and metadata
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 10.4, 10.5_

  - [x] 4.2 Implement DeleteVolume
    - Check if volume exists
    - Ensure volume is detached
    - Call Emma API to delete volume
    - Handle already-deleted volumes gracefully
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

  - [x] 4.3 Implement ControllerPublishVolume (attach)
    - Get node VM ID from node metadata
    - Call Emma API to attach volume to VM
    - Poll for attachment completion
    - Return device path information
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x] 4.4 Implement ControllerUnpublishVolume (detach)
    - Call Emma API to detach volume from VM
    - Poll for detachment completion
    - Implement retry logic for detachment failures
    - Handle already-detached volumes
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [x] 4.5 Implement ControllerExpandVolume
    - Validate new size is larger than current size
    - Check new size against Emma volume configs
    - Call Emma API to resize volume
    - Wait for resize completion
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

  - [x] 4.6 Implement ValidateVolumeCapabilities
    - Validate access mode (ReadWriteOnce only)
    - Validate volume capabilities
    - Return validation result
    - _Requirements: 3.4_

  - [x] 4.7 Implement ListVolumes
    - Call Emma API to list volumes
    - Return paginated volume list
    - _Requirements: 2.1_

  - [x] 4.8 Implement ControllerGetCapabilities
    - Return supported controller capabilities
    - _Requirements: 1.1_

- [x] 5. Implement CSI Node Service
  - [x] 5.1 Implement NodeStageVolume
    - Discover attached volume device
    - Format volume with specified filesystem type (ext4/xfs)
    - Mount volume to staging path
    - _Requirements: 7.1, 7.2, 7.3_

  - [x] 5.2 Implement NodeUnstageVolume
    - Unmount volume from staging path
    - Clean up staging directory
    - _Requirements: 7.4_

  - [x] 5.3 Implement NodePublishVolume
    - Bind mount from staging path to target path
    - Set appropriate mount options
    - _Requirements: 7.3_

  - [x] 5.4 Implement NodeUnpublishVolume
    - Unmount from target path
    - Clean up target directory
    - _Requirements: 7.4_

  - [x] 5.5 Implement NodeExpandVolume
    - Resize filesystem after volume expansion
    - Support ext4 and xfs resize operations
    - _Requirements: 6.4, 6.5_

  - [x] 5.6 Implement NodeGetCapabilities
    - Return supported node capabilities
    - _Requirements: 1.1_

  - [x] 5.7 Implement NodeGetInfo
    - Return node ID (VM ID from Emma metadata)
    - Return topology information
    - _Requirements: 1.1, 10.5_

  - [x] 5.8 Implement NodeGetVolumeStats
    - Return volume usage statistics
    - _Requirements: 8.3_

- [x] 6. Implement logging and metrics
  - [x] 6.1 Set up structured logging
    - Configure log levels (debug, info, warn, error)
    - Implement JSON log formatting
    - Add contextual logging for operations
    - _Requirements: 8.1, 8.2, 8.4, 8.5_

  - [x] 6.2 Implement Prometheus metrics
    - Create metrics for operation counts and latencies
    - Create metrics for API requests
    - Create metrics for volume states
    - Expose metrics endpoint on port 8080
    - _Requirements: 8.3_

- [x] 7. Create Kubernetes deployment manifests
  - [x] 7.1 Create CSIDriver resource
    - Define driver name and capabilities
    - Configure attachment required
    - Configure pod info on mount
    - _Requirements: 1.1, 1.2_

  - [x] 7.2 Create controller deployment
    - Create StatefulSet for controller
    - Configure service account and RBAC
    - Add Emma API credentials secret volume
    - Configure resource limits
    - Add liveness and readiness probes
    - _Requirements: 1.2, 1.3_

  - [x] 7.3 Create node deployment
    - Create DaemonSet for node plugin
    - Configure service account and RBAC
    - Mount host paths for device and mount operations
    - Configure privileged security context
    - Add liveness probe
    - _Requirements: 1.1, 1.3_

  - [x] 7.4 Create RBAC resources
    - Create ServiceAccounts for controller and node
    - Create ClusterRoles with required permissions
    - Create ClusterRoleBindings
    - _Requirements: 1.1, 1.2_

  - [x] 7.5 Create StorageClass examples
    - Create StorageClass for SSD volumes
    - Create StorageClass for HDD volumes
    - Configure volume binding mode and expansion
    - _Requirements: 2.1, 2.2, 2.3, 10.1, 10.4_

  - [x] 7.6 Create Secret template
    - Create Secret template for Emma API credentials
    - Document credential requirements
    - _Requirements: 1.3_

  - [x] 7.7 Create ConfigMap for driver configuration
    - Add Emma API URL configuration
    - Add default datacenter configuration
    - Add log level configuration
    - _Requirements: 8.5, 10.3_

- [x] 8. Create documentation
  - [x] 8.1 Write installation guide
    - Document prerequisites
    - Document installation steps
    - Document configuration options
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 8.2 Write user guide
    - Document StorageClass creation
    - Document PVC creation examples
    - Document volume expansion
    - _Requirements: 2.1, 6.1_

  - [x] 8.3 Write troubleshooting guide
    - Document common issues and solutions
    - Document log analysis
    - Document metrics interpretation
    - _Requirements: 8.1, 8.2, 8.3_

  - [x] 8.4 Write API reference
    - Document Emma API integration
    - Document error handling
    - Document retry behavior
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5_

- [x] 9. Create build and release infrastructure
  - [x] 9.1 Create Dockerfile
    - Create multi-stage build for controller
    - Create multi-stage build for node plugin
    - Minimize image size
    - _Requirements: 1.1, 1.2_

  - [x] 9.2 Create build scripts
    - Create Makefile for building binaries
    - Create scripts for building container images
    - Create scripts for pushing images
    - _Requirements: 1.1, 1.2_

  - [x] 9.3 Create CI/CD pipeline
    - Set up automated testing
    - Set up automated builds
    - Set up automated releases
    - _Requirements: 1.1, 1.2_

- [x] 10. Write tests
  - [x] 10.1 Write unit tests for Emma API client
    - Test authentication manager
    - Test retry logic
    - Test volume operations
    - Test VM operations
    - _Requirements: 1.4, 1.5, 9.1, 9.2, 9.3_

  - [x] 10.2 Write unit tests for CSI services
    - Test controller service methods
    - Test node service methods
    - Test identity service methods
    - _Requirements: 2.1, 3.1, 4.1, 5.1, 6.1, 7.1_

  - [x] 10.3 Write integration tests
    - Test volume lifecycle against Emma API
    - Test concurrent operations
    - Test failure scenarios
    - _Requirements: 2.1, 3.1, 4.1, 5.1, 6.1_

  - [x] 10.4 Write end-to-end tests
    - Test PVC provisioning
    - Test pod mounting
    - Test volume expansion
    - Test cleanup
    - _Requirements: 2.1, 3.1, 4.1, 5.1, 6.1, 7.1_
