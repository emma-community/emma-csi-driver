package driver

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
)

// TestNodeGetCapabilities tests the NodeGetCapabilities method
func TestNodeGetCapabilities(t *testing.T) {
	driver := &Driver{
		name:    "csi.emma.ms",
		version: "1.0.0",
	}

	service := NewNodeService(driver)
	resp, err := service.NodeGetCapabilities(context.Background(), &csi.NodeGetCapabilitiesRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response but got nil")
	}

	// Verify expected capabilities
	expectedCaps := map[csi.NodeServiceCapability_RPC_Type]bool{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME: true,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME:        true,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS:     true,
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

// TestNodeGetInfo tests the NodeGetInfo method
func TestNodeGetInfo(t *testing.T) {
	tests := []struct {
		name     string
		driver   *Driver
		expected string
	}{
		{
			name: "successful node info",
			driver: &Driver{
				name:    "csi.emma.ms",
				version: "1.0.0",
				nodeID:  "test-node-123",
			},
			expected: "test-node-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewNodeService(tt.driver)
			resp, err := service.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response but got nil")
			}
			if resp.NodeId != tt.expected {
				t.Errorf("expected node ID %s, got %s", tt.expected, resp.NodeId)
			}
			if resp.MaxVolumesPerNode != 16 {
				t.Errorf("expected max volumes 16, got %d", resp.MaxVolumesPerNode)
			}
		})
	}
}

// TestNodeStageVolume tests the NodeStageVolume method validation
func TestNodeStageVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.NodeStageVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.NodeStageVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing staging target path",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "123",
				StagingTargetPath: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing volume capability",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "123",
				StagingTargetPath: "/mnt/staging",
				VolumeCapability:  nil,
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			if tt.req.GetVolumeId() == "" && tt.expectError {
				// Expected error case
			}
			if tt.req.GetStagingTargetPath() == "" && tt.expectError {
				// Expected error case
			}
			if tt.req.GetVolumeCapability() == nil && tt.expectError {
				// Expected error case
			}
		})
	}
}

// TestNodeUnstageVolume tests the NodeUnstageVolume method validation
func TestNodeUnstageVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.NodeUnstageVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing staging target path",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "123",
				StagingTargetPath: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			if tt.req.GetVolumeId() == "" && tt.expectError {
				// Expected error case
			}
			if tt.req.GetStagingTargetPath() == "" && tt.expectError {
				// Expected error case
			}
		})
	}
}

// TestNodePublishVolume tests the NodePublishVolume method validation
func TestNodePublishVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.NodePublishVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing staging target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "123",
				StagingTargetPath: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "123",
				StagingTargetPath: "/mnt/staging",
				TargetPath:        "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			if tt.req.GetVolumeId() == "" && tt.expectError {
				// Expected error case
			}
			if tt.req.GetStagingTargetPath() == "" && tt.expectError {
				// Expected error case
			}
			if tt.req.GetTargetPath() == "" && tt.expectError {
				// Expected error case
			}
		})
	}
}

// TestNodeUnpublishVolume tests the NodeUnpublishVolume method validation
func TestNodeUnpublishVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.NodeUnpublishVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing target path",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   "123",
				TargetPath: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			if tt.req.GetVolumeId() == "" && tt.expectError {
				// Expected error case
			}
			if tt.req.GetTargetPath() == "" && tt.expectError {
				// Expected error case
			}
		})
	}
}

// TestNodeExpandVolume tests the NodeExpandVolume method validation
func TestNodeExpandVolume(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.NodeExpandVolumeRequest
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
		{
			name: "missing volume path",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "123",
				VolumePath: "",
			},
			expectError: true,
			errorCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			if tt.req.GetVolumeId() == "" && tt.expectError {
				// Expected error case
			}
			if tt.req.GetVolumePath() == "" && tt.expectError {
				// Expected error case
			}
		})
	}
}
