package driver

import (
	"context"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/emma-csi-driver/pkg/mount"
)

// NodeService implements the CSI Node service
type NodeService struct {
	driver  *Driver
	mounter mount.Mounter
}

// NewNodeService creates a new node service
func NewNodeService(driver *Driver) *NodeService {
	return &NodeService{
		driver:  driver,
		mounter: mount.NewMounter(),
	}
}

// NodeStageVolume stages a volume
func (s *NodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(4).Infof("NodeStageVolume called with request: %+v", req)

	// Validate request
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}

	volumeCapability := req.GetVolumeCapability()
	if volumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability is required")
	}

	// Get filesystem type from volume capability
	fsType := "ext4" // default
	if mnt := volumeCapability.GetMount(); mnt != nil {
		if mnt.FsType != "" {
			fsType = mnt.FsType
		}
	}

	// Validate filesystem type
	if fsType != "ext4" && fsType != "xfs" {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported filesystem type: %s", fsType)
	}

	// Check if already staged
	notMnt, err := s.mounter.IsLikelyNotMountPoint(stagingTargetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to check if %s is a mount point: %v", stagingTargetPath, err)
	}

	if !notMnt {
		klog.V(4).Infof("Volume %s is already staged at %s", volumeID, stagingTargetPath)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// Discover the device path for the volume
	devicePath, err := s.mounter.GetDevicePath(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find device for volume %s: %v", volumeID, err)
	}

	klog.V(4).Infof("Found device %s for volume %s", devicePath, volumeID)

	// Get mount options
	mountOptions := []string{}
	if mnt := volumeCapability.GetMount(); mnt != nil {
		mountOptions = mnt.MountFlags
	}

	// Format and mount the device
	klog.V(4).Infof("Formatting and mounting device %s to %s with fstype %s", devicePath, stagingTargetPath, fsType)
	if err := s.mounter.FormatAndMount(devicePath, stagingTargetPath, fsType, mountOptions); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to format and mount device: %v", err)
	}

	klog.Infof("Successfully staged volume %s at %s", volumeID, stagingTargetPath)
	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume unstages a volume
func (s *NodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(4).Infof("NodeUnstageVolume called with request: %+v", req)

	// Validate request
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}

	// Check if the path is a mount point
	notMnt, err := s.mounter.IsLikelyNotMountPoint(stagingTargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(4).Infof("Staging path %s does not exist, nothing to unstage", stagingTargetPath)
			return &csi.NodeUnstageVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to check if %s is a mount point: %v", stagingTargetPath, err)
	}

	if notMnt {
		klog.V(4).Infof("Staging path %s is not a mount point, nothing to unstage", stagingTargetPath)
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	// Unmount the volume
	klog.V(4).Infof("Unmounting volume %s from %s", volumeID, stagingTargetPath)
	if err := s.mounter.Unmount(stagingTargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount volume: %v", err)
	}

	// Clean up the staging directory
	klog.V(4).Infof("Removing staging directory %s", stagingTargetPath)
	if err := os.Remove(stagingTargetPath); err != nil && !os.IsNotExist(err) {
		klog.Warningf("Failed to remove staging directory %s: %v", stagingTargetPath, err)
		// Don't fail the operation if we can't remove the directory
	}

	klog.Infof("Successfully unstaged volume %s from %s", volumeID, stagingTargetPath)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodePublishVolume publishes a volume
func (s *NodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(4).Infof("NodePublishVolume called with request: %+v", req)

	// Validate request
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}

	volumeCapability := req.GetVolumeCapability()
	if volumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability is required")
	}

	// Check if already published
	notMnt, err := s.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to check if %s is a mount point: %v", targetPath, err)
	}

	if !notMnt {
		klog.V(4).Infof("Volume %s is already published at %s", volumeID, targetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	// Get mount options
	mountOptions := []string{"bind"}
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	if mnt := volumeCapability.GetMount(); mnt != nil {
		mountOptions = append(mountOptions, mnt.MountFlags...)
	}

	// Bind mount from staging path to target path
	klog.V(4).Infof("Bind mounting from %s to %s with options %v", stagingTargetPath, targetPath, mountOptions)
	if err := s.mounter.Mount(stagingTargetPath, targetPath, "", mountOptions); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to bind mount volume: %v", err)
	}

	klog.Infof("Successfully published volume %s at %s", volumeID, targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unpublishes a volume
func (s *NodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(4).Infof("NodeUnpublishVolume called with request: %+v", req)

	// Validate request
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}

	// Check if the path is a mount point
	notMnt, err := s.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(4).Infof("Target path %s does not exist, nothing to unpublish", targetPath)
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to check if %s is a mount point: %v", targetPath, err)
	}

	if notMnt {
		klog.V(4).Infof("Target path %s is not a mount point, nothing to unpublish", targetPath)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	// Unmount the volume
	klog.V(4).Infof("Unmounting volume %s from %s", volumeID, targetPath)
	if err := s.mounter.Unmount(targetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount volume: %v", err)
	}

	// Clean up the target directory
	klog.V(4).Infof("Removing target directory %s", targetPath)
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		klog.Warningf("Failed to remove target directory %s: %v", targetPath, err)
		// Don't fail the operation if we can't remove the directory
	}

	klog.Infof("Successfully unpublished volume %s from %s", volumeID, targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetVolumeStats gets volume statistics
func (s *NodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(4).Infof("NodeGetVolumeStats called with request: %+v", req)

	// Validate request
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	volumePath := req.GetVolumePath()
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is required")
	}

	// Check if path exists
	_, err := os.Stat(volumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "volume path %s does not exist", volumePath)
		}
		return nil, status.Errorf(codes.Internal, "failed to stat volume path: %v", err)
	}

	// Get volume statistics
	stats, err := s.mounter.GetVolumeStats(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get volume stats: %v", err)
	}

	klog.V(4).Infof("Volume %s stats: total=%d, used=%d, available=%d",
		volumeID, stats.TotalBytes, stats.UsedBytes, stats.AvailableBytes)

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Unit:      csi.VolumeUsage_BYTES,
				Available: stats.AvailableBytes,
				Total:     stats.TotalBytes,
				Used:      stats.UsedBytes,
			},
			{
				Unit:      csi.VolumeUsage_INODES,
				Available: stats.AvailableInodes,
				Total:     stats.TotalInodes,
				Used:      stats.UsedInodes,
			},
		},
	}, nil
}

// NodeExpandVolume expands a volume on the node
func (s *NodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.V(4).Infof("NodeExpandVolume called with request: %+v", req)

	// Validate request
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	volumePath := req.GetVolumePath()
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is required")
	}

	volumeCapability := req.GetVolumeCapability()
	if volumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability is required")
	}

	// Get filesystem type
	fsType := "ext4" // default
	if mnt := volumeCapability.GetMount(); mnt != nil {
		if mnt.FsType != "" {
			fsType = mnt.FsType
		}
	}

	// Validate filesystem type
	if fsType != "ext4" && fsType != "xfs" {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported filesystem type: %s", fsType)
	}

	klog.V(4).Infof("Expanding filesystem on volume %s at %s (fstype: %s)", volumeID, volumePath, fsType)

	// For ext4, we need the device path
	// For xfs, we need the mount path
	if fsType == "ext4" {
		// Get device path from volume ID
		devicePath, err := s.mounter.GetDevicePath(volumeID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to find device for volume %s: %v", volumeID, err)
		}

		if err := s.mounter.ResizeFilesystem(devicePath, fsType); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to resize filesystem: %v", err)
		}
	} else if fsType == "xfs" {
		// For xfs, use the mount path
		if err := s.mounter.(*mount.LinuxMounter).ResizeFilesystemAtPath(volumePath, fsType); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to resize filesystem: %v", err)
		}
	}

	klog.Infof("Successfully expanded filesystem on volume %s", volumeID)

	// Return the new capacity if provided
	capacity := req.GetCapacityRange().GetRequiredBytes()
	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: capacity,
	}, nil
}

// NodeGetCapabilities returns node capabilities
func (s *NodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Info("NodeGetCapabilities called")

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
		},
	}, nil
}

// NodeGetInfo returns node information
func (s *NodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Info("NodeGetInfo called")

	// Get datacenter information from environment or metadata
	// In Emma.ms, the datacenter can be retrieved from VM metadata
	// For now, we'll use an environment variable
	datacenterID := os.Getenv("EMMA_DATACENTER_ID")

	response := &csi.NodeGetInfoResponse{
		NodeId: s.driver.nodeID,
		// Maximum number of volumes that can be attached to this node
		MaxVolumesPerNode: 16, // Emma.ms platform limit
	}

	// Add topology information if datacenter is available
	if datacenterID != "" {
		response.AccessibleTopology = &csi.Topology{
			Segments: map[string]string{
				"topology.csi.emma.ms/datacenter": datacenterID,
			},
		}
		klog.V(4).Infof("Node %s is in datacenter %s", s.driver.nodeID, datacenterID)
	}

	return response, nil
}
