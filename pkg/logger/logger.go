package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
}

type zapLogger struct {
	logger *zap.SugaredLogger
}

func New(level string) Logger {
	config := zap.NewProductionConfig()

	// Set log level
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	}

	// Custom encoder config for MIRADOR
	config.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	return &zapLogger{
		logger: logger.Sugar(),
	}
}

func (l *zapLogger) Info(msg string, fields ...interface{}) {
	l.logger.Infow(msg, fields...)
}

func (l *zapLogger) Error(msg string, fields ...interface{}) {
	l.logger.Errorw(msg, fields...)
}

func (l *zapLogger) Warn(msg string, fields ...interface{}) {
	l.logger.Warnw(msg, fields...)
}

func (l *zapLogger) Debug(msg string, fields ...interface{}) {
	l.logger.Debugw(msg, fields...)
}

func (l *zapLogger) Fatal(msg string, fields ...interface{}) {
	l.logger.Fatalw(msg, fields...)
}

// MockLogger is a test logger that captures output to a buffer
type MockLogger struct {
	output *strings.Builder
}

func NewMockLogger(output *strings.Builder) Logger {
	return &MockLogger{output: output}
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	m.output.WriteString("[INFO] " + msg + "\n")
}

func (m *MockLogger) Error(msg string, fields ...interface{}) {
	m.output.WriteString("[ERROR] " + msg + "\n")
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	m.output.WriteString("[WARN] " + msg + "\n")
}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	m.output.WriteString("[DEBUG] " + msg + "\n")
}

func (m *MockLogger) Fatal(msg string, fields ...interface{}) {
	m.output.WriteString("[FATAL] " + msg + "\n")
}
