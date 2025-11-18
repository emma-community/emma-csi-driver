package emma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	emma "github.com/emma-community/emma-go-sdk"
	"k8s.io/klog/v2"

	"github.com/emma-csi-driver/pkg/logging"
	"github.com/emma-csi-driver/pkg/metrics"
)

// Client wraps the Emma SDK client with CSI-specific functionality
type Client struct {
	apiClient    *emma.APIClient
	ctx          context.Context
	baseURL      string
	httpClient   *http.Client
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time
	tokenMutex   sync.RWMutex
	clientID     string
	clientSecret string
	logger       *logging.Logger
}

// VolumeCreateRequest represents a volume creation request
type VolumeCreateRequest struct {
	Name         string `json:"name"`
	VolumeGb     int32  `json:"volumeGb"`
	VolumeType   string `json:"volumeType"`
	DataCenterID string `json:"dataCenterId"`
}

// VolumeResponse represents a volume from the API
type VolumeResponse struct {
	ID           int32  `json:"id"`
	Name         string `json:"name"`
	SizeGB       int32  `json:"sizeGb"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	AttachedToID *int32 `json:"attachedToId,omitempty"`
	DataCenterID string `json:"dataCenterId"`
	CreatedAt    string `json:"createdAt"`
}

// VMActionRequest represents a VM action request
type VMActionRequest struct {
	Action   string `json:"action"`
	VolumeID *int32 `json:"volumeId,omitempty"`
}

// NewClient creates a new Emma API client using the SDK
func NewClient(baseURL, clientID, clientSecret string) (*Client, error) {
	// Issue token
	config := emma.NewConfiguration()
	if baseURL != "" {
		config.Servers = emma.ServerConfigurations{
			{
				URL: baseURL,
			},
		}
	}

	apiClient := emma.NewAPIClient(config)

	// Get access token
	credentials := emma.NewCredentials(clientID, clientSecret)
	tokenResp, _, err := apiClient.AuthenticationAPI.IssueToken(context.Background()).Credentials(*credentials).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to issue token: %w", err)
	}

	// Create authenticated context
	ctx := context.WithValue(context.Background(), emma.ContextAccessToken, tokenResp.GetAccessToken())

	// Determine base URL
	if baseURL == "" {
		baseURL = "https://api.emma.ms/external"
	}

	logger := logging.NewLogger("emma-client")
	logger.Info("Emma API client initialized successfully")

	return &Client{
		apiClient:    apiClient,
		ctx:          ctx,
		baseURL:      baseURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		accessToken:  tokenResp.GetAccessToken(),
		refreshToken: tokenResp.GetRefreshToken(),
		tokenExpiry:  time.Now().Add(time.Duration(tokenResp.GetExpiresIn()) * time.Second),
		clientID:     clientID,
		clientSecret: clientSecret,
		logger:       logger,
	}, nil
}

// getAccessToken returns a valid access token, refreshing if necessary
func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMutex.RLock()
	// Check if we have a valid token (with 5 minute buffer)
	// Token is valid if current time is before (expiry - 5 minutes)
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute)) {
		token := c.accessToken
		c.tokenMutex.RUnlock()
		return token, nil
	}
	c.tokenMutex.RUnlock()

	// Need to refresh token
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	// Double-check after acquiring write lock
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute)) {
		return c.accessToken, nil
	}

	klog.Infof("Access token expired or expiring soon (expiry: %v), refreshing...", c.tokenExpiry)

	// Try refresh token first
	if c.refreshToken != "" {
		refresh := emma.NewRefreshToken(c.refreshToken)
		tokenResp, _, err := c.apiClient.AuthenticationAPI.RefreshToken(context.Background()).RefreshToken(*refresh).Execute()
		if err == nil {
			c.accessToken = tokenResp.GetAccessToken()
			c.refreshToken = tokenResp.GetRefreshToken()
			c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.GetExpiresIn()) * time.Second)
			c.ctx = context.WithValue(context.Background(), emma.ContextAccessToken, c.accessToken)
			klog.Infof("Access token refreshed successfully (new expiry: %v)", c.tokenExpiry)
			c.logger.Info("Access token refreshed successfully")
			return c.accessToken, nil
		}
		klog.Warningf("Failed to refresh token, will re-authenticate: %v", err)
		c.logger.Warn("Token refresh failed, re-authenticating", map[string]interface{}{"error": err.Error()})
	}

	// If refresh fails, re-authenticate with credentials
	klog.Info("Re-authenticating with credentials")
	c.logger.Info("Re-authenticating with Emma API")
	credentials := emma.NewCredentials(c.clientID, c.clientSecret)
	tokenResp, _, err := c.apiClient.AuthenticationAPI.IssueToken(context.Background()).Credentials(*credentials).Execute()
	if err != nil {
		c.logger.Error("Re-authentication failed", err)
		return "", fmt.Errorf("failed to issue new token: %w", err)
	}

	c.accessToken = tokenResp.GetAccessToken()
	c.refreshToken = tokenResp.GetRefreshToken()
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.GetExpiresIn()) * time.Second)
	c.ctx = context.WithValue(context.Background(), emma.ContextAccessToken, c.accessToken)
	klog.Infof("Re-authenticated successfully (new expiry: %v)", c.tokenExpiry)
	c.logger.Info("Re-authenticated successfully")

	return c.accessToken, nil
}

// doRequest executes an authenticated HTTP request for endpoints not in SDK
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	timer := metrics.NewAPIRequestTimer(method, path)

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Get access token
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logger.Debug("Emma API request", map[string]interface{}{
		"method": method,
		"path":   path,
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		timer.Observe(0)
		c.logger.Error("Emma API request failed", err, map[string]interface{}{
			"method": method,
			"path":   path,
		})
		return nil, fmt.Errorf("request failed: %w", err)
	}

	timer.Observe(resp.StatusCode)
	c.logger.Debug("Emma API response", map[string]interface{}{
		"method": method,
		"path":   path,
		"status": resp.StatusCode,
	})

	// If we get 401 Unauthorized, the token might have expired
	// Return the response so caller can handle it
	if resp.StatusCode == http.StatusUnauthorized {
		c.logger.Warn("Received 401 Unauthorized, token may have expired", map[string]interface{}{
			"method": method,
			"path":   path,
		})
	}

	return resp, nil
}

// CreateVolume creates a new volume using direct API call
func (c *Client) CreateVolume(ctx context.Context, name string, sizeGB int32, volumeType string, dataCenterID string) (*VolumeResponse, error) {
	klog.V(4).Infof("Creating volume: %s, size: %dGB, type: %s, datacenter: %s",
		name, sizeGB, volumeType, dataCenterID)

	req := &VolumeCreateRequest{
		Name:         name,
		VolumeGb:     sizeGB,
		VolumeType:   volumeType,
		DataCenterID: dataCenterID,
	}

	resp, err := c.doRequest(ctx, "POST", "/v1/volumes", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create volume: status %d, body: %s", resp.StatusCode, string(body))
	}

	var volume VolumeResponse
	if err := json.NewDecoder(resp.Body).Decode(&volume); err != nil {
		return nil, fmt.Errorf("failed to decode volume response: %w", err)
	}

	klog.V(4).Infof("Volume created successfully: ID=%d, status=%s", volume.ID, volume.Status)
	return &volume, nil
}

// GetVolume retrieves a volume by ID using direct API call
func (c *Client) GetVolume(ctx context.Context, volumeID int32) (*VolumeResponse, error) {
	klog.V(5).Infof("Getting volume: %d", volumeID)

	path := fmt.Sprintf("/v1/volumes/%d", volumeID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("volume %d not found", volumeID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get volume: status %d, body: %s", resp.StatusCode, string(body))
	}

	var volume VolumeResponse
	if err := json.NewDecoder(resp.Body).Decode(&volume); err != nil {
		return nil, fmt.Errorf("failed to decode volume response: %w", err)
	}

	return &volume, nil
}

// ListVolumes lists all volumes using direct API call
func (c *Client) ListVolumes(ctx context.Context) ([]*VolumeResponse, error) {
	klog.V(5).Info("Listing volumes")

	resp, err := c.doRequest(ctx, "GET", "/v1/volumes", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list volumes: status %d, body: %s", resp.StatusCode, string(body))
	}

	var volumes []*VolumeResponse
	if err := json.NewDecoder(resp.Body).Decode(&volumes); err != nil {
		return nil, fmt.Errorf("failed to decode volumes response: %w", err)
	}

	klog.V(5).Infof("Listed %d volumes", len(volumes))
	return volumes, nil
}

// DeleteVolume deletes a volume using direct API call
func (c *Client) DeleteVolume(ctx context.Context, volumeID int32) error {
	klog.V(4).Infof("Deleting volume: %d", volumeID)

	path := fmt.Sprintf("/v1/volumes/%d", volumeID)
	resp, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		klog.V(4).Infof("Volume %d not found, considering it already deleted", volumeID)
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete volume: status %d, body: %s", resp.StatusCode, string(body))
	}

	klog.V(4).Infof("Volume %d deleted successfully", volumeID)
	return nil
}

// ResizeVolume resizes a volume using direct API call
func (c *Client) ResizeVolume(ctx context.Context, volumeID int32, newSizeGB int32) error {
	klog.V(4).Infof("Resizing volume %d to %dGB", volumeID, newSizeGB)

	path := fmt.Sprintf("/v1/volumes/%d/actions", volumeID)
	req := map[string]interface{}{
		"action": "edit",
		"sizeGb": newSizeGB,
	}

	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to resize volume: status %d, body: %s", resp.StatusCode, string(body))
	}

	klog.V(4).Infof("Volume %d resize initiated successfully", volumeID)
	return nil
}

// AttachVolume attaches a volume to a VM using direct API call with retry logic
func (c *Client) AttachVolume(ctx context.Context, vmID int32, volumeID int32) error {
	klog.V(4).Infof("Attaching volume %d to VM %d", volumeID, vmID)

	path := fmt.Sprintf("/v1/vms/%d/actions", vmID)
	req := &VMActionRequest{
		Action:   "attach",
		VolumeID: &volumeID,
	}

	// Optimized retry logic for VM state conflicts
	// Emma VMs can be in transitional states during startup/operations
	// Use shorter initial delays with faster ramp-up
	maxRetries := 12
	initialDelay := 1 * time.Second
	maxDelay := 15 * time.Second

	startTime := time.Now()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.doRequest(ctx, "POST", path, req)
		if err != nil {
			// Check if it's a token error and retry once
			if attempt == 0 && (resp == nil || resp.StatusCode == http.StatusUnauthorized) {
				klog.V(4).Info("Possible token issue, refreshing and retrying")
				_, _ = c.getAccessToken(ctx)
				continue
			}
			return err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
			elapsed := time.Since(startTime)
			klog.V(4).Infof("Volume %d attach to VM %d initiated successfully (took %v, %d attempts)",
				volumeID, vmID, elapsed, attempt+1)
			return nil
		}

		// Handle 409 CONFLICT - VM in transitional state
		if resp.StatusCode == http.StatusConflict {
			if attempt < maxRetries {
				// Optimized backoff: start fast, ramp up gradually
				// 1s, 2s, 3s, 5s, 8s, 12s, 15s, 15s...
				var delay time.Duration
				if attempt < 3 {
					delay = initialDelay * time.Duration(attempt+1)
				} else {
					delay = initialDelay * time.Duration(1<<uint(attempt-2))
				}
				if delay > maxDelay {
					delay = maxDelay
				}

				klog.V(4).Infof("VM %d not ready for attach (attempt %d/%d), retrying in %v: %s",
					vmID, attempt+1, maxRetries+1, delay, string(body))

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		}

		// Handle 400 BAD_REQUEST - might be a transient issue
		if resp.StatusCode == http.StatusBadRequest && attempt < 3 {
			klog.V(4).Infof("Bad request on attempt %d, retrying after 2s: %s", attempt+1, string(body))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}

		// Non-retryable error or max retries exceeded
		return fmt.Errorf("failed to attach volume: status %d, body: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("failed to attach volume after %d retries: VM not ready", maxRetries+1)
}

// DetachVolume detaches a volume from a VM using direct API call with retry logic
func (c *Client) DetachVolume(ctx context.Context, vmID int32, volumeID int32) error {
	klog.V(4).Infof("Detaching volume %d from VM %d", volumeID, vmID)

	path := fmt.Sprintf("/v1/vms/%d/actions", vmID)
	req := &VMActionRequest{
		Action:   "detach",
		VolumeID: &volumeID,
	}

	// Optimized retry logic for VM state conflicts
	maxRetries := 12
	initialDelay := 1 * time.Second
	maxDelay := 15 * time.Second

	startTime := time.Now()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.doRequest(ctx, "POST", path, req)
		if err != nil {
			// Check if it's a token error and retry once
			if attempt == 0 && (resp == nil || resp.StatusCode == http.StatusUnauthorized) {
				klog.V(4).Info("Possible token issue, refreshing and retrying")
				_, _ = c.getAccessToken(ctx)
				continue
			}
			return err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
			elapsed := time.Since(startTime)
			klog.V(4).Infof("Volume %d detach from VM %d initiated successfully (took %v, %d attempts)",
				volumeID, vmID, elapsed, attempt+1)
			return nil
		}

		// Handle 409 CONFLICT - VM in transitional state
		if resp.StatusCode == http.StatusConflict {
			if attempt < maxRetries {
				// Optimized backoff: start fast, ramp up gradually
				var delay time.Duration
				if attempt < 3 {
					delay = initialDelay * time.Duration(attempt+1)
				} else {
					delay = initialDelay * time.Duration(1<<uint(attempt-2))
				}
				if delay > maxDelay {
					delay = maxDelay
				}

				klog.V(4).Infof("VM %d not ready for detach (attempt %d/%d), retrying in %v: %s",
					vmID, attempt+1, maxRetries+1, delay, string(body))

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		}

		// Handle 400 BAD_REQUEST - might be a transient issue
		if resp.StatusCode == http.StatusBadRequest && attempt < 3 {
			klog.V(4).Infof("Bad request on attempt %d, retrying after 2s: %s", attempt+1, string(body))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}

		// Non-retryable error or max retries exceeded
		return fmt.Errorf("failed to detach volume: status %d, body: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("failed to detach volume after %d retries: VM not ready", maxRetries+1)
}

// GetVM retrieves a VM by ID
func (c *Client) GetVM(ctx context.Context, vmID int32) (*emma.Vm, error) {
	klog.V(5).Infof("Getting VM: %d", vmID)

	// Ensure we have a valid token
	if _, err := c.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	vm, _, err := c.apiClient.VirtualMachinesAPI.GetVm(c.ctx, vmID).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get VM: %w", err)
	}

	return vm, nil
}

// ListVMs lists all VMs
func (c *Client) ListVMs(ctx context.Context) ([]emma.Vm, error) {
	klog.V(5).Info("Listing VMs")

	// Ensure we have a valid token
	if _, err := c.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	vms, _, err := c.apiClient.VirtualMachinesAPI.GetVms(c.ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	klog.V(5).Infof("Listed %d VMs", len(vms))
	return vms, nil
}

// ListKubernetesClusters lists all Kubernetes clusters
func (c *Client) ListKubernetesClusters(ctx context.Context) ([]emma.Kubernetes, error) {
	klog.V(5).Info("Listing Kubernetes clusters")

	// Ensure we have a valid token
	if _, err := c.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	clusters, _, err := c.apiClient.KubernetesClustersAPI.GetKubernetesClusters(c.ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to list Kubernetes clusters: %w", err)
	}

	klog.V(5).Infof("Listed %d Kubernetes clusters", len(clusters))
	return clusters, nil
}

// GetKubernetesCluster retrieves a specific Kubernetes cluster by ID
func (c *Client) GetKubernetesCluster(ctx context.Context, clusterID int32) (*emma.Kubernetes, error) {
	klog.V(5).Infof("Getting Kubernetes cluster: %d", clusterID)

	// Ensure we have a valid token
	if _, err := c.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	cluster, _, err := c.apiClient.KubernetesClustersAPI.GetKubernetesCluster(c.ctx, clusterID).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes cluster: %w", err)
	}

	return cluster, nil
}

// GetDataCenters retrieves all available data centers
func (c *Client) GetDataCenters(ctx context.Context) ([]emma.DataCenter, error) {
	klog.V(5).Info("Getting data centers")

	// Ensure we have a valid token
	if _, err := c.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	dataCenters, _, err := c.apiClient.DataCentersAPI.GetDataCenters(c.ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get data centers: %w", err)
	}

	klog.V(5).Infof("Retrieved %d data centers", len(dataCenters))
	return dataCenters, nil
}

// GetDataCenter retrieves a specific data center by ID
func (c *Client) GetDataCenter(ctx context.Context, dataCenterID string) (*emma.DataCenter, error) {
	klog.V(5).Infof("Getting data center: %s", dataCenterID)

	// Ensure we have a valid token
	if _, err := c.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	dc, _, err := c.apiClient.DataCentersAPI.GetDataCenter(c.ctx, dataCenterID).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get data center: %w", err)
	}

	return dc, nil
}

// GetVolumeConfigs retrieves available volume configurations
func (c *Client) GetVolumeConfigs(ctx context.Context) ([]emma.VolumeConfiguration, error) {
	klog.V(5).Info("Getting volume configs")

	// Ensure we have a valid token
	if _, err := c.getAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	_, _, err := c.apiClient.VolumesConfigurationsAPI.GetSystemVolumeConfigs(c.ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get volume configs: %w", err)
	}

	// Note: The SDK response structure needs to be verified
	// For now, return empty slice as this is primarily used for validation
	klog.V(5).Info("Volume configs retrieved successfully")
	return []emma.VolumeConfiguration{}, nil
}

// ValidateDataCenter checks if a data center ID is valid
func (c *Client) ValidateDataCenter(ctx context.Context, dataCenterID string) error {
	klog.V(5).Infof("Validating data center: %s", dataCenterID)

	_, err := c.GetDataCenter(ctx, dataCenterID)
	if err != nil {
		return fmt.Errorf("data center %s not found: %w", dataCenterID, err)
	}

	klog.V(5).Infof("Data center %s is valid", dataCenterID)
	return nil
}

// WaitForVolumeStatus polls until volume reaches desired status or timeout
func (c *Client) WaitForVolumeStatus(ctx context.Context, volumeID int32, desiredStatus string, timeout time.Duration) error {
	klog.V(4).Infof("Waiting for volume %d to reach status %s (timeout: %v)", volumeID, desiredStatus, timeout)

	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for volume %d to reach status %s", volumeID, desiredStatus)
		}

		volume, err := c.GetVolume(ctx, volumeID)
		if err != nil {
			return fmt.Errorf("failed to get volume status: %w", err)
		}

		klog.V(5).Infof("Volume %d current status: %s", volumeID, volume.Status)

		if volume.Status == desiredStatus {
			klog.V(4).Infof("Volume %d reached desired status: %s", volumeID, desiredStatus)
			return nil
		}

		if volume.Status == "FAILED" {
			return fmt.Errorf("volume %d entered FAILED state", volumeID)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			// Continue polling
		}
	}
}

// WaitForVolumeAttachment polls until volume is attached to the specified VM
func (c *Client) WaitForVolumeAttachment(ctx context.Context, volumeID int32, vmID int32, timeout time.Duration) error {
	klog.V(4).Infof("Waiting for volume %d to attach to VM %d (timeout: %v)", volumeID, vmID, timeout)

	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for volume %d to attach to VM %d", volumeID, vmID)
		}

		volume, err := c.GetVolume(ctx, volumeID)
		if err != nil {
			return fmt.Errorf("failed to get volume status: %w", err)
		}

		klog.V(5).Infof("Volume %d status: %s, attachedTo: %v", volumeID, volume.Status, volume.AttachedToID)

		if volume.Status == "ACTIVE" && volume.AttachedToID != nil && *volume.AttachedToID == vmID {
			klog.V(4).Infof("Volume %d successfully attached to VM %d", volumeID, vmID)
			return nil
		}

		if volume.Status == "FAILED" {
			return fmt.Errorf("volume %d entered FAILED state during attachment", volumeID)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			// Continue polling
		}
	}
}

// WaitForVolumeDetachment polls until volume is detached
func (c *Client) WaitForVolumeDetachment(ctx context.Context, volumeID int32, timeout time.Duration) error {
	klog.V(4).Infof("Waiting for volume %d to detach (timeout: %v)", volumeID, timeout)

	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for volume %d to detach", volumeID)
		}

		volume, err := c.GetVolume(ctx, volumeID)
		if err != nil {
			return fmt.Errorf("failed to get volume status: %w", err)
		}

		klog.V(5).Infof("Volume %d status: %s, attachedTo: %v", volumeID, volume.Status, volume.AttachedToID)

		if volume.Status == "AVAILABLE" && volume.AttachedToID == nil {
			klog.V(4).Infof("Volume %d successfully detached", volumeID)
			return nil
		}

		if volume.Status == "FAILED" {
			return fmt.Errorf("volume %d entered FAILED state during detachment", volumeID)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			// Continue polling
		}
	}
}
