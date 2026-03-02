// Package mariadb provides a read-only MariaDB client for mirador-core.
// mirador-core connects to the same MariaDB used by mirador-ui to read
// data sources and KPI definitions. Each deployment is tenant-specific.
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL/MariaDB driver
	"go.uber.org/zap"

	"github.com/platformbuilds/mirador-core/internal/config"
)

// Static errors for err113 compliance
var (
	ErrMariaDBDisabled     = errors.New("mariadb: client is disabled")
	ErrMariaDBNotConnected = errors.New("mariadb: not connected")
	ErrMariaDBPingFailed   = errors.New("mariadb: ping failed")
)

// Default connection pool settings
const (
	defaultMaxOpenConns    = 10
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 5 * time.Minute
	defaultPingTimeout     = 10 * time.Second
)

// Client wraps a read-only MariaDB connection pool.
type Client struct {
	db     *sql.DB
	cfg    config.MariaDBConfig
	logger *zap.Logger

	mu        sync.RWMutex
	connected bool
	lastError error
}

// NewClient creates a new MariaDB client from configuration.
// Returns a disabled client if cfg.Enabled is false.
func NewClient(cfg *config.MariaDBConfig, logger *zap.Logger) (*Client, error) {
	if cfg == nil {
		return &Client{logger: logger}, nil
	}
	c := &Client{
		cfg:    *cfg,
		logger: logger,
	}

	if !cfg.Enabled {
		if logger != nil {
			logger.Info("mariadb: client disabled by configuration")
		}
		return c, nil
	}

	if err := c.connect(); err != nil {
		// Don't fail startup; log and allow graceful degradation
		if logger != nil {
			logger.Error("mariadb: initial connection failed", zap.Error(err))
		}
		c.lastError = err
		return c, nil
	}

	return c, nil
}

// connect establishes the database connection.
func (c *Client) connect() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC&timeout=10s&readTimeout=30s&writeTimeout=30s",
		c.cfg.Username,
		c.cfg.Password,
		c.cfg.Host,
		c.cfg.Port,
		c.cfg.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("mariadb: failed to open connection: %w", err)
	}

	// Configure connection pool
	if c.cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(c.cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(defaultMaxOpenConns)
	}

	if c.cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(c.cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(defaultMaxIdleConns)
	}

	if c.cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(c.cfg.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(defaultConnMaxLifetime)
	}

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), defaultPingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close() // Best-effort close on failure
		return fmt.Errorf("mariadb: ping failed: %w", err)
	}

	c.mu.Lock()
	c.db = db
	c.connected = true
	c.lastError = nil
	c.mu.Unlock()

	if c.logger != nil {
		c.logger.Info("mariadb: connected successfully",
			zap.String("host", c.cfg.Host),
			zap.Int("port", c.cfg.Port),
			zap.String("database", c.cfg.Database),
		)
	}

	return nil
}

// IsEnabled returns true if MariaDB is enabled in configuration.
func (c *Client) IsEnabled() bool {
	return c.cfg.Enabled
}

// IsConnected returns true if the client has an active connection.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// LastError returns the last connection error, if any.
func (c *Client) LastError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

// DB returns the underlying sql.DB for direct queries.
// Returns nil if not connected or disabled.
func (c *Client) DB() *sql.DB {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db
}

// Ping verifies the connection is still alive.
func (c *Client) Ping(ctx context.Context) error {
	if !c.cfg.Enabled {
		return ErrMariaDBDisabled
	}

	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db == nil {
		return ErrMariaDBNotConnected
	}

	if err := db.PingContext(ctx); err != nil {
		c.mu.Lock()
		c.connected = false
		c.lastError = err
		c.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrMariaDBPingFailed, err)
	}

	c.mu.Lock()
	c.connected = true
	c.lastError = nil
	c.mu.Unlock()

	return nil
}

// Reconnect attempts to re-establish the connection.
func (c *Client) Reconnect() error {
	if !c.cfg.Enabled {
		return ErrMariaDBDisabled
	}

	c.mu.Lock()
	if c.db != nil {
		_ = c.db.Close() // Best-effort close before reconnect
		c.db = nil
	}
	c.connected = false
	c.mu.Unlock()

	return c.connect()
}

// Close closes the database connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		err := c.db.Close()
		c.db = nil
		c.connected = false
		return err
	}
	return nil
}

// Health returns a health status suitable for health check endpoints.
type Health struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Host      string `json:"host"`
	Database  string `json:"database"`
	Error     string `json:"error,omitempty"`
}

// HealthCheck returns the current health status.
func (c *Client) HealthCheck(ctx context.Context) Health {
	h := Health{
		Enabled:  c.cfg.Enabled,
		Host:     c.cfg.Host,
		Database: c.cfg.Database,
	}

	if !c.cfg.Enabled {
		return h
	}

	if err := c.Ping(ctx); err != nil {
		h.Error = err.Error()
		h.Connected = false
	} else {
		h.Connected = true
	}

	return h
}

// Config returns the current configuration (password redacted).
func (c *Client) Config() config.MariaDBConfig {
	cfg := c.cfg
	cfg.Password = "***"
	return cfg
}
