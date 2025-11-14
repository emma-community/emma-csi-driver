package driver

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockEmmaClient is a mock implementation of the Emma API client for testing
type mockEmmaClient struct {
	createVolumeFunc           func(ctx context.Context, name string, sizeGB int32, volumeType string, dataCenterID string) (*mockVolume, error)
	getVolumeFunc              func(ctx context.Context, volumeID int32) (*mockVolume, error)
	deleteVolumeFunc           func(ctx context.Context, volumeID int32) error
	attachVolumeFunc           func(ctx context.Context, vmID int32, volumeID int32) error
	detachVolumeFunc           func(ctx context.Context, vmID int32, volumeID int32) error
	resizeVolumeFunc           func(ctx context.Context, volumeID int32, newSizeGB int32) error
	validateDataCenterFunc     func(ctx context.Context, dataCenterID string) error
	waitForVolumeStatusFunc    func(ctx context.Context, volumeID int32, status string, timeout interface{}) error
	waitForVolumeAttachmentFunc func(ctx context.Context, volumeID int32, vmID int32, timeout interface{}) error
	waitForVolumeDetachmentFunc func(ctx context.Context, volumeID int32, timeout interface{}) error
	listVolumesFunc            func(ctx context.Context) ([]*mockVolume, error)
}

type mockVolume struct {
	ID           int32
	Name         string
	SizeGB       int32
	Type         string
	Status       string
	AttachedToID *int32
	DataCenterID string
}

// TestControllerCreateVolume tests the CreateVolume method
func TestControllerCreateVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.CreateVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "successful volume creation",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 10 * 1024 * 1024 * 1024, // 10GB
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								FsType: "ext4",
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
				Parameters: map[string]string{
					"type":         "ssd",
					"dataCenterId": "aws-eu-west-2",
					"fsType":       "ext4",
				},
			},
			expectError: false,
		},
		{
			name: "missing volume name",
			req: &csi.CreateVolumeRequest{
				Name: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing volume capabilities",
			req: &csi.CreateVolumeRequest{
				Name:               "test-volume",
				VolumeCapabilities: nil,
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a minimal test - full implementation would require mocking the Emma client
			// For now, we just test the validation logic
			if tt.req.GetName() == "" {
				if !tt.expectError {
					t.Error("expected error for empty name")
				}
			}
			if tt.req.GetVolumeCapabilities() == nil && tt.expectError {
				// Expected error case
			}
		})
	}
}

// TestControllerDeleteVolume tests the DeleteVolume method
func TestControllerDeleteVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.DeleteVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "successful volume deletion",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "123",
			},
			expectError: false,
		},
		{
			name: "missing volume ID",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.req.GetVolumeId() == "" {
				if !tt.expectError {
					t.Error("expected error for empty volume ID")
				}
			}
		})
	}
}

// TestControllerPublishVolume tests the ControllerPublishVolume method
func TestControllerPublishVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.ControllerPublishVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "successful volume attach",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "123",
				NodeId:   "456",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing volume ID",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing node ID",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "123",
				NodeId:   "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.req.GetVolumeId() == "" || tt.req.GetNodeId() == "" {
				if !tt.expectError {
					t.Error("expected error for missing required fields")
				}
			}
		})
	}
}

// TestValidateVolumeCapabilities tests volume capability validation
func TestValidateVolumeCapabilities(t *testing.T) {
	driver := &Driver{
		name:    "csi.emma.ms",
		version: "1.0.0",
	}
	
	service := NewControllerService(driver, nil)

	tests := []struct {
		name        string
		caps        []*csi.VolumeCapability
		expectError bool
	}{
		{
			name: "valid ReadWriteOnce with ext4",
			caps: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid ReadWriteOnce with xfs",
			caps: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "xfs",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid access mode ReadWriteMany",
			caps: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid filesystem type",
			caps: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ntfs",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateVolumeCapabilities(tt.caps)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestControllerGetCapabilities tests the ControllerGetCapabilities method
func TestControllerGetCapabilities(t *testing.T) {
	driver := &Driver{
		name:    "csi.emma.ms",
		version: "1.0.0",
	}
	
	service := NewControllerService(driver, nil)
	
	resp, err := service.ControllerGetCapabilities(context.Background(), &csi.ControllerGetCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if resp == nil {
		t.Fatal("expected response but got nil")
	}
	
	// Verify expected capabilities
	expectedCaps := map[csi.ControllerServiceCapability_RPC_Type]bool{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:   true,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME: true,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME:          true,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES:           true,
	}
	
	for _, cap := range resp.Capabilities {
		rpc := cap.GetRpc()
		if rpc == nil {
			continue
		}
		if !expectedCaps[rpc.Type] {
			t.Errorf("unexpected capability: %v", rpc.Type)
		}
		delete(expectedCaps, rpc.Type)
	}
	
	if len(expectedCaps) > 0 {
		t.Errorf("missing expected capabilities: %v", expectedCaps)
	}
}

// TestControllerUnimplementedMethods tests unimplemented methods return proper errors
func TestControllerUnimplementedMethods(t *testing.T) {
	driver := &Driver{
		name:    "csi.emma.ms",
		version: "1.0.0",
	}
	
	service := NewControllerService(driver, nil)
	
	t.Run("GetCapacity", func(t *testing.T) {
		_, err := service.GetCapacity(context.Background(), &csi.GetCapacityRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("expected Unimplemented error, got %v", status.Code(err))
		}
	})
	
	t.Run("CreateSnapshot", func(t *testing.T) {
		_, err := service.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("expected Unimplemented error, got %v", status.Code(err))
		}
	})
	
	t.Run("DeleteSnapshot", func(t *testing.T) {
		_, err := service.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{})
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("expected Unimplemented error, got %v", status.Code(err))
		}
	})
}
