package driver

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	emma "github.com/emma-community/emma-go-sdk"
	"k8s.io/klog/v2"
)

const (
	// DriverName is the name of the CSI driver
	DriverName = "csi.emma.ms"

	// DriverVersion is the version of the CSI driver
	DriverVersion = "v1.0.0"
)

// EmmaClient defines the interface for Emma API operations
type EmmaClient interface {
	GetDataCenters(ctx context.Context) ([]emma.DataCenter, error)
}

// Driver represents the Emma CSI driver
type Driver struct {
	name     string
	version  string
	nodeID   string
	endpoint string

	// Emma API client
	emmaClient EmmaClient

	// CSI services
	identityService   csi.IdentityServer
	controllerService csi.ControllerServer
	nodeService       csi.NodeServer

	// Server
	srv *NonBlockingGRPCServer
}

// NewDriver creates a new Emma CSI driver
func NewDriver(nodeID, endpoint string) (*Driver, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("nodeID is required")
	}
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	klog.Infof("Creating Emma CSI driver: %s version: %s", DriverName, DriverVersion)

	return &Driver{
		name:     DriverName,
		version:  DriverVersion,
		nodeID:   nodeID,
		endpoint: endpoint,
	}, nil
}

// SetControllerService sets the controller service
func (d *Driver) SetControllerService(cs csi.ControllerServer) {
	d.controllerService = cs
}

// SetNodeService sets the node service
func (d *Driver) SetNodeService(ns csi.NodeServer) {
	d.nodeService = ns
}

// SetIdentityService sets the identity service
func (d *Driver) SetIdentityService(is csi.IdentityServer) {
	d.identityService = is
}

// SetEmmaClient sets the Emma API client
func (d *Driver) SetEmmaClient(client EmmaClient) {
	d.emmaClient = client
}

// Run starts the CSI driver gRPC server
func (d *Driver) Run() error {
	klog.Infof("Starting Emma CSI driver on endpoint: %s", d.endpoint)

	// Create gRPC server
	d.srv = NewNonBlockingGRPCServer()

	// Start the server
	if err := d.srv.Start(d.endpoint, d.identityService, d.controllerService, d.nodeService); err != nil {
		return err
	}

	// Block forever - signal handler will stop the server
	klog.Info("Emma CSI driver is running")
	select {}
}

// Stop stops the CSI driver
func (d *Driver) Stop() {
	klog.Info("Stopping Emma CSI driver")
	if d.srv != nil {
		d.srv.Stop()
	}
}
