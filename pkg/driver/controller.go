package driver

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/emma-csi-driver/pkg/emma"
	"github.com/emma-csi-driver/pkg/logging"
	"github.com/emma-csi-driver/pkg/metrics"
)

const (
	// Volume parameters
	paramType         = "type"
	paramDataCenterID = "dataCenterId"
	paramFSType       = "fsType"

	// Default values
	defaultVolumeType = "ssd"
	defaultFSType     = "ext4"

	// Timeouts
	volumeCreateTimeout = 5 * time.Minute
	volumeAttachTimeout = 5 * time.Minute
	volumeDetachTimeout = 5 * time.Minute
	volumeResizeTimeout = 5 * time.Minute

	// Size constants
	bytesPerGB = 1024 * 1024 * 1024
)

// ControllerService implements the CSI Controller service
type ControllerService struct {
	driver     *Driver
	emmaClient *emma.Client
	logger     *logging.Logger
}

// NewControllerService creates a new controller service
func NewControllerService(driver *Driver, emmaClient *emma.Client) *ControllerService {
	return &ControllerService{
		driver:     driver,
		emmaClient: emmaClient,
		logger:     logging.NewLogger("controller-service"),
	}
}

// CreateVolume creates a new volume
func (s *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	timer := metrics.NewOperationTimer("CreateVolume")
	opLog := s.logger.WithOperation("CreateVolume").WithField("volumeName", req.GetName())
	
	opLog.Info("CreateVolume request received")
	klog.V(4).Infof("CreateVolume called with request: %+v", req)

	// Validate request
	if req.GetName() == "" {
		timer.ObserveError()
		opLog.Error("Volume name is required", nil)
		return nil, status.Error(codes.InvalidArgument, "volume name is required")
	}

	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		timer.ObserveError()
		opLog.Error("Volume capabilities are required", nil)
		return nil, status.Error(codes.InvalidArgument, "volume capabilities are required")
	}

	// Validate volume capabilities
	if err := s.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		timer.ObserveError()
		opLog.Error("Invalid volume capabilities", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capabilities: %v", err)
	}

	// Parse capacity (required range in bytes)
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	if capacityBytes == 0 {
		capacityBytes = req.GetCapacityRange().GetLimitBytes()
	}
	if capacityBytes == 0 {
		timer.ObserveError()
		opLog.Error("Volume capacity is required", nil)
		return nil, status.Error(codes.InvalidArgument, "volume capacity is required")
	}

	// Convert to GB (round up)
	requestedGB := int32((capacityBytes + bytesPerGB - 1) / bytesPerGB)
	if requestedGB < 1 {
		requestedGB = 1
	}

	// Emma requires disk sizes to be powers of 2 (1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048)
	// Round up to the nearest power of 2
	sizeGB := roundUpToPowerOfTwo(requestedGB)
	
	if sizeGB != requestedGB {
		opLog.WithField("requestedGB", requestedGB).WithField("actualGB", sizeGB).Info("Rounded volume size to nearest power of 2")
		klog.Infof("Volume size rounded: %dGB → %dGB (Emma requires powers of 2)", requestedGB, sizeGB)
	}

	// Parse StorageClass parameters
	params := req.GetParameters()
	volumeType := defaultVolumeType
	if t, ok := params[paramType]; ok && t != "" {
		volumeType = t
	}

	dataCenterID := ""
	if dc, ok := params[paramDataCenterID]; ok && dc != "" {
		dataCenterID = dc
	}
	if dataCenterID == "" {
		timer.ObserveError()
		opLog.Error("DataCenter ID parameter is required", nil)
		return nil, status.Error(codes.InvalidArgument, "dataCenterId parameter is required")
	}

	fsType := defaultFSType
	if fs, ok := params[paramFSType]; ok && fs != "" {
		fsType = fs
	}

	// Validate filesystem type
	if fsType != "ext4" && fsType != "xfs" {
		timer.ObserveError()
		opLog.WithField("fsType", fsType).Error("Unsupported filesystem type", nil)
		return nil, status.Errorf(codes.InvalidArgument, "unsupported filesystem type: %s (supported: ext4, xfs)", fsType)
	}

	// Validate data center
	if err := s.emmaClient.ValidateDataCenter(ctx, dataCenterID); err != nil {
		timer.ObserveError()
		opLog.WithField("dataCenterId", dataCenterID).Error("Invalid data center", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid data center: %v", err)
	}

	opLog.WithField("sizeGB", sizeGB).
		WithField("volumeType", volumeType).
		WithField("dataCenterId", dataCenterID).
		WithField("fsType", fsType).
		Info("Creating volume via Emma API")

	// Create volume via Emma API
	volume, err := s.emmaClient.CreateVolume(ctx, req.GetName(), sizeGB, volumeType, dataCenterID)
	if err != nil {
		timer.ObserveError()
		opLog.Error("Failed to create volume via Emma API", err)
		return nil, status.Errorf(codes.Internal, "failed to create volume: %v", err)
	}

	opLog.WithVolumeID(strconv.Itoa(int(volume.ID))).Info("Volume created, waiting for AVAILABLE status")

	// Wait for volume to become AVAILABLE
	if err := s.emmaClient.WaitForVolumeStatus(ctx, volume.ID, "AVAILABLE", volumeCreateTimeout); err != nil {
		// Try to clean up the volume
		_ = s.emmaClient.DeleteVolume(ctx, volume.ID)
		timer.ObserveError()
		opLog.WithVolumeID(strconv.Itoa(int(volume.ID))).Error("Volume creation timeout", err)
		return nil, status.Errorf(codes.Internal, "volume creation timeout: %v", err)
	}

	timer.ObserveSuccess()
	opLog.WithVolumeID(strconv.Itoa(int(volume.ID))).Complete("Volume created successfully")

	// Build CSI volume response
	csiVolume := &csi.Volume{
		VolumeId:      strconv.Itoa(int(volume.ID)),
		CapacityBytes: int64(volume.SizeGB) * bytesPerGB,
		VolumeContext: map[string]string{
			paramType:         volume.Type,
			paramDataCenterID: volume.DataCenterID,
			paramFSType:       fsType,
		},
	}

	return &csi.CreateVolumeResponse{
		Volume: csiVolume,
	}, nil
}

// DeleteVolume deletes a volume
func (s *ControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	timer := metrics.NewOperationTimer("DeleteVolume")
	opLog := s.logger.WithOperation("DeleteVolume").WithVolumeID(req.GetVolumeId())
	
	opLog.Info("DeleteVolume request received")
	klog.V(4).Infof("DeleteVolume called with request: %+v", req)

	// Validate request
	if req.GetVolumeId() == "" {
		timer.ObserveError()
		opLog.Error("Volume ID is required", nil)
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	// Parse volume ID
	volumeID, err := strconv.ParseInt(req.GetVolumeId(), 10, 32)
	if err != nil {
		timer.ObserveError()
		opLog.Error("Invalid volume ID", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	opLog.Info("Deleting volume")

	// Check if volume exists
	volume, err := s.emmaClient.GetVolume(ctx, int32(volumeID))
	if err != nil {
		// If volume doesn't exist, consider it already deleted
		if status.Code(err) == codes.NotFound {
			timer.ObserveSuccess()
			opLog.Info("Volume not found, considering it already deleted")
			return &csi.DeleteVolumeResponse{}, nil
		}
		timer.ObserveError()
		opLog.Error("Failed to get volume", err)
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	}

	// Ensure volume is detached
	if volume.AttachedToID != nil {
		opLog.WithField("vmId", *volume.AttachedToID).Info("Volume is attached, detaching first")
		
		// Detach volume
		if err := s.emmaClient.DetachVolume(ctx, *volume.AttachedToID, int32(volumeID)); err != nil {
			timer.ObserveError()
			opLog.Error("Failed to detach volume before deletion", err)
			return nil, status.Errorf(codes.Internal, "failed to detach volume before deletion: %v", err)
		}

		// Wait for detachment
		if err := s.emmaClient.WaitForVolumeDetachment(ctx, int32(volumeID), volumeDetachTimeout); err != nil {
			timer.ObserveError()
			opLog.Error("Volume detachment timeout", err)
			return nil, status.Errorf(codes.Internal, "volume detachment timeout: %v", err)
		}
	}

	// Delete volume via Emma API
	if err := s.emmaClient.DeleteVolume(ctx, int32(volumeID)); err != nil {
		timer.ObserveError()
		opLog.Error("Failed to delete volume via Emma API", err)
		return nil, status.Errorf(codes.Internal, "failed to delete volume: %v", err)
	}

	timer.ObserveSuccess()
	opLog.Complete("Volume deleted successfully")

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume attaches a volume to a node
func (s *ControllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	timer := metrics.NewOperationTimer("ControllerPublishVolume")
	attachTimer := time.Now()
	opLog := s.logger.WithOperation("ControllerPublishVolume").
		WithVolumeID(req.GetVolumeId()).
		WithNodeID(req.GetNodeId())
	
	opLog.Info("ControllerPublishVolume request received")
	klog.V(4).Infof("ControllerPublishVolume called with request: %+v", req)

	// Validate request
	if req.GetVolumeId() == "" {
		timer.ObserveError()
		opLog.Error("Volume ID is required", nil)
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	if req.GetNodeId() == "" {
		timer.ObserveError()
		opLog.Error("Node ID is required", nil)
		return nil, status.Error(codes.InvalidArgument, "node ID is required")
	}

	if req.GetVolumeCapability() == nil {
		timer.ObserveError()
		opLog.Error("Volume capability is required", nil)
		return nil, status.Error(codes.InvalidArgument, "volume capability is required")
	}

	// Validate volume capability
	if err := s.validateVolumeCapabilities([]*csi.VolumeCapability{req.GetVolumeCapability()}); err != nil {
		timer.ObserveError()
		opLog.Error("Invalid volume capability", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume capability: %v", err)
	}

	// Parse volume ID and node ID (VM ID)
	volumeID, err := strconv.ParseInt(req.GetVolumeId(), 10, 32)
	if err != nil {
		timer.ObserveError()
		opLog.Error("Invalid volume ID", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	// Resolve node ID to VM ID (handles both integer VM IDs and node names)
	vmID, err := s.resolveNodeIDToVMID(ctx, req.GetNodeId())
	if err != nil {
		timer.ObserveError()
		opLog.WithField("nodeId", req.GetNodeId()).Error("Failed to resolve node ID to VM ID", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to resolve node ID: %v", err)
	}

	opLog.WithField("vmId", vmID).Info("Attaching volume to node")

	// Check if volume is already attached to this node
	volume, err := s.emmaClient.GetVolume(ctx, int32(volumeID))
	if err != nil {
		timer.ObserveError()
		opLog.Error("Failed to get volume", err)
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	}

	if volume.AttachedToID != nil {
		if *volume.AttachedToID == int32(vmID) {
			timer.ObserveSuccess()
			opLog.Info("Volume is already attached to this node")
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					"devicePath": fmt.Sprintf("/dev/disk/by-id/virtio-%d", volumeID),
				},
			}, nil
		}
		timer.ObserveError()
		opLog.WithField("attachedToVmId", *volume.AttachedToID).Error("Volume is already attached to another node", nil)
		return nil, status.Errorf(codes.FailedPrecondition, "volume %d is already attached to another node", volumeID)
	}

	// Attach volume to VM via Emma API
	opLog.Info("Initiating volume attach via Emma API")
	if err := s.emmaClient.AttachVolume(ctx, int32(vmID), int32(volumeID)); err != nil {
		timer.ObserveError()
		opLog.Error("Failed to attach volume via Emma API", err)
		return nil, status.Errorf(codes.Internal, "failed to attach volume: %v", err)
	}

	// Wait for attachment to complete
	opLog.Info("Waiting for volume attachment to complete")
	if err := s.emmaClient.WaitForVolumeAttachment(ctx, int32(volumeID), int32(vmID), volumeAttachTimeout); err != nil {
		timer.ObserveError()
		opLog.Error("Volume attachment timeout", err)
		return nil, status.Errorf(codes.Internal, "volume attachment timeout: %v", err)
	}

	// Record attach duration
	metrics.RecordVolumeAttach(time.Since(attachTimer))
	timer.ObserveSuccess()
	opLog.Complete("Volume attached successfully")

	// Return device path information
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"devicePath": fmt.Sprintf("/dev/disk/by-id/virtio-%d", volumeID),
		},
	}, nil
}

// ControllerUnpublishVolume detaches a volume from a node
func (s *ControllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	timer := metrics.NewOperationTimer("ControllerUnpublishVolume")
	detachTimer := time.Now()
	opLog := s.logger.WithOperation("ControllerUnpublishVolume").
		WithVolumeID(req.GetVolumeId()).
		WithNodeID(req.GetNodeId())
	
	opLog.Info("ControllerUnpublishVolume request received")
	klog.V(4).Infof("ControllerUnpublishVolume called with request: %+v", req)

	// Validate request
	if req.GetVolumeId() == "" {
		timer.ObserveError()
		opLog.Error("Volume ID is required", nil)
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	if req.GetNodeId() == "" {
		timer.ObserveError()
		opLog.Error("Node ID is required", nil)
		return nil, status.Error(codes.InvalidArgument, "node ID is required")
	}

	// Parse volume ID and node ID (VM ID)
	volumeID, err := strconv.ParseInt(req.GetVolumeId(), 10, 32)
	if err != nil {
		timer.ObserveError()
		opLog.Error("Invalid volume ID", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	// Resolve node ID to VM ID (handles both integer VM IDs and node names)
	vmID, err := s.resolveNodeIDToVMID(ctx, req.GetNodeId())
	if err != nil {
		timer.ObserveError()
		opLog.WithField("nodeId", req.GetNodeId()).Error("Failed to resolve node ID to VM ID", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to resolve node ID: %v", err)
	}

	opLog.WithField("vmId", vmID).Info("Detaching volume from node")

	// Check if volume is already detached
	volume, err := s.emmaClient.GetVolume(ctx, int32(volumeID))
	if err != nil {
		// If volume doesn't exist, consider it already detached
		if status.Code(err) == codes.NotFound {
			timer.ObserveSuccess()
			opLog.Info("Volume not found, considering it already detached")
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		timer.ObserveError()
		opLog.Error("Failed to get volume", err)
		return nil, status.Errorf(codes.Internal, "failed to get volume: %v", err)
	}

	if volume.AttachedToID == nil {
		timer.ObserveSuccess()
		opLog.Info("Volume is already detached")
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	if *volume.AttachedToID != int32(vmID) {
		timer.ObserveSuccess()
		opLog.WithField("attachedToVmId", *volume.AttachedToID).Info("Volume is not attached to this node, skipping detachment")
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	// Detach volume from VM via Emma API
	opLog.Info("Initiating volume detach via Emma API")
	if err := s.emmaClient.DetachVolume(ctx, int32(vmID), int32(volumeID)); err != nil {
		timer.ObserveError()
		opLog.Error("Failed to detach volume via Emma API", err)
		return nil, status.Errorf(codes.Internal, "failed to detach volume: %v", err)
	}

	// Wait for detachment to complete
	opLog.Info("Waiting for volume detachment to complete")
	if err := s.emmaClient.WaitForVolumeDetachment(ctx, int32(volumeID), volumeDetachTimeout); err != nil {
		timer.ObserveError()
		opLog.Error("Volume detachment timeout", err)
		return nil, status.Errorf(codes.Internal, "volume detachment timeout: %v", err)
	}

	// Record detach duration
	metrics.RecordVolumeDetach(time.Since(detachTimer))
	timer.ObserveSuccess()
	opLog.Complete("Volume detached successfully")

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities validates volume capabilities
func (s *ControllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities called with request: %+v", req)

	// Validate request
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities are required")
	}

	// Parse volume ID
	volumeID, err := strconv.ParseInt(req.GetVolumeId(), 10, 32)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	// Check if volume exists
	_, err = s.emmaClient.GetVolume(ctx, int32(volumeID))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "volume %d not found: %v", volumeID, err)
	}

	// Validate capabilities
	if err := s.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		klog.V(4).Infof("Volume capabilities validation failed: %v", err)
		return &csi.ValidateVolumeCapabilitiesResponse{
			Message: err.Error(),
		}, nil
	}

	klog.V(4).Infof("Volume %d capabilities validated successfully", volumeID)

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
			VolumeContext:      req.GetVolumeContext(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

// ListVolumes lists volumes
func (s *ControllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).Infof("ListVolumes called with request: %+v", req)

	// List volumes via Emma API
	volumes, err := s.emmaClient.ListVolumes(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list volumes: %v", err)
	}

	// Convert to CSI volume entries
	entries := make([]*csi.ListVolumesResponse_Entry, 0, len(volumes))
	for _, vol := range volumes {
		entry := &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      strconv.Itoa(int(vol.ID)),
				CapacityBytes: int64(vol.SizeGB) * bytesPerGB,
				VolumeContext: map[string]string{
					paramType:         vol.Type,
					paramDataCenterID: vol.DataCenterID,
				},
			},
		}

		// Add status information if available
		if vol.Status != "" {
			entry.Status = &csi.ListVolumesResponse_VolumeStatus{
				VolumeCondition: &csi.VolumeCondition{
					Message: fmt.Sprintf("Status: %s", vol.Status),
				},
			}
		}

		entries = append(entries, entry)
	}

	klog.V(4).Infof("Listed %d volumes", len(entries))

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

// GetCapacity returns available capacity
func (s *ControllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).Infof("GetCapacity called with request: %+v", req)
	
	// Not supported
	return nil, status.Error(codes.Unimplemented, "GetCapacity not supported")
}

// ControllerGetCapabilities returns controller capabilities
func (s *ControllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Info("ControllerGetCapabilities called")

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
					},
				},
			},
		},
	}, nil
}

// CreateSnapshot creates a snapshot
func (s *ControllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).Infof("CreateSnapshot called with request: %+v", req)
	
	// Not supported
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot not supported")
}

// DeleteSnapshot deletes a snapshot
func (s *ControllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(4).Infof("DeleteSnapshot called with request: %+v", req)
	
	// Not supported
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot not supported")
}

// ListSnapshots lists snapshots
func (s *ControllerService) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(4).Infof("ListSnapshots called with request: %+v", req)
	
	// Not supported
	return nil, status.Error(codes.Unimplemented, "ListSnapshots not supported")
}

// ControllerExpandVolume expands a volume
func (s *ControllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(4).Infof("ControllerExpandVolume called with request: %+v", req)

	// Validate request
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "capacity range is required")
	}

	// Parse volume ID
	volumeID, err := strconv.ParseInt(req.GetVolumeId(), 10, 32)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume ID: %v", err)
	}

	// Get current volume
	volume, err := s.emmaClient.GetVolume(ctx, int32(volumeID))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "volume %d not found: %v", volumeID, err)
	}

	// Parse new capacity
	newCapacityBytes := req.GetCapacityRange().GetRequiredBytes()
	if newCapacityBytes == 0 {
		newCapacityBytes = req.GetCapacityRange().GetLimitBytes()
	}
	if newCapacityBytes == 0 {
		return nil, status.Error(codes.InvalidArgument, "new capacity is required")
	}

	// Convert to GB (round up)
	newSizeGB := int32((newCapacityBytes + bytesPerGB - 1) / bytesPerGB)
	if newSizeGB < 1 {
		newSizeGB = 1
	}

	// Validate new size is larger than current size
	if newSizeGB <= volume.SizeGB {
		return nil, status.Errorf(codes.InvalidArgument, "new size (%dGB) must be larger than current size (%dGB)", newSizeGB, volume.SizeGB)
	}

	klog.V(4).Infof("Expanding volume %d from %dGB to %dGB", volumeID, volume.SizeGB, newSizeGB)

	// Resize volume via Emma API
	if err := s.emmaClient.ResizeVolume(ctx, int32(volumeID), newSizeGB); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize volume: %v", err)
	}

	// Wait for resize to complete (volume should return to AVAILABLE or ACTIVE state)
	targetStatus := "AVAILABLE"
	if volume.AttachedToID != nil {
		targetStatus = "ACTIVE"
	}

	if err := s.emmaClient.WaitForVolumeStatus(ctx, int32(volumeID), targetStatus, volumeResizeTimeout); err != nil {
		return nil, status.Errorf(codes.Internal, "volume resize timeout: %v", err)
	}

	klog.V(4).Infof("Volume %d expanded successfully to %dGB", volumeID, newSizeGB)

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         int64(newSizeGB) * bytesPerGB,
		NodeExpansionRequired: true, // Node needs to expand the filesystem
	}, nil
}

// ControllerGetVolume gets volume information
func (s *ControllerService) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(4).Infof("ControllerGetVolume called with request: %+v", req)
	
	// Not supported
	return nil, status.Error(codes.Unimplemented, "ControllerGetVolume not supported")
}

// ControllerModifyVolume modifies a volume (not supported)
func (s *ControllerService) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	klog.V(4).Infof("ControllerModifyVolume called with request: %+v", req)
	
	// Not supported
	return nil, status.Error(codes.Unimplemented, "ControllerModifyVolume not supported")
}

// validateVolumeCapabilities validates that the requested capabilities are supported
func (s *ControllerService) validateVolumeCapabilities(caps []*csi.VolumeCapability) error {
	for _, cap := range caps {
		// Validate access mode - only ReadWriteOnce is supported
		accessMode := cap.GetAccessMode()
		if accessMode == nil {
			return fmt.Errorf("access mode is required")
		}

		if accessMode.GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
			return fmt.Errorf("unsupported access mode: %v (only ReadWriteOnce is supported)", accessMode.GetMode())
		}

		// Validate access type (block or mount)
		accessType := cap.GetAccessType()
		if accessType == nil {
			return fmt.Errorf("access type is required")
		}

		// Both block and mount are supported
		switch accessType.(type) {
		case *csi.VolumeCapability_Block:
			// Block volumes are supported
		case *csi.VolumeCapability_Mount:
			// Mount volumes are supported
			mount := cap.GetMount()
			if mount != nil {
				fsType := mount.GetFsType()
				if fsType != "" && fsType != "ext4" && fsType != "xfs" {
					return fmt.Errorf("unsupported filesystem type: %s (supported: ext4, xfs)", fsType)
				}
			}
		default:
			return fmt.Errorf("unsupported access type: %T", accessType)
		}
	}

	return nil
}

// roundUpToPowerOfTwo rounds up a size to the nearest power of 2
// Emma requires disk sizes to be powers of 2: 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048 GB
func roundUpToPowerOfTwo(size int32) int32 {
	if size <= 0 {
		return 1
	}
	
	// If already a power of 2, return as is
	if size&(size-1) == 0 {
		return size
	}
	
	// Find the next power of 2
	power := int32(1)
	for power < size {
		power *= 2
	}
	
	// Cap at 2048 GB (Emma's maximum)
	if power > 2048 {
		return 2048
	}
	
	return power
}

// resolveNodeIDToVMID resolves a Kubernetes node ID (which may be a name) to an Emma VM ID
func (s *ControllerService) resolveNodeIDToVMID(ctx context.Context, nodeID string) (int32, error) {
	// First, try to parse as integer (direct VM ID)
	if vmID, err := strconv.ParseInt(nodeID, 10, 32); err == nil {
		return int32(vmID), nil
	}
	
	// If not an integer, treat as node name and look it up in Kubernetes clusters
	klog.V(4).Infof("Node ID '%s' is not a number, looking up node in Kubernetes clusters", nodeID)
	
	// Get all Kubernetes clusters
	clusters, err := s.emmaClient.ListKubernetesClusters(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list Kubernetes clusters: %w", err)
	}
	
	// Search through all clusters for a node with matching name
	for _, cluster := range clusters {
		klog.V(5).Infof("Searching cluster '%s' (ID: %d) for node '%s'", 
			cluster.GetName(), cluster.GetId(), nodeID)
		
		// Check all node groups in the cluster
		for _, nodeGroup := range cluster.GetNodeGroups() {
			klog.V(5).Infof("Checking node group '%s'", nodeGroup.GetName())
			
			// Check all nodes in the node group
			for _, node := range nodeGroup.GetNodes() {
				if node.GetName() == nodeID {
					vmID := node.GetId()
					klog.V(4).Infof("Found node '%s' with VM ID %d in cluster '%s'", 
						nodeID, vmID, cluster.GetName())
					return vmID, nil
				}
			}
		}
	}
	
	return 0, fmt.Errorf("node not found with name: %s", nodeID)
}
