package mariadb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// DataSourceType represents the type of data source.
type DataSourceType string

const (
	DataSourceTypePrometheus      DataSourceType = "prometheus"
	DataSourceTypeVictoriaLogs    DataSourceType = "victorialogs"
	DataSourceTypeJaeger          DataSourceType = "jaeger"
	DataSourceTypeMiradorCore     DataSourceType = "miradorcore"
	DataSourceTypeMiradorSecurity DataSourceType = "miradorsecurity"
	DataSourceTypeAIEngine        DataSourceType = "aiengine"
	DataSourceTypeVictoriaTraces  DataSourceType = "victoriatraces"
)

// DataSource represents a data source record from MariaDB.
type DataSource struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Type              DataSourceType `json:"type"`
	ProjectIdentifier string         `json:"project_identifier,omitempty"`
	URL               string         `json:"url"`
	APIKey            string         `json:"api_key,omitempty"` //nolint:gosec // G117: Intentional mapping to DB field
	Username          string         `json:"username,omitempty"`
	Password          string         `json:"password,omitempty"` //nolint:gosec // G117: Intentional mapping to DB field`

	// Health check configuration
	HealthURL                 string `json:"health_url,omitempty"`
	HealthExpectedStatus      int    `json:"health_expected_status"`
	HealthBodyType            string `json:"health_body_type,omitempty"`
	HealthBodyMatchMode       string `json:"health_body_match_mode,omitempty"`
	HealthBodyTextPattern     string `json:"health_body_text_pattern,omitempty"`
	HealthBodyJSONKey         string `json:"health_body_json_key,omitempty"`
	HealthBodyJSONExpectedVal string `json:"health_body_json_expected_value,omitempty"`
	HealthCheckIntervalMs     int    `json:"health_check_interval_ms"`

	IsActive  bool            `json:"is_active"`
	AIConfig  json.RawMessage `json:"ai_config,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Static errors for DataSource operations
var (
	ErrDataSourceNotFound   = errors.New("mariadb: data source not found")
	ErrNoActiveDataSources  = errors.New("mariadb: no active data sources found")
	ErrDataSourceQueryError = errors.New("mariadb: query failed")
)

// DataSourceRepo provides read-only access to data sources in MariaDB.
type DataSourceRepo struct {
	client *Client
	logger *zap.Logger
}

// NewDataSourceRepo creates a new DataSourceRepo.
func NewDataSourceRepo(client *Client, logger *zap.Logger) *DataSourceRepo {
	return &DataSourceRepo{
		client: client,
		logger: logger,
	}
}

// GetByID retrieves a single data source by ID.
func (r *DataSourceRepo) GetByID(ctx context.Context, id string) (*DataSource, error) {
	if !r.client.IsEnabled() {
		return nil, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return nil, ErrMariaDBNotConnected
	}

	query := `
		SELECT id, name, type, project_identifier, url, api_key, username, password,
		       health_url, health_expected_status, health_body_type, health_body_match_mode,
		       health_body_text_pattern, health_body_json_key, health_body_json_expected_value,
		       health_check_interval_ms, is_active, ai_config, created_at, updated_at
		FROM data_sources
		WHERE id = ?
	`

	row := db.QueryRowContext(ctx, query, id)
	ds, err := r.scanDataSource(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDataSourceNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrDataSourceQueryError, err)
	}

	return ds, nil
}

// ListByType retrieves all active data sources of a given type.
func (r *DataSourceRepo) ListByType(ctx context.Context, dsType DataSourceType) ([]*DataSource, error) {
	if !r.client.IsEnabled() {
		return nil, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return nil, ErrMariaDBNotConnected
	}

	query := `
		SELECT id, name, type, project_identifier, url, api_key, username, password,
		       health_url, health_expected_status, health_body_type, health_body_match_mode,
		       health_body_text_pattern, health_body_json_key, health_body_json_expected_value,
		       health_check_interval_ms, is_active, ai_config, created_at, updated_at
		FROM data_sources
		WHERE type = ? AND is_active = TRUE
		ORDER BY name ASC
	`

	rows, err := db.QueryContext(ctx, query, string(dsType))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDataSourceQueryError, err)
	}
	defer rows.Close()

	return r.scanDataSources(rows)
}

// ListAll retrieves all active data sources.
func (r *DataSourceRepo) ListAll(ctx context.Context) ([]*DataSource, error) {
	if !r.client.IsEnabled() {
		return nil, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return nil, ErrMariaDBNotConnected
	}

	query := `
		SELECT id, name, type, project_identifier, url, api_key, username, password,
		       health_url, health_expected_status, health_body_type, health_body_match_mode,
		       health_body_text_pattern, health_body_json_key, health_body_json_expected_value,
		       health_check_interval_ms, is_active, ai_config, created_at, updated_at
		FROM data_sources
		WHERE is_active = TRUE
		ORDER BY type, name ASC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDataSourceQueryError, err)
	}
	defer rows.Close()

	return r.scanDataSources(rows)
}

// GetMetricsEndpoints returns all active VictoriaMetrics/Prometheus endpoints.
// Maps to config.yaml's database.victoria_metrics.endpoints.
func (r *DataSourceRepo) GetMetricsEndpoints(ctx context.Context) ([]string, error) {
	sources, err := r.ListByType(ctx, DataSourceTypePrometheus)
	if err != nil {
		return nil, err
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("%w for type prometheus", ErrNoActiveDataSources)
	}

	endpoints := make([]string, 0, len(sources))
	for _, s := range sources {
		if s.URL != "" {
			endpoints = append(endpoints, s.URL)
		}
	}

	return endpoints, nil
}

// GetLogsEndpoints returns all active VictoriaLogs endpoints.
// Maps to config.yaml's database.victoria_logs.endpoints.
func (r *DataSourceRepo) GetLogsEndpoints(ctx context.Context) ([]string, error) {
	sources, err := r.ListByType(ctx, DataSourceTypeVictoriaLogs)
	if err != nil {
		return nil, err
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("%w for type victorialogs", ErrNoActiveDataSources)
	}

	endpoints := make([]string, 0, len(sources))
	for _, s := range sources {
		if s.URL != "" {
			endpoints = append(endpoints, s.URL)
		}
	}

	return endpoints, nil
}

// GetTracesEndpoints returns all active VictoriaTraces endpoints.
// Maps to config.yaml's database.victoria_traces.endpoints.
func (r *DataSourceRepo) GetTracesEndpoints(ctx context.Context) ([]string, error) {
	sources, err := r.ListByType(ctx, DataSourceTypeVictoriaTraces)
	if err != nil {
		return nil, err
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("%w for type victoriatraces", ErrNoActiveDataSources)
	}

	endpoints := make([]string, 0, len(sources))
	for _, s := range sources {
		if s.URL != "" {
			endpoints = append(endpoints, s.URL)
		}
	}

	return endpoints, nil
}

// GetDataSourcesWithCredentials returns data sources with auth credentials.
// Useful for initializing VictoriaMetrics services with proper authentication.
type DataSourceWithCredentials struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"` //nolint:gosec // G117: Intentional credential transport
	APIKey   string `json:"api_key,omitempty"`  //nolint:gosec // G117: Intentional credential transport
}

// GetMetricsSourcesWithCreds returns metrics endpoints with authentication info.
func (r *DataSourceRepo) GetMetricsSourcesWithCreds(ctx context.Context) ([]DataSourceWithCredentials, error) {
	sources, err := r.ListByType(ctx, DataSourceTypePrometheus)
	if err != nil {
		return nil, err
	}

	result := make([]DataSourceWithCredentials, 0, len(sources))
	for _, s := range sources {
		if s.URL != "" {
			result = append(result, DataSourceWithCredentials{
				URL:      s.URL,
				Username: s.Username,
				Password: s.Password,
				APIKey:   s.APIKey,
			})
		}
	}

	return result, nil
}

// scanDataSource scans a single row into a DataSource.
func (r *DataSourceRepo) scanDataSource(row *sql.Row) (*DataSource, error) {
	var ds DataSource
	var projectIdentifier, apiKey, username, password sql.NullString
	var healthURL, healthBodyType, healthBodyMatchMode sql.NullString
	var healthBodyTextPattern, healthBodyJSONKey, healthBodyJSONExpectedVal sql.NullString
	var aiConfig []byte

	err := row.Scan(
		&ds.ID, &ds.Name, &ds.Type, &projectIdentifier, &ds.URL,
		&apiKey, &username, &password,
		&healthURL, &ds.HealthExpectedStatus, &healthBodyType, &healthBodyMatchMode,
		&healthBodyTextPattern, &healthBodyJSONKey, &healthBodyJSONExpectedVal,
		&ds.HealthCheckIntervalMs, &ds.IsActive, &aiConfig,
		&ds.CreatedAt, &ds.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	ds.ProjectIdentifier = projectIdentifier.String
	ds.APIKey = apiKey.String
	ds.Username = username.String
	ds.Password = password.String
	ds.HealthURL = healthURL.String
	ds.HealthBodyType = healthBodyType.String
	ds.HealthBodyMatchMode = healthBodyMatchMode.String
	ds.HealthBodyTextPattern = healthBodyTextPattern.String
	ds.HealthBodyJSONKey = healthBodyJSONKey.String
	ds.HealthBodyJSONExpectedVal = healthBodyJSONExpectedVal.String

	if len(aiConfig) > 0 {
		ds.AIConfig = aiConfig
	}

	return &ds, nil
}

// scanDataSources scans multiple rows into DataSource slice.
func (r *DataSourceRepo) scanDataSources(rows *sql.Rows) ([]*DataSource, error) {
	var result []*DataSource

	for rows.Next() {
		var ds DataSource
		var projectIdentifier, apiKey, username, password sql.NullString
		var healthURL, healthBodyType, healthBodyMatchMode sql.NullString
		var healthBodyTextPattern, healthBodyJSONKey, healthBodyJSONExpectedVal sql.NullString
		var aiConfig []byte

		err := rows.Scan(
			&ds.ID, &ds.Name, &ds.Type, &projectIdentifier, &ds.URL,
			&apiKey, &username, &password,
			&healthURL, &ds.HealthExpectedStatus, &healthBodyType, &healthBodyMatchMode,
			&healthBodyTextPattern, &healthBodyJSONKey, &healthBodyJSONExpectedVal,
			&ds.HealthCheckIntervalMs, &ds.IsActive, &aiConfig,
			&ds.CreatedAt, &ds.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		// Handle nullable fields
		ds.ProjectIdentifier = projectIdentifier.String
		ds.APIKey = apiKey.String
		ds.Username = username.String
		ds.Password = password.String
		ds.HealthURL = healthURL.String
		ds.HealthBodyType = healthBodyType.String
		ds.HealthBodyMatchMode = healthBodyMatchMode.String
		ds.HealthBodyTextPattern = healthBodyTextPattern.String
		ds.HealthBodyJSONKey = healthBodyJSONKey.String
		ds.HealthBodyJSONExpectedVal = healthBodyJSONExpectedVal.String

		if len(aiConfig) > 0 {
			ds.AIConfig = aiConfig
		}

		result = append(result, &ds)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}
