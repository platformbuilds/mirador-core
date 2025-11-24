package logging

import (
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
	"go.uber.org/zap"
)

// Logger is a minimal logging interface used across the server and handlers.
// It mirrors the public surface used by the rest of the codebase so callers
// can depend on internal/logging rather than pkg/logger (depguard rule).
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
}

// New returns a Logger backed by a zap SugaredLogger. For now this produces
// a no-op zap logger; callers may provide their own implementations in tests.
func New(level string) Logger {
	// Keep this simple and return a zap-backed adapter. Production callers
	// typically supply the project-wide logger from `pkg/logger` directly,
	// so this constructor is a convenience for local tests and small tools.
	zl := zap.NewNop()
	return &zapAdapter{logger: zl}
}

// ExtractZapLogger attempts to obtain an underlying *zap.Logger from any
// provided value. If the value exposes a ZapLogger() *zap.Logger method,
// that logger is returned; otherwise a no-op *zap.Logger is returned.
func ExtractZapLogger(v interface{}) *zap.Logger {
	if zl, ok := v.(interface{ ZapLogger() *zap.Logger }); ok {
		return zl.ZapLogger()
	}
	if za, ok := v.(*zapAdapter); ok {
		return za.logger
	}
	return zap.NewNop()
}

// FromCoreLogger wraps the project core logger (pkg/logger.Logger) and
// returns an internal/logging.Logger adapter. This provides a single
// logging interface usable across internal packages while allowing the
// cmd/ main packages to construct the concrete core logger implementation.
func FromCoreLogger(core corelogger.Logger) Logger {
	if core == nil {
		return New("info")
	}
	return &coreAdapter{core: core}
}

type coreAdapter struct {
	core corelogger.Logger
}

func (c *coreAdapter) Info(msg string, fields ...interface{})  { c.core.Info(msg, fields...) }
func (c *coreAdapter) Error(msg string, fields ...interface{}) { c.core.Error(msg, fields...) }
func (c *coreAdapter) Warn(msg string, fields ...interface{})  { c.core.Warn(msg, fields...) }
func (c *coreAdapter) Debug(msg string, fields ...interface{}) { c.core.Debug(msg, fields...) }
func (c *coreAdapter) Fatal(msg string, fields ...interface{}) { c.core.Fatal(msg, fields...) }

// ZapLogger returns the underlying *zap.Logger from the core logger when
// available. This preserves the previous behavior where ExtractZapLogger
// could retrieve a real zap logger; otherwise a no-op is returned.
func (c *coreAdapter) ZapLogger() *zap.Logger {
	if zl, ok := c.core.(interface{ ZapLogger() *zap.Logger }); ok {
		return zl.ZapLogger()
	}
	return zap.NewNop()
}

type zapAdapter struct {
	logger *zap.Logger
}

func (z *zapAdapter) Info(msg string, fields ...interface{}) {
	z.logger.Sugar().Infow(msg, fields...)
}

func (z *zapAdapter) Error(msg string, fields ...interface{}) {
	z.logger.Sugar().Errorw(msg, fields...)
}

func (z *zapAdapter) Warn(msg string, fields ...interface{}) {
	z.logger.Sugar().Warnw(msg, fields...)
}

func (z *zapAdapter) Debug(msg string, fields ...interface{}) {
	z.logger.Sugar().Debugw(msg, fields...)
}

func (z *zapAdapter) Fatal(msg string, fields ...interface{}) {
	z.logger.Sugar().Fatalw(msg, fields...)
}
