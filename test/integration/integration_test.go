//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/emma-csi-driver/pkg/emma"
)

// TestVolumeLifecycle tests the complete volume lifecycle against Emma API
// This test requires real Emma API credentials and should be run with:
// go test -tags=integration ./test/integration/...
func TestVolumeLifecycle(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("EMMA_CLIENT_ID") == "" || os.Getenv("EMMA_CLIENT_SECRET") == "" {
		t.Skip("Skipping integration test: EMMA_CLIENT_ID and EMMA_CLIENT_SECRET not set")
	}

	clientID := os.Getenv("EMMA_CLIENT_ID")
	clientSecret := os.Getenv("EMMA_CLIENT_SECRET")
	apiURL := os.Getenv("EMMA_API_URL")
	if apiURL == "" {
		apiURL = "https://api.emma.ms/external"
	}

	// Create Emma client
	client, err := emma.NewClient(apiURL, clientID, clientSecret)
	if err != nil {
		t.Fatalf("failed to create Emma client: %v", err)
	}

	ctx := context.Background()

	// Test 1: Create volume
	t.Run("CreateVolume", func(t *testing.T) {
		volumeName := "test-volume-" + time.Now().Format("20060102-150405")
		volume, err := client.CreateVolume(ctx, volumeName, 10, "ssd", "aws-eu-west-2")
		if err != nil {
			t.Fatalf("failed to create volume: %v", err)
		}
		if volume.ID == 0 {
			t.Error("expected non-zero volume ID")
		}
		t.Logf("Created volume: ID=%d, Name=%s", volume.ID, volume.Name)

		// Clean up
		defer func() {
			_ = client.DeleteVolume(ctx, volume.ID)
		}()

		// Test 2: Get volume
		t.Run("GetVolume", func(t *testing.T) {
			vol, err := client.GetVolume(ctx, volume.ID)
			if err != nil {
				t.Fatalf("failed to get volume: %v", err)
			}
			if vol.ID != volume.ID {
				t.Errorf("expected volume ID %d, got %d", volume.ID, vol.ID)
			}
		})

		// Test 3: Wait for volume to become available
		t.Run("WaitForAvailable", func(t *testing.T) {
			err := client.WaitForVolumeStatus(ctx, volume.ID, "AVAILABLE", 5*time.Minute)
			if err != nil {
				t.Fatalf("failed to wait for volume: %v", err)
			}
		})
	})
}

// TestConcurrentOperations tests concurrent volume operations
func TestConcurrentOperations(t *testing.T) {
	if os.Getenv("EMMA_CLIENT_ID") == "" || os.Getenv("EMMA_CLIENT_SECRET") == "" {
		t.Skip("Skipping integration test: EMMA_CLIENT_ID and EMMA_CLIENT_SECRET not set")
	}

	clientID := os.Getenv("EMMA_CLIENT_ID")
	clientSecret := os.Getenv("EMMA_CLIENT_SECRET")
	apiURL := os.Getenv("EMMA_API_URL")
	if apiURL == "" {
		apiURL = "https://api.emma.ms/external"
	}

	client, err := emma.NewClient(apiURL, clientID, clientSecret)
	if err != nil {
		t.Fatalf("failed to create Emma client: %v", err)
	}

	ctx := context.Background()

	// Create multiple volumes concurrently
	numVolumes := 3
	volumeIDs := make([]int32, numVolumes)
	errors := make(chan error, numVolumes)

	for i := 0; i < numVolumes; i++ {
		go func(index int) {
			volumeName := "test-concurrent-" + time.Now().Format("20060102-150405")
			volume, err := client.CreateVolume(ctx, volumeName, 10, "ssd", "aws-eu-west-2")
			if err != nil {
				errors <- err
				return
			}
			volumeIDs[index] = volume.ID
			errors <- nil
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numVolumes; i++ {
		if err := <-errors; err != nil {
			t.Errorf("concurrent operation failed: %v", err)
		}
	}

	// Clean up
	for _, volumeID := range volumeIDs {
		if volumeID != 0 {
			_ = client.DeleteVolume(ctx, volumeID)
		}
	}
}

// TestFailureScenarios tests various failure scenarios
func TestFailureScenarios(t *testing.T) {
	if os.Getenv("EMMA_CLIENT_ID") == "" || os.Getenv("EMMA_CLIENT_SECRET") == "" {
		t.Skip("Skipping integration test: EMMA_CLIENT_ID and EMMA_CLIENT_SECRET not set")
	}

	clientID := os.Getenv("EMMA_CLIENT_ID")
	clientSecret := os.Getenv("EMMA_CLIENT_SECRET")
	apiURL := os.Getenv("EMMA_API_URL")
	if apiURL == "" {
		apiURL = "https://api.emma.ms/external"
	}

	client, err := emma.NewClient(apiURL, clientID, clientSecret)
	if err != nil {
		t.Fatalf("failed to create Emma client: %v", err)
	}

	ctx := context.Background()

	t.Run("GetNonExistentVolume", func(t *testing.T) {
		_, err := client.GetVolume(ctx, 999999)
		if err == nil {
			t.Error("expected error for non-existent volume")
		}
	})

	t.Run("DeleteNonExistentVolume", func(t *testing.T) {
		err := client.DeleteVolume(ctx, 999999)
		// Should not error - idempotent operation
		if err != nil {
			t.Logf("delete non-existent volume returned: %v", err)
		}
	})

	t.Run("InvalidDataCenter", func(t *testing.T) {
		err := client.ValidateDataCenter(ctx, "invalid-datacenter-id")
		if err == nil {
			t.Error("expected error for invalid datacenter")
		}
	})
}
