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

// KPIDataType represents the data type of a KPI.
type KPIDataType string

const (
	KPIDataTypeTimeseries  KPIDataType = "timeseries"
	KPIDataTypeValue       KPIDataType = "value"
	KPIDataTypeCategorical KPIDataType = "categorical"
)

// KPI represents a KPI record from MariaDB.
// Matches the schema in tenant_slug.sql kpis table.
type KPI struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	DataType        KPIDataType     `json:"data_type"`
	Definition      string          `json:"definition,omitempty"`
	Formula         string          `json:"formula,omitempty"`
	DataSourceID    string          `json:"data_source_id"`
	KPIDatastoreID  string          `json:"kpi_datastore_id,omitempty"`
	Unit            string          `json:"unit,omitempty"`
	Thresholds      json.RawMessage `json:"thresholds,omitempty"`
	RefreshInterval int             `json:"refresh_interval"`
	IsShared        bool            `json:"is_shared"`
	UserID          string          `json:"user_id"`
	Namespace       string          `json:"namespace"`
	Kind            string          `json:"kind"`
	Layer           string          `json:"layer"`
	Classifier      string          `json:"classifier,omitempty"`
	SignalType      string          `json:"signal_type"`
	Sentiment       string          `json:"sentiment,omitempty"`
	ComponentType   string          `json:"component_type,omitempty"`
	Query           json.RawMessage `json:"query,omitempty"`
	Examples        string          `json:"examples,omitempty"`
	DimensionsHint  json.RawMessage `json:"dimensions_hint,omitempty"`
	QueryType       string          `json:"query_type,omitempty"`
	Datastore       string          `json:"datastore,omitempty"`
	ServiceFamily   string          `json:"service_family,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// KPIListOptions provides filtering and pagination for listing KPIs.
type KPIListOptions struct {
	Limit      int
	Offset     int
	Namespace  string
	Kind       string
	Layer      string
	SignalType string
	UserID     string
}

// Static errors for KPI operations
var (
	ErrKPINotFound   = errors.New("mariadb: kpi not found")
	ErrKPIQueryError = errors.New("mariadb: kpi query failed")
)

// KPIRepo provides read-only access to KPIs in MariaDB.
type KPIRepo struct {
	client *Client
	logger *zap.Logger
}

// NewKPIRepo creates a new KPIRepo.
func NewKPIRepo(client *Client, logger *zap.Logger) *KPIRepo {
	return &KPIRepo{
		client: client,
		logger: logger,
	}
}

// GetByID retrieves a single KPI by ID.
func (r *KPIRepo) GetByID(ctx context.Context, id string) (*KPI, error) {
	if !r.client.IsEnabled() {
		return nil, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return nil, ErrMariaDBNotConnected
	}

	query := `
		SELECT id, name, description, data_type, definition, formula,
		       data_source_id, kpi_datastore_id, unit, thresholds, refresh_interval,
		       is_shared, user_id, namespace, kind, layer, classifier, signal_type,
		       sentiment, component_type, query, examples, dimensions_hint,
		       query_type, datastore, service_family, created_at, updated_at
		FROM kpis
		WHERE id = ?
	`

	row := db.QueryRowContext(ctx, query, id)
	kpi, err := r.scanKPI(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKPINotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrKPIQueryError, err)
	}

	return kpi, nil
}

// List retrieves KPIs with optional filtering and pagination.
func (r *KPIRepo) List(ctx context.Context, opts *KPIListOptions) ([]*KPI, int64, error) {
	if !r.client.IsEnabled() {
		return nil, 0, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return nil, 0, ErrMariaDBNotConnected
	}

	if opts == nil {
		opts = &KPIListOptions{}
	}

	// Build query with optional filters
	baseQuery := `
		SELECT id, name, description, data_type, definition, formula,
		       data_source_id, kpi_datastore_id, unit, thresholds, refresh_interval,
		       is_shared, user_id, namespace, kind, layer, classifier, signal_type,
		       sentiment, component_type, query, examples, dimensions_hint,
		       query_type, datastore, service_family, created_at, updated_at
		FROM kpis
		WHERE 1=1
	`

	countQuery := `SELECT COUNT(*) FROM kpis WHERE 1=1`

	var args []interface{}
	var countArgs []interface{}
	filters := ""

	if opts.Namespace != "" {
		filters += " AND namespace = ?"
		args = append(args, opts.Namespace)
		countArgs = append(countArgs, opts.Namespace)
	}
	if opts.Kind != "" {
		filters += " AND kind = ?"
		args = append(args, opts.Kind)
		countArgs = append(countArgs, opts.Kind)
	}
	if opts.Layer != "" {
		filters += " AND layer = ?"
		args = append(args, opts.Layer)
		countArgs = append(countArgs, opts.Layer)
	}
	if opts.SignalType != "" {
		filters += " AND signal_type = ?"
		args = append(args, opts.SignalType)
		countArgs = append(countArgs, opts.SignalType)
	}
	if opts.UserID != "" {
		filters += " AND user_id = ?"
		args = append(args, opts.UserID)
		countArgs = append(countArgs, opts.UserID)
	}

	// Get total count
	var total int64
	countRow := db.QueryRowContext(ctx, countQuery+filters, countArgs...)
	if err := countRow.Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("%w: count query: %v", ErrKPIQueryError, err)
	}

	// Apply pagination
	query := baseQuery + filters + " ORDER BY name ASC"
	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrKPIQueryError, err)
	}
	defer rows.Close()

	kpis, err := r.scanKPIs(rows)
	if err != nil {
		return nil, 0, err
	}

	return kpis, total, nil
}

// ListAll retrieves all KPIs (for sync purposes).
func (r *KPIRepo) ListAll(ctx context.Context) ([]*KPI, error) {
	if !r.client.IsEnabled() {
		return nil, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return nil, ErrMariaDBNotConnected
	}

	query := `
		SELECT id, name, description, data_type, definition, formula,
		       data_source_id, kpi_datastore_id, unit, thresholds, refresh_interval,
		       is_shared, user_id, namespace, kind, layer, classifier, signal_type,
		       sentiment, component_type, query, examples, dimensions_hint,
		       query_type, datastore, service_family, created_at, updated_at
		FROM kpis
		ORDER BY updated_at DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKPIQueryError, err)
	}
	defer rows.Close()

	return r.scanKPIs(rows)
}

// ListUpdatedSince retrieves KPIs updated after a given timestamp (for incremental sync).
func (r *KPIRepo) ListUpdatedSince(ctx context.Context, since time.Time) ([]*KPI, error) {
	if !r.client.IsEnabled() {
		return nil, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return nil, ErrMariaDBNotConnected
	}

	query := `
		SELECT id, name, description, data_type, definition, formula,
		       data_source_id, kpi_datastore_id, unit, thresholds, refresh_interval,
		       is_shared, user_id, namespace, kind, layer, classifier, signal_type,
		       sentiment, component_type, query, examples, dimensions_hint,
		       query_type, datastore, service_family, created_at, updated_at
		FROM kpis
		WHERE updated_at > ?
		ORDER BY updated_at ASC
	`

	rows, err := db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKPIQueryError, err)
	}
	defer rows.Close()

	return r.scanKPIs(rows)
}

// Count returns the total number of KPIs.
func (r *KPIRepo) Count(ctx context.Context) (int64, error) {
	if !r.client.IsEnabled() {
		return 0, ErrMariaDBDisabled
	}

	db := r.client.DB()
	if db == nil {
		return 0, ErrMariaDBNotConnected
	}

	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM kpis").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrKPIQueryError, err)
	}

	return count, nil
}

// scanKPI scans a single row into a KPI.
func (r *KPIRepo) scanKPI(row *sql.Row) (*KPI, error) {
	var kpi KPI
	var description, definition, formula, kpiDatastoreID, unit sql.NullString
	var classifier, sentiment, componentType, examples sql.NullString
	var queryType, datastore, serviceFamily sql.NullString
	var thresholds, query, dimensionsHint []byte

	err := row.Scan(
		&kpi.ID, &kpi.Name, &description, &kpi.DataType, &definition, &formula,
		&kpi.DataSourceID, &kpiDatastoreID, &unit, &thresholds, &kpi.RefreshInterval,
		&kpi.IsShared, &kpi.UserID, &kpi.Namespace, &kpi.Kind, &kpi.Layer,
		&classifier, &kpi.SignalType, &sentiment, &componentType, &query,
		&examples, &dimensionsHint, &queryType, &datastore, &serviceFamily,
		&kpi.CreatedAt, &kpi.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	kpi.Description = description.String
	kpi.Definition = definition.String
	kpi.Formula = formula.String
	kpi.KPIDatastoreID = kpiDatastoreID.String
	kpi.Unit = unit.String
	kpi.Classifier = classifier.String
	kpi.Sentiment = sentiment.String
	kpi.ComponentType = componentType.String
	kpi.Examples = examples.String
	kpi.QueryType = queryType.String
	kpi.Datastore = datastore.String
	kpi.ServiceFamily = serviceFamily.String

	if len(thresholds) > 0 {
		kpi.Thresholds = thresholds
	}
	if len(query) > 0 {
		kpi.Query = query
	}
	if len(dimensionsHint) > 0 {
		kpi.DimensionsHint = dimensionsHint
	}

	return &kpi, nil
}

// scanKPIs scans multiple rows into KPI slice.
func (r *KPIRepo) scanKPIs(rows *sql.Rows) ([]*KPI, error) {
	var result []*KPI

	for rows.Next() {
		var kpi KPI
		var description, definition, formula, kpiDatastoreID, unit sql.NullString
		var classifier, sentiment, componentType, examples sql.NullString
		var queryType, datastore, serviceFamily sql.NullString
		var thresholds, query, dimensionsHint []byte

		err := rows.Scan(
			&kpi.ID, &kpi.Name, &description, &kpi.DataType, &definition, &formula,
			&kpi.DataSourceID, &kpiDatastoreID, &unit, &thresholds, &kpi.RefreshInterval,
			&kpi.IsShared, &kpi.UserID, &kpi.Namespace, &kpi.Kind, &kpi.Layer,
			&classifier, &kpi.SignalType, &sentiment, &componentType, &query,
			&examples, &dimensionsHint, &queryType, &datastore, &serviceFamily,
			&kpi.CreatedAt, &kpi.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		// Handle nullable fields
		kpi.Description = description.String
		kpi.Definition = definition.String
		kpi.Formula = formula.String
		kpi.KPIDatastoreID = kpiDatastoreID.String
		kpi.Unit = unit.String
		kpi.Classifier = classifier.String
		kpi.Sentiment = sentiment.String
		kpi.ComponentType = componentType.String
		kpi.Examples = examples.String
		kpi.QueryType = queryType.String
		kpi.Datastore = datastore.String
		kpi.ServiceFamily = serviceFamily.String

		if len(thresholds) > 0 {
			kpi.Thresholds = thresholds
		}
		if len(query) > 0 {
			kpi.Query = query
		}
		if len(dimensionsHint) > 0 {
			kpi.DimensionsHint = dimensionsHint
		}

		result = append(result, &kpi)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}
