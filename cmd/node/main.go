package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"

	"github.com/emma-csi-driver/pkg/driver"
	"github.com/emma-csi-driver/pkg/logging"
	"github.com/emma-csi-driver/pkg/metrics"
)

var (
	endpoint    = flag.String("endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	nodeID      = flag.String("node-id", "", "Node ID (VM ID in Emma)")
	logLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	jsonLogs    = flag.Bool("json-logs", false, "Enable JSON log formatting")
	metricsAddr = flag.String("metrics-addr", ":8080", "Metrics server address")
	version     = "dev"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Configure logging
	logging.SetGlobalLogLevel(*logLevel)
	logging.SetJSONMode(*jsonLogs)

	logger := logging.NewLogger("node")

	if *nodeID == "" {
		// Try to get node ID from environment or metadata service
		*nodeID = os.Getenv("NODE_ID")
		if *nodeID == "" {
			klog.Fatal("node-id is required (set via flag or NODE_ID environment variable)")
		}
	}

	logger.Info("Emma CSI Driver Node Plugin starting", map[string]interface{}{
		"version":  version,
		"endpoint": *endpoint,
		"nodeId":   *nodeID,
		"logLevel": *logLevel,
		"jsonLogs": *jsonLogs,
	})

	// Start metrics server
	if err := metrics.StartMetricsServer(*metricsAddr); err != nil {
		logger.Error("Failed to start metrics server", err)
		klog.Fatalf("Failed to start metrics server: %v", err)
	}

	// Initialize CSI driver
	drv, err := driver.NewDriver(*nodeID, *endpoint)
	if err != nil {
		logger.Error("Failed to create driver", err)
		klog.Fatalf("Failed to create driver: %v", err)
	}

	// Initialize services
	identityService := driver.NewIdentityService(drv)
	nodeService := driver.NewNodeService(drv)

	drv.SetIdentityService(identityService)
	drv.SetNodeService(nodeService)

	logger.Info("Node service started successfully")

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, stopping driver")
		drv.Stop()
		os.Exit(0)
	}()

	// Start the driver (this will block)
	if err := drv.Run(); err != nil {
		logger.Error("Failed to run driver", err)
		klog.Fatalf("Failed to run driver: %v", err)
	}
}
