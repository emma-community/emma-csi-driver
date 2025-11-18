package emma

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emma-csi-driver/pkg/logging"
)

// newTestClient creates a test client with a mock server
func newTestClient(server *httptest.Server) *Client {
	return &Client{
		baseURL:     server.URL,
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		accessToken: "test-token",
		tokenExpiry: time.Now().Add(1 * time.Hour),
		logger:      logging.NewLogger("test-client"),
	}
}

// TestCreateVolume tests volume creation
func TestCreateVolume(t *testing.T) {
	tests := []struct {
		name           string
		volumeName     string
		sizeGB         int32
		volumeType     string
		dataCenterID   string
		responseStatus int
		responseBody   *VolumeResponse
		expectError    bool
	}{
		{
			name:           "successful volume creation",
			volumeName:     "test-volume",
			sizeGB:         10,
			volumeType:     "ssd",
			dataCenterID:   "aws-eu-west-2",
			responseStatus: http.StatusOK,
			responseBody: &VolumeResponse{
				ID:           123,
				Name:         "test-volume",
				SizeGB:       10,
				Type:         "ssd",
				Status:       "DRAFT",
				DataCenterID: "aws-eu-west-2",
			},
			expectError: false,
		},
		{
			name:           "API error",
			volumeName:     "test-volume",
			sizeGB:         10,
			volumeType:     "ssd",
			dataCenterID:   "aws-eu-west-2",
			responseStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/volumes" && r.Method == "POST" {
					w.WriteHeader(tt.responseStatus)
					if tt.responseBody != nil {
						json.NewEncoder(w).Encode(tt.responseBody)
					}
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			volume, err := client.CreateVolume(context.Background(), tt.volumeName, tt.sizeGB, tt.volumeType, tt.dataCenterID)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && volume != nil {
				if volume.ID != tt.responseBody.ID {
					t.Errorf("expected volume ID %d, got %d", tt.responseBody.ID, volume.ID)
				}
			}
		})
	}
}

// TestGetVolume tests volume retrieval
func TestGetVolume(t *testing.T) {
	tests := []struct {
		name           string
		volumeID       int32
		responseStatus int
		responseBody   *VolumeResponse
		expectError    bool
	}{
		{
			name:           "successful get volume",
			volumeID:       123,
			responseStatus: http.StatusOK,
			responseBody: &VolumeResponse{
				ID:           123,
				Name:         "test-volume",
				SizeGB:       10,
				Type:         "ssd",
				Status:       "AVAILABLE",
				DataCenterID: "aws-eu-west-2",
			},
			expectError: false,
		},
		{
			name:           "volume not found",
			volumeID:       999,
			responseStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			volume, err := client.GetVolume(context.Background(), tt.volumeID)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && volume != nil {
				if volume.ID != tt.responseBody.ID {
					t.Errorf("expected volume ID %d, got %d", tt.responseBody.ID, volume.ID)
				}
			}
		})
	}
}

// TestDeleteVolume tests volume deletion
func TestDeleteVolume(t *testing.T) {
	tests := []struct {
		name           string
		volumeID       int32
		responseStatus int
		expectError    bool
	}{
		{
			name:           "successful delete",
			volumeID:       123,
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "volume not found (already deleted)",
			volumeID:       999,
			responseStatus: http.StatusNotFound,
			expectError:    false, // Should not error on already deleted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
			}))
			defer server.Close()

			client := newTestClient(server)
			err := client.DeleteVolume(context.Background(), tt.volumeID)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestAttachVolume tests volume attachment
func TestAttachVolume(t *testing.T) {
	tests := []struct {
		name           string
		vmID           int32
		volumeID       int32
		responseStatus int
		expectError    bool
	}{
		{
			name:           "successful attach",
			vmID:           456,
			volumeID:       123,
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "API error",
			vmID:           456,
			volumeID:       123,
			responseStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
			}))
			defer server.Close()

			client := newTestClient(server)
			err := client.AttachVolume(context.Background(), tt.vmID, tt.volumeID)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestDetachVolume tests volume detachment
func TestDetachVolume(t *testing.T) {
	tests := []struct {
		name           string
		vmID           int32
		volumeID       int32
		responseStatus int
		expectError    bool
	}{
		{
			name:           "successful detach",
			vmID:           456,
			volumeID:       123,
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "API error",
			vmID:           456,
			volumeID:       123,
			responseStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
			}))
			defer server.Close()

			client := newTestClient(server)
			err := client.DetachVolume(context.Background(), tt.vmID, tt.volumeID)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestResizeVolume tests volume resizing
func TestResizeVolume(t *testing.T) {
	tests := []struct {
		name           string
		volumeID       int32
		newSizeGB      int32
		responseStatus int
		expectError    bool
	}{
		{
			name:           "successful resize",
			volumeID:       123,
			newSizeGB:      20,
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "API error",
			volumeID:       123,
			newSizeGB:      20,
			responseStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
			}))
			defer server.Close()

			client := newTestClient(server)
			err := client.ResizeVolume(context.Background(), tt.volumeID, tt.newSizeGB)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestListVolumes tests volume listing
func TestListVolumes(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   []*VolumeResponse
		expectError    bool
	}{
		{
			name:           "successful list",
			responseStatus: http.StatusOK,
			responseBody: []*VolumeResponse{
				{
					ID:           123,
					Name:         "volume-1",
					SizeGB:       10,
					Type:         "ssd",
					Status:       "AVAILABLE",
					DataCenterID: "aws-eu-west-2",
				},
				{
					ID:           124,
					Name:         "volume-2",
					SizeGB:       20,
					Type:         "hdd",
					Status:       "ACTIVE",
					DataCenterID: "aws-eu-west-2",
				},
			},
			expectError: false,
		},
		{
			name:           "API error",
			responseStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			volumes, err := client.ListVolumes(context.Background())

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && volumes != nil {
				if len(volumes) != len(tt.responseBody) {
					t.Errorf("expected %d volumes, got %d", len(tt.responseBody), len(volumes))
				}
			}
		})
	}
}

// TestGetAccessToken tests token management
func TestGetAccessToken(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		client := &Client{
			accessToken: "valid-token",
			tokenExpiry: time.Now().Add(1 * time.Hour),
		}

		token, err := client.getAccessToken(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if token != "valid-token" {
			t.Errorf("expected token 'valid-token', got '%s'", token)
		}
	})
}
