package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// IdentityService implements the CSI Identity service
type IdentityService struct {
	driver *Driver
}

// NewIdentityService creates a new identity service
func NewIdentityService(driver *Driver) *IdentityService {
	return &IdentityService{
		driver: driver,
	}
}

// GetPluginInfo returns plugin information
func (s *IdentityService) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	klog.V(4).Info("GetPluginInfo called")

	if s.driver.name == "" {
		return nil, status.Error(codes.Unavailable, "driver name not configured")
	}

	if s.driver.version == "" {
		return nil, status.Error(codes.Unavailable, "driver version not configured")
	}

	return &csi.GetPluginInfoResponse{
		Name:          s.driver.name,
		VendorVersion: s.driver.version,
	}, nil
}

// GetPluginCapabilities returns plugin capabilities
func (s *IdentityService) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	klog.V(4).Info("GetPluginCapabilities called")

	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}, nil
}

// Probe checks if the plugin is ready
func (s *IdentityService) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	klog.V(4).Info("Probe called")

	// Perform Emma API health check if client is available
	if s.driver.emmaClient != nil {
		// Try to list data centers as a health check
		// This verifies authentication and API connectivity
		_, err := s.driver.emmaClient.GetDataCenters(ctx)
		if err != nil {
			klog.Errorf("Emma API health check failed: %v", err)
			return &csi.ProbeResponse{}, nil
		}
		klog.V(4).Info("Emma API health check passed")
	}
	
	return &csi.ProbeResponse{}, nil
}
