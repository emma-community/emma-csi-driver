package driver

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestGetPluginInfo tests the GetPluginInfo method
func TestGetPluginInfo(t *testing.T) {
	tests := []struct {
		name        string
		driver      *Driver
		expectError bool
		errorCode   codes.Code
	}{
		{
			name: "successful plugin info",
			driver: &Driver{
				name:    "csi.emma.ms",
				version: "1.0.0",
			},
			expectError: false,
		},
		{
			name: "missing driver name",
			driver: &Driver{
				name:    "",
				version: "1.0.0",
			},
			expectError: true,
			errorCode:   codes.Unavailable,
		},
		{
			name: "missing driver version",
			driver: &Driver{
				name:    "csi.emma.ms",
				version: "",
			},
			expectError: true,
			errorCode:   codes.Unavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewIdentityService(tt.driver)
			resp, err := service.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if status.Code(err) != tt.errorCode {
					t.Errorf("expected error code %v, got %v", tt.errorCode, status.Code(err))
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Fatal("expected response but got nil")
				}
				if resp.Name != tt.driver.name {
					t.Errorf("expected name %s, got %s", tt.driver.name, resp.Name)
				}
				if resp.VendorVersion != tt.driver.version {
					t.Errorf("expected version %s, got %s", tt.driver.version, resp.VendorVersion)
				}
			}
		})
	}
}

// TestGetPluginCapabilities tests the GetPluginCapabilities method
func TestGetPluginCapabilities(t *testing.T) {
	driver := &Driver{
		name:    "csi.emma.ms",
		version: "1.0.0",
	}

	service := NewIdentityService(driver)
	resp, err := service.GetPluginCapabilities(context.Background(), &csi.GetPluginCapabilitiesRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response but got nil")
	}

	// Verify expected capabilities
	expectedCaps := map[csi.PluginCapability_Service_Type]bool{
		csi.PluginCapability_Service_CONTROLLER_SERVICE:               true,
		csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS: true,
	}

	for _, cap := range resp.Capabilities {
		svc := cap.GetService()
		if svc == nil {
			continue
		}
		if !expectedCaps[svc.Type] {
			t.Errorf("unexpected capability: %v", svc.Type)
		}
		delete(expectedCaps, svc.Type)
	}

	if len(expectedCaps) > 0 {
		t.Errorf("missing expected capabilities: %v", expectedCaps)
	}
}

// TestProbe tests the Probe method
func TestProbe(t *testing.T) {
	tests := []struct {
		name        string
		driver      *Driver
		expectError bool
	}{
		{
			name: "successful probe without Emma client",
			driver: &Driver{
				name:       "csi.emma.ms",
				version:    "1.0.0",
				emmaClient: nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewIdentityService(tt.driver)
			resp, err := service.Probe(context.Background(), &csi.ProbeRequest{})

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Fatal("expected response but got nil")
				}
			}
		})
	}
}
