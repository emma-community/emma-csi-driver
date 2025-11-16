package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2"
)

// LogLevel represents the logging level
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

// Logger provides structured logging capabilities
type Logger struct {
	component string
	level     LogLevel
	jsonMode  bool
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	Component  string                 `json:"component"`
	Message    string                 `json:"message"`
	Operation  string                 `json:"operation,omitempty"`
	VolumeID   string                 `json:"volumeId,omitempty"`
	NodeID     string                 `json:"nodeId,omitempty"`
	DurationMS int64                  `json:"duration_ms,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
}

var (
	globalLevel    = InfoLevel
	globalJSONMode = false
)

// SetGlobalLogLevel sets the global log level
func SetGlobalLogLevel(level string) {
	switch level {
	case "debug":
		globalLevel = DebugLevel
	case "info":
		globalLevel = InfoLevel
	case "warn":
		globalLevel = WarnLevel
	case "error":
		globalLevel = ErrorLevel
	default:
		globalLevel = InfoLevel
	}
}

// SetJSONMode enables or disables JSON log formatting
func SetJSONMode(enabled bool) {
	globalJSONMode = enabled
}

// NewLogger creates a new logger for a component
func NewLogger(component string) *Logger {
	return &Logger{
		component: component,
		level:     globalLevel,
		jsonMode:  globalJSONMode,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	if !l.shouldLog(DebugLevel) {
		return
	}
	l.log(DebugLevel, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	if !l.shouldLog(InfoLevel) {
		return
	}
	l.log(InfoLevel, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	if !l.shouldLog(WarnLevel) {
		return
	}
	l.log(WarnLevel, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, fields ...map[string]interface{}) {
	if !l.shouldLog(ErrorLevel) {
		return
	}

	mergedFields := make(map[string]interface{})
	if len(fields) > 0 {
		for k, v := range fields[0] {
			mergedFields[k] = v
		}
	}

	if err != nil {
		mergedFields["error"] = err.Error()
	}

	l.log(ErrorLevel, msg, mergedFields)
}

// WithOperation creates a new logger with operation context
func (l *Logger) WithOperation(operation string) *OperationLogger {
	return &OperationLogger{
		logger:    l,
		operation: operation,
		startTime: time.Now(),
		fields:    make(map[string]interface{}),
	}
}

// shouldLog checks if a message at the given level should be logged
func (l *Logger) shouldLog(level LogLevel) bool {
	levelOrder := map[LogLevel]int{
		DebugLevel: 0,
		InfoLevel:  1,
		WarnLevel:  2,
		ErrorLevel: 3,
	}

	return levelOrder[level] >= levelOrder[l.level]
}

// log writes a log entry
func (l *Logger) log(level LogLevel, msg string, fields ...map[string]interface{}) {
	if l.jsonMode {
		entry := LogEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Level:     string(level),
			Component: l.component,
			Message:   msg,
		}

		if len(fields) > 0 {
			entry.Fields = fields[0]

			// Extract common fields
			if op, ok := fields[0]["operation"].(string); ok {
				entry.Operation = op
			}
			if vid, ok := fields[0]["volumeId"].(string); ok {
				entry.VolumeID = vid
			}
			if nid, ok := fields[0]["nodeId"].(string); ok {
				entry.NodeID = nid
			}
			if dur, ok := fields[0]["duration_ms"].(int64); ok {
				entry.DurationMS = dur
			}
			if errStr, ok := fields[0]["error"].(string); ok {
				entry.Error = errStr
			}
		}

		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
			return
		}

		fmt.Println(string(jsonBytes))
	} else {
		// Use klog for non-JSON mode
		fieldsStr := ""
		if len(fields) > 0 {
			fieldsBytes, _ := json.Marshal(fields[0])
			fieldsStr = " " + string(fieldsBytes)
		}

		switch level {
		case DebugLevel:
			klog.V(4).Infof("[%s] %s%s", l.component, msg, fieldsStr)
		case InfoLevel:
			klog.Infof("[%s] %s%s", l.component, msg, fieldsStr)
		case WarnLevel:
			klog.Warningf("[%s] %s%s", l.component, msg, fieldsStr)
		case ErrorLevel:
			klog.Errorf("[%s] %s%s", l.component, msg, fieldsStr)
		}
	}
}

// OperationLogger provides contextual logging for operations
type OperationLogger struct {
	logger    *Logger
	operation string
	startTime time.Time
	fields    map[string]interface{}
}

// WithField adds a field to the operation logger
func (ol *OperationLogger) WithField(key string, value interface{}) *OperationLogger {
	ol.fields[key] = value
	return ol
}

// WithVolumeID adds volume ID to the operation logger
func (ol *OperationLogger) WithVolumeID(volumeID string) *OperationLogger {
	ol.fields["volumeId"] = volumeID
	return ol
}

// WithNodeID adds node ID to the operation logger
func (ol *OperationLogger) WithNodeID(nodeID string) *OperationLogger {
	ol.fields["nodeId"] = nodeID
	return ol
}

// Debug logs a debug message with operation context
func (ol *OperationLogger) Debug(msg string) {
	fields := ol.getFields()
	fields["operation"] = ol.operation
	ol.logger.Debug(msg, fields)
}

// Info logs an info message with operation context
func (ol *OperationLogger) Info(msg string) {
	fields := ol.getFields()
	fields["operation"] = ol.operation
	ol.logger.Info(msg, fields)
}

// Warn logs a warning message with operation context
func (ol *OperationLogger) Warn(msg string) {
	fields := ol.getFields()
	fields["operation"] = ol.operation
	ol.logger.Warn(msg, fields)
}

// Error logs an error message with operation context
func (ol *OperationLogger) Error(msg string, err error) {
	fields := ol.getFields()
	fields["operation"] = ol.operation
	if err != nil {
		fields["error"] = err.Error()
	}
	ol.logger.Error(msg, err, fields)
}

// Complete logs operation completion with duration
func (ol *OperationLogger) Complete(msg string) {
	duration := time.Since(ol.startTime)
	fields := ol.getFields()
	fields["operation"] = ol.operation
	fields["duration_ms"] = duration.Milliseconds()
	ol.logger.Info(msg, fields)
}

// CompleteWithError logs operation completion with error
func (ol *OperationLogger) CompleteWithError(msg string, err error) {
	duration := time.Since(ol.startTime)
	fields := ol.getFields()
	fields["operation"] = ol.operation
	fields["duration_ms"] = duration.Milliseconds()
	if err != nil {
		fields["error"] = err.Error()
	}
	ol.logger.Error(msg, err, fields)
}

// getFields returns a copy of the fields map
func (ol *OperationLogger) getFields() map[string]interface{} {
	fields := make(map[string]interface{})
	for k, v := range ol.fields {
		fields[k] = v
	}
	return fields
}
