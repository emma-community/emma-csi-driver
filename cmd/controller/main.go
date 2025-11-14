package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"

	"github.com/emma-csi-driver/pkg/driver"
	"github.com/emma-csi-driver/pkg/emma"
	"github.com/emma-csi-driver/pkg/logging"
	"github.com/emma-csi-driver/pkg/metrics"
)

var (
	endpoint       = flag.String("endpoint", "unix:///var/lib/csi/sockets/pluginproxy/csi.sock", "CSI endpoint")
	emmaAPIURL     = flag.String("emma-api-url", "https://api.emma.ms/external", "Emma API base URL")
	clientID       = flag.String("client-id", "", "Emma API client ID")
	clientSecret   = flag.String("client-secret", "", "Emma API client secret")
	dataCenterID   = flag.String("datacenter-id", "", "Default datacenter ID")
	logLevel       = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	jsonLogs       = flag.Bool("json-logs", false, "Enable JSON log formatting")
	metricsAddr    = flag.String("metrics-addr", ":8080", "Metrics server address")
	version        = "dev"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Configure logging
	logging.SetGlobalLogLevel(*logLevel)
	logging.SetJSONMode(*jsonLogs)
	
	logger := logging.NewLogger("controller")

	if *clientID == "" {
		klog.Fatal("client-id is required")
	}
	if *clientSecret == "" {
		klog.Fatal("client-secret is required")
	}

	logger.Info("Emma CSI Driver Controller starting", map[string]interface{}{
		"version":    version,
		"endpoint":   *endpoint,
		"apiURL":     *emmaAPIURL,
		"datacenter": *dataCenterID,
		"logLevel":   *logLevel,
		"jsonLogs":   *jsonLogs,
	})

	// Start metrics server
	if err := metrics.StartMetricsServer(*metricsAddr); err != nil {
		logger.Error("Failed to start metrics server", err)
		klog.Fatalf("Failed to start metrics server: %v", err)
	}

	// Initialize Emma API client
	logger.Info("Initializing Emma API client")
	emmaClient, err := emma.NewClient(*emmaAPIURL, *clientID, *clientSecret)
	if err != nil {
		logger.Error("Failed to initialize Emma API client", err)
		klog.Fatalf("Failed to initialize Emma API client: %v", err)
	}
	logger.Info("Emma API client initialized successfully")

	// Discover and log available datacenters
	logger.Info("Discovering available datacenters")
	ctx := context.Background()
	datacenters, err := emmaClient.GetDataCenters(ctx)
	if err != nil {
		logger.Warn("Failed to discover datacenters", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		logger.Info("Available datacenters discovered", map[string]interface{}{
			"count": len(datacenters),
		})
		// Log first few datacenters as examples
		for i, dc := range datacenters {
			if i < 10 {
				logger.Info("Datacenter available", map[string]interface{}{
					"id":       dc.GetId(),
					"name":     dc.GetName(),
					"location": dc.GetLocationName(),
					"provider": dc.GetProviderName(),
				})
			}
		}
		if len(datacenters) > 10 {
			logger.Info("Additional datacenters available", map[string]interface{}{
				"count": len(datacenters) - 10,
			})
		}
	}

	// Validate default datacenter if specified
	if *dataCenterID != "" {
		logger.Info("Validating default datacenter", map[string]interface{}{
			"datacenter": *dataCenterID,
		})
		if err := emmaClient.ValidateDataCenter(ctx, *dataCenterID); err != nil {
			logger.Error("Invalid default datacenter", err)
			klog.Fatalf("Invalid default datacenter %s: %v", *dataCenterID, err)
		}
		logger.Info("Default datacenter validated successfully")
	}

	// Initialize CSI driver (use "controller" as node ID for controller service)
	drv, err := driver.NewDriver("controller", *endpoint)
	if err != nil {
		logger.Error("Failed to create driver", err)
		klog.Fatalf("Failed to create driver: %v", err)
	}

	// Initialize services
	identityService := driver.NewIdentityService(drv)
	controllerService := driver.NewControllerService(drv, emmaClient)

	drv.SetEmmaClient(emmaClient)
	drv.SetIdentityService(identityService)
	drv.SetControllerService(controllerService)

	logger.Info("Starting controller service")
	
	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, stopping driver")
		drv.Stop()
		os.Exit(0)
	}()

	// Start the driver
	if err := drv.Run(); err != nil {
		logger.Error("Failed to run driver", err)
		klog.Fatalf("Failed to run driver: %v", err)
	}
}
