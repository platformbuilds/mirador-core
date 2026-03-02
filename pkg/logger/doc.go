// Package logger provides structured logging capabilities for MIRADOR-CORE
// using the zap logging library.
//
// # Overview
//
// This package wraps [go.uber.org/zap] to provide a consistent logging
// interface across the application with support for:
//   - Structured logging with fields
//   - Log levels (Debug, Info, Warn, Error)
//   - Context-aware logging
//   - Request tracing integration
//
// # Interface
//
// The [Logger] interface defines the logging contract:
//
//	type Logger interface {
//	    Debug(msg string, fields ...zap.Field)
//	    Info(msg string, fields ...zap.Field)
//	    Warn(msg string, fields ...zap.Field)
//	    Error(msg string, fields ...zap.Field)
//	    With(fields ...zap.Field) Logger
//	    ZapLogger() *zap.Logger
//	}
//
// # Usage
//
// Create a logger instance:
//
//	logger, err := logger.New(logger.Config{
//	    Level:       "info",
//	    Development: false,
//	    Encoding:    "json",
//	})
//	if err != nil {
//	    return err
//	}
//	defer logger.Sync()
//
//	// Log with structured fields
//	logger.Info("request processed",
//	    zap.String("method", "GET"),
//	    zap.Int("status", 200),
//	    zap.Duration("latency", time.Since(start)),
//	)
//
//	// Create a child logger with context
//	reqLogger := logger.With(
//	    zap.String("request_id", requestID),
//	    zap.String("user_id", userID),
//	)
//
// # Log Levels
//
// Configure log levels via environment or config:
//   - debug: Verbose debugging information
//   - info: Normal operational messages
//   - warn: Warning conditions
//   - error: Error conditions
//
// # Integration
//
// The logger integrates with:
//   - Gin middleware for request logging
//   - OpenTelemetry for trace correlation
//   - Prometheus for log metrics
package logger
