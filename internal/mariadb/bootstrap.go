// Package mariadb provides MariaDB integration for mirador-core.
// This file contains bootstrapping logic for creating tables and syncing
// data sources from config.yaml to MariaDB.
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/platformbuilds/mirador-core/internal/config"
)

// BootstrapConfig controls the bootstrapping behavior.
type BootstrapConfig struct {
	// CreateTablesIfMissing creates the data_sources and kpis tables if they don't exist
	CreateTablesIfMissing bool
	// SyncDataSourcesFromConfig syncs data sources from config.yaml to MariaDB
	SyncDataSourcesFromConfig bool
}

// Bootstrap handles one-time setup of MariaDB tables and initial data sync.
type Bootstrap struct {
	client       *Client
	cfg          *config.Config
	logger       *zap.Logger
	bootstrapCfg BootstrapConfig
}

// NewBootstrap creates a new Bootstrap instance.
func NewBootstrap(client *Client, cfg *config.Config, logger *zap.Logger, bootstrapCfg BootstrapConfig) *Bootstrap {
	return &Bootstrap{
		client:       client,
		cfg:          cfg,
		logger:       logger,
		bootstrapCfg: bootstrapCfg,
	}
}

// Run executes the bootstrap process:
// 1. Creates tables if they don't exist (when enabled)
// 2. Syncs data sources from config.yaml to MariaDB (when enabled)
func (b *Bootstrap) Run(ctx context.Context) error {
	if !b.client.IsEnabled() {
		b.logger.Info("mariadb: bootstrap skipped - MariaDB is disabled")
		return nil
	}

	if !b.client.IsConnected() {
		if err := b.client.Reconnect(); err != nil {
			return fmt.Errorf("mariadb: bootstrap failed - cannot connect: %w", err)
		}
	}

	// Step 1: Create tables if missing
	if b.bootstrapCfg.CreateTablesIfMissing {
		if err := b.createTablesIfMissing(ctx); err != nil {
			return fmt.Errorf("mariadb: failed to create tables: %w", err)
		}
	}

	// Step 2: Sync data sources from config
	if b.bootstrapCfg.SyncDataSourcesFromConfig {
		if err := b.syncDataSourcesFromConfig(ctx); err != nil {
			return fmt.Errorf("mariadb: failed to sync data sources: %w", err)
		}
	}

	return nil
}

// createTablesIfMissing creates the data_sources and kpis tables if they don't exist.
func (b *Bootstrap) createTablesIfMissing(ctx context.Context) error {
	db := b.client.DB()
	if db == nil {
		return ErrMariaDBNotConnected
	}

	// Check and create data_sources table
	if err := b.createDataSourcesTableIfMissing(ctx, db); err != nil {
		return err
	}

	// Check and create kpis table
	if err := b.createKPIsTableIfMissing(ctx, db); err != nil {
		return err
	}

	return nil
}

// createDataSourcesTableIfMissing creates the data_sources table if it doesn't exist.
func (b *Bootstrap) createDataSourcesTableIfMissing(ctx context.Context, db *sql.DB) error {
	// Check if table exists
	var tableName string
	err := db.QueryRowContext(ctx, `
		SELECT TABLE_NAME FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'data_sources'
	`).Scan(&tableName)

	if err == nil {
		b.logger.Debug("mariadb: data_sources table already exists")
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check data_sources table: %w", err)
	}

	// Table doesn't exist, create it
	b.logger.Info("mariadb: creating data_sources table")

	createSQL := `
		CREATE TABLE data_sources (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			project_identifier VARCHAR(255),
			url VARCHAR(500) NOT NULL,
			api_key VARCHAR(255),
			username VARCHAR(255),
			password VARCHAR(255),
			health_url VARCHAR(500),
			health_expected_status INT DEFAULT 200,
			health_body_type VARCHAR(50),
			health_body_match_mode VARCHAR(50),
			health_body_text_pattern VARCHAR(255),
			health_body_json_key VARCHAR(255),
			health_body_json_expected_value VARCHAR(255),
			health_check_interval_ms INT DEFAULT 60000,
			is_active BOOLEAN DEFAULT TRUE,
			ai_config JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			
			INDEX idx_data_sources_type (type),
			INDEX idx_data_sources_is_active (is_active),
			INDEX idx_data_sources_type_active (type, is_active)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`

	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("failed to create data_sources table: %w", err)
	}

	b.logger.Info("mariadb: data_sources table created successfully")
	return nil
}

// createKPIsTableIfMissing creates the kpis table if it doesn't exist.
func (b *Bootstrap) createKPIsTableIfMissing(ctx context.Context, db *sql.DB) error {
	// Check if table exists
	var tableName string
	err := db.QueryRowContext(ctx, `
		SELECT TABLE_NAME FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'kpis'
	`).Scan(&tableName)

	if err == nil {
		b.logger.Debug("mariadb: kpis table already exists")
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check kpis table: %w", err)
	}

	// Table doesn't exist, create it
	b.logger.Info("mariadb: creating kpis table")

	createSQL := `
		CREATE TABLE kpis (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			data_type VARCHAR(50) NOT NULL DEFAULT 'timeseries',
			definition TEXT,
			formula TEXT,
			data_source_id VARCHAR(36),
			kpi_datastore_id VARCHAR(36),
			unit VARCHAR(50),
			thresholds JSON,
			refresh_interval INT DEFAULT 60,
			is_shared BOOLEAN DEFAULT FALSE,
			user_id VARCHAR(36) NOT NULL DEFAULT 'system',
			namespace VARCHAR(100) NOT NULL DEFAULT 'default',
			kind VARCHAR(50) NOT NULL DEFAULT 'metric',
			layer VARCHAR(50) NOT NULL DEFAULT 'infrastructure',
			classifier VARCHAR(100),
			signal_type VARCHAR(50) NOT NULL DEFAULT 'counter',
			sentiment VARCHAR(50),
			component_type VARCHAR(100),
			query JSON,
			examples TEXT,
			dimensions_hint JSON,
			query_type VARCHAR(50),
			datastore VARCHAR(100),
			service_family VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			
			INDEX idx_kpis_namespace (namespace),
			INDEX idx_kpis_kind (kind),
			INDEX idx_kpis_layer (layer),
			INDEX idx_kpis_signal_type (signal_type),
			INDEX idx_kpis_data_source_id (data_source_id),
			INDEX idx_kpis_updated_at (updated_at),
			
			FOREIGN KEY (data_source_id) REFERENCES data_sources(id) ON DELETE SET NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`

	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("failed to create kpis table: %w", err)
	}

	b.logger.Info("mariadb: kpis table created successfully")
	return nil
}

// syncDataSourcesFromConfig syncs data sources from config.yaml to MariaDB.
// For each data source in config:
// 1. Check if it exists in MariaDB (by URL match)
// 2. If exists, validate and log
// 3. If not exists, create it
func (b *Bootstrap) syncDataSourcesFromConfig(ctx context.Context) error {
	db := b.client.DB()
	if db == nil {
		return ErrMariaDBNotConnected
	}

	// Collect all data sources from config
	dataSources := b.collectDataSourcesFromConfig()

	if len(dataSources) == 0 {
		b.logger.Debug("mariadb: no data sources found in config.yaml to sync")
		return nil
	}

	b.logger.Info("mariadb: syncing data sources from config.yaml",
		zap.Int("count", len(dataSources)))

	synced := 0
	validated := 0

	for i := range dataSources {
		ds := &dataSources[i]
		exists, existingID, err := b.dataSourceExistsByURL(ctx, db, ds.URL)
		if err != nil {
			b.logger.Warn("mariadb: failed to check data source existence",
				zap.String("url", ds.URL),
				zap.Error(err))
			continue
		}

		if exists {
			b.logger.Debug("mariadb: data source already exists, validated",
				zap.String("name", ds.Name),
				zap.String("url", ds.URL),
				zap.String("id", existingID))
			validated++
			continue
		}

		// Create the data source
		if err := b.createDataSource(ctx, db, ds); err != nil {
			b.logger.Warn("mariadb: failed to create data source",
				zap.String("name", ds.Name),
				zap.String("url", ds.URL),
				zap.Error(err))
			continue
		}

		b.logger.Info("mariadb: created data source from config",
			zap.String("name", ds.Name),
			zap.String("type", string(ds.Type)),
			zap.String("url", ds.URL))
		synced++
	}

	b.logger.Info("mariadb: data source sync complete",
		zap.Int("synced", synced),
		zap.Int("validated", validated),
		zap.Int("total", len(dataSources)))

	return nil
}

// ConfigDataSource represents a data source extracted from config.yaml.
type ConfigDataSource struct {
	Name     string
	Type     DataSourceType
	URL      string
	Username string
	Password string //nolint:gosec // G117: Intentional credential transport from config
}

// collectDataSourcesFromConfig extracts all data sources from the config.
func (b *Bootstrap) collectDataSourcesFromConfig() []ConfigDataSource {
	var sources []ConfigDataSource

	// VictoriaMetrics endpoints
	sources = b.collectVictoriaMetricsSources(sources)

	// Additional metrics sources
	for i := range b.cfg.Database.MetricsSources {
		src := &b.cfg.Database.MetricsSources[i]
		sources = b.collectEndpointSources(sources, src.Endpoints, src.Name, "metrics-source",
			DataSourceTypePrometheus, src.Username, src.Password)
	}

	// VictoriaLogs endpoints
	sources = b.collectVictoriaLogsSources(sources)

	// Additional logs sources
	for i := range b.cfg.Database.LogsSources {
		src := &b.cfg.Database.LogsSources[i]
		sources = b.collectEndpointSources(sources, src.Endpoints, src.Name, "logs-source",
			DataSourceTypeVictoriaLogs, src.Username, src.Password)
	}

	// VictoriaTraces endpoints
	sources = b.collectVictoriaTracesSources(sources)

	// Additional traces sources
	for i := range b.cfg.Database.TracesSources {
		src := &b.cfg.Database.TracesSources[i]
		sources = b.collectEndpointSources(sources, src.Endpoints, src.Name, "traces-source",
			DataSourceTypeVictoriaTraces, src.Username, src.Password)
	}

	return sources
}

// collectVictoriaMetricsSources extracts VictoriaMetrics endpoints from config.
func (b *Bootstrap) collectVictoriaMetricsSources(sources []ConfigDataSource) []ConfigDataSource {
	vm := &b.cfg.Database.VictoriaMetrics
	return b.collectEndpointSources(sources, vm.Endpoints, vm.Name, "victoriametrics",
		DataSourceTypePrometheus, vm.Username, vm.Password)
}

// collectVictoriaLogsSources extracts VictoriaLogs endpoints from config.
func (b *Bootstrap) collectVictoriaLogsSources(sources []ConfigDataSource) []ConfigDataSource {
	vl := &b.cfg.Database.VictoriaLogs
	return b.collectEndpointSources(sources, vl.Endpoints, vl.Name, "victorialogs",
		DataSourceTypeVictoriaLogs, vl.Username, vl.Password)
}

// collectVictoriaTracesSources extracts VictoriaTraces endpoints from config.
func (b *Bootstrap) collectVictoriaTracesSources(sources []ConfigDataSource) []ConfigDataSource {
	vt := &b.cfg.Database.VictoriaTraces
	return b.collectEndpointSources(sources, vt.Endpoints, vt.Name, "victoriatraces",
		DataSourceTypeVictoriaTraces, vt.Username, vt.Password)
}

// collectEndpointSources is a helper that creates ConfigDataSource entries from endpoints.
func (b *Bootstrap) collectEndpointSources(
	sources []ConfigDataSource,
	endpoints []string,
	configName, defaultPrefix string,
	dsType DataSourceType,
	username, password string,
) []ConfigDataSource {
	for i, endpoint := range endpoints {
		name := configName
		if name == "" {
			name = fmt.Sprintf("%s-%d", defaultPrefix, i+1)
		} else if len(endpoints) > 1 {
			name = fmt.Sprintf("%s-%d", name, i+1)
		}
		sources = append(sources, ConfigDataSource{
			Name:     name,
			Type:     dsType,
			URL:      endpoint,
			Username: username,
			Password: password,
		})
	}
	return sources
}

// dataSourceExistsByURL checks if a data source with the given URL exists.
// Returns: exists, existingID, error
func (b *Bootstrap) dataSourceExistsByURL(ctx context.Context, db *sql.DB, url string) (exists bool, existingID string, err error) {
	var id string
	err = db.QueryRowContext(ctx, `
		SELECT id FROM data_sources WHERE url = ? LIMIT 1
	`, url).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, id, nil
}

// createDataSource creates a new data source in MariaDB.
func (b *Bootstrap) createDataSource(ctx context.Context, db *sql.DB, ds *ConfigDataSource) error {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := db.ExecContext(ctx, `
		INSERT INTO data_sources (
			id, name, type, url, username, password, 
			is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, TRUE, ?, ?)
	`, id, ds.Name, strings.ToLower(string(ds.Type)), ds.URL, ds.Username, ds.Password, now, now)

	return err
}

// CreateDatabase creates the tenant database if it doesn't exist.
// This requires a connection with CREATE DATABASE privileges.
// Returns true if the database was created, false if it already existed.
func (b *Bootstrap) CreateDatabase(ctx context.Context) (bool, error) {
	if !b.client.IsEnabled() {
		return false, nil
	}

	// For creating a database, we need to connect without specifying a database
	// This is handled by connecting to 'mysql' or 'information_schema' system DB
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?parseTime=true&loc=UTC&timeout=10s",
		b.client.cfg.Username,
		b.client.cfg.Password,
		b.client.cfg.Host,
		b.client.cfg.Port,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return false, fmt.Errorf("failed to connect for database creation: %w", err)
	}
	defer db.Close()

	// Check if database exists
	var dbName string
	err = db.QueryRowContext(ctx, `
		SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?
	`, b.client.cfg.Database).Scan(&dbName)

	if err == nil {
		b.logger.Debug("mariadb: database already exists",
			zap.String("database", b.client.cfg.Database))
		return false, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("failed to check database existence: %w", err)
	}

	// Create the database
	b.logger.Info("mariadb: creating database",
		zap.String("database", b.client.cfg.Database))

	// Use backticks for database name to handle special characters
	createSQL := fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		b.client.cfg.Database)

	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return false, fmt.Errorf("failed to create database: %w", err)
	}

	b.logger.Info("mariadb: database created successfully",
		zap.String("database", b.client.cfg.Database))

	return true, nil
}
