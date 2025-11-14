package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

const (
	namespace = "emma_csi"
)

var (
	// Operation metrics
	operationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "operations_total",
			Help:      "Total number of CSI operations",
		},
		[]string{"operation", "status"},
	)

	operationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "operation_duration_seconds",
			Help:      "Duration of CSI operations in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~102s
		},
		[]string{"operation"},
	)

	// API request metrics
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "api_requests_total",
			Help:      "Total number of Emma API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "api_request_duration_seconds",
			Help:      "Duration of Emma API requests in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.05, 2, 10), // 0.05s to ~51s
		},
		[]string{"method", "endpoint"},
	)

	// Volume state metrics
	volumesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "volumes_total",
			Help:      "Total number of volumes by status",
		},
		[]string{"status"},
	)

	// Volume operation specific metrics
	volumeAttachDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "volume_attach_duration_seconds",
			Help:      "Duration of volume attach operations in seconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~1024s
		},
	)

	volumeDetachDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "volume_detach_duration_seconds",
			Help:      "Duration of volume detach operations in seconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~1024s
		},
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(operationsTotal)
	prometheus.MustRegister(operationDuration)
	prometheus.MustRegister(apiRequestsTotal)
	prometheus.MustRegister(apiRequestDuration)
	prometheus.MustRegister(volumesTotal)
	prometheus.MustRegister(volumeAttachDuration)
	prometheus.MustRegister(volumeDetachDuration)
}

// RecordOperation records a CSI operation
func RecordOperation(operation string, status string, duration time.Duration) {
	operationsTotal.WithLabelValues(operation, status).Inc()
	operationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordAPIRequest records an Emma API request
func RecordAPIRequest(method, endpoint, status string, duration time.Duration) {
	apiRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	apiRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// SetVolumeCount sets the count of volumes by status
func SetVolumeCount(status string, count float64) {
	volumesTotal.WithLabelValues(status).Set(count)
}

// RecordVolumeAttach records a volume attach operation duration
func RecordVolumeAttach(duration time.Duration) {
	volumeAttachDuration.Observe(duration.Seconds())
}

// RecordVolumeDetach records a volume detach operation duration
func RecordVolumeDetach(duration time.Duration) {
	volumeDetachDuration.Observe(duration.Seconds())
}

// OperationTimer helps track operation duration
type OperationTimer struct {
	operation string
	startTime time.Time
}

// NewOperationTimer creates a new operation timer
func NewOperationTimer(operation string) *OperationTimer {
	return &OperationTimer{
		operation: operation,
		startTime: time.Now(),
	}
}

// ObserveSuccess records a successful operation
func (t *OperationTimer) ObserveSuccess() {
	duration := time.Since(t.startTime)
	RecordOperation(t.operation, "success", duration)
}

// ObserveError records a failed operation
func (t *OperationTimer) ObserveError() {
	duration := time.Since(t.startTime)
	RecordOperation(t.operation, "error", duration)
}

// APIRequestTimer helps track API request duration
type APIRequestTimer struct {
	method    string
	endpoint  string
	startTime time.Time
}

// NewAPIRequestTimer creates a new API request timer
func NewAPIRequestTimer(method, endpoint string) *APIRequestTimer {
	return &APIRequestTimer{
		method:    method,
		endpoint:  endpoint,
		startTime: time.Now(),
	}
}

// Observe records the API request with status
func (t *APIRequestTimer) Observe(statusCode int) {
	duration := time.Since(t.startTime)
	status := http.StatusText(statusCode)
	if status == "" {
		status = "unknown"
	}
	RecordAPIRequest(t.method, t.endpoint, status, duration)
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(addr string) error {
	klog.Infof("Starting metrics server on %s", addr)
	
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	
	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Metrics server error: %v", err)
		}
	}()
	
	klog.Info("Metrics server started successfully")
	return nil
}
