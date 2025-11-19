package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
)

type MetricDef struct {
	Metric      string    `json:"metric"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	Tags        []string  `json:"tags"`
	Category    string    `json:"category"`
	Sentiment   string    `json:"sentiment"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type LogFieldDef struct {
	Field       string    `json:"field"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Category    string    `json:"category"`
	Sentiment   string    `json:"sentiment"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SchemaRepo struct{ DB *sql.DB }

func NewSchemaRepo(db *sql.DB) *SchemaRepo { return &SchemaRepo{DB: db} }

func (r *SchemaRepo) UpsertMetric(ctx context.Context, m MetricDef, author string) error {
	tagsJSON, _ := json.Marshal(m.Tags)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO metric_def (metric, description, owner, tags, category, sentiment, current_version) VALUES (?,?,?,?,?,?,?,current_version)
         ON DUPLICATE KEY UPDATE description=VALUES(description), owner=VALUES(owner), tags=VALUES(tags), category=VALUES(category), sentiment=VALUES(sentiment), updated_at=CURRENT_TIMESTAMP`,
		m.Metric, m.Description, m.Owner, string(tagsJSON), m.Category, m.Sentiment); err != nil {
		return err
	}
	// bump version counter
	if _, err := tx.ExecContext(ctx, `UPDATE metric_def SET current_version = current_version + 1 WHERE metric=?`, m.Metric); err != nil {
		return err
	}
	// insert version row from current
	var ver int64
	if err := tx.QueryRowContext(ctx, `SELECT current_version FROM metric_def WHERE metric=?`, m.Metric).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(m)
	if _, err := tx.ExecContext(ctx, `INSERT INTO metric_def_versions(metric,version,payload,author) VALUES (?,?,?,?)`, m.Metric, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SchemaRepo) GetMetric(ctx context.Context, metric string) (*MetricDef, error) {

	row := r.DB.QueryRowContext(ctx, `SELECT description, owner, tags, category, sentiment, updated_at FROM metric_def WHERE metric=?`, metric)
	var desc, owner, category, sentiment string
	var tagsRaw sql.NullString
	var updated time.Time
	if err := row.Scan(&desc, &owner, &tagsRaw, &category, &sentiment, &updated); err != nil {
		return nil, err
	}
	var tags []string
	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &MetricDef{Metric: metric, Description: desc, Owner: owner, Tags: tags, Category: category, Sentiment: sentiment, UpdatedAt: updated}, nil
}

func (r *SchemaRepo) UpsertLogField(ctx context.Context, f LogFieldDef, author string) error {
	tagsJSON, _ := json.Marshal(f.Tags)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO log_field_def (field, type, description, tags, category, sentiment) VALUES (?,?,?,?,?,?)
         ON DUPLICATE KEY UPDATE type=VALUES(type), description=VALUES(description), tags=VALUES(tags), category=VALUES(category), sentiment=VALUES(sentiment), updated_at=CURRENT_TIMESTAMP`,
		f.Field, f.Type, f.Description, string(tagsJSON), f.Category, f.Sentiment); err != nil {
		return err
	}
	// bump & version
	var ver int64
	// implement version table by selecting MAX(version)+1
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(MAX(version),0)+1 FROM log_field_def_versions WHERE field=?`, f.Field).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(f)
	if _, err := tx.ExecContext(ctx, `INSERT INTO log_field_def_versions(field,version,payload,author) VALUES (?,?,?,?)`, f.Field, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SchemaRepo) GetLogField(ctx context.Context, field string) (*LogFieldDef, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT type, description, tags, category, sentiment, updated_at FROM log_field_def WHERE field=?`, field)
	var typ, desc, category, sentiment string
	var tagsRaw sql.NullString
	var updated time.Time
	if err := row.Scan(&typ, &desc, &tagsRaw, &category, &sentiment, &updated); err != nil {
		return nil, err
	}
	var tags []string

	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &LogFieldDef{Field: field, Type: typ, Description: desc, Tags: tags, Category: category, Sentiment: sentiment, UpdatedAt: updated}, nil
}

// UpsertLabel inserts or updates a label definition.
func (r *SchemaRepo) UpsertLabel(ctx context.Context, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	allowedJSON, _ := json.Marshal(allowed)
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO label_def (name, type, required, allowed_values, description, category, sentiment)
         VALUES (?,?,?,?,?,?,?)
         ON DUPLICATE KEY UPDATE type=VALUES(type), required=VALUES(required), allowed_values=VALUES(allowed_values), description=VALUES(description), category=VALUES(category), sentiment=VALUES(sentiment), updated_at=CURRENT_TIMESTAMP`,
		name, typ, required, string(allowedJSON), description, category, sentiment)
	return err
}

// GetLabel retrieves a label definition.
func (r *SchemaRepo) GetLabel(ctx context.Context, name string) (*LabelDef, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT type, required, allowed_values, description, category, sentiment, updated_at FROM label_def WHERE name=?`, name)
	var typ, desc, category, sentiment string
	var req bool
	var allowed sql.NullString
	var updated time.Time
	if err := row.Scan(&typ, &req, &allowed, &desc, &category, &sentiment, &updated); err != nil {
		return nil, err
	}
	var allowedMap map[string]any
	if allowed.Valid {
		_ = json.Unmarshal([]byte(allowed.String), &allowedMap)
	}
	return &LabelDef{Name: name, Type: typ, Required: req, AllowedVals: allowedMap, Description: desc, Category: category, Sentiment: sentiment, UpdatedAt: updated}, nil
}

// Versioned upserts with author for traces service/operation
func (r *SchemaRepo) UpsertTraceServiceWithAuthor(ctx context.Context, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	tagsJSON, _ := json.Marshal(tags)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO traces_service_def (service, service_purpose, owner, tags, category, sentiment)
         VALUES (?,?,?,?,?,?)
         ON DUPLICATE KEY UPDATE service_purpose=VALUES(service_purpose), owner=VALUES(owner), tags=VALUES(tags), category=VALUES(category), sentiment=VALUES(sentiment), updated_at=CURRENT_TIMESTAMP`,
		service, servicePurpose, owner, string(tagsJSON), category, sentiment); err != nil {
		return err
	}
	var ver int64
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(MAX(version),0)+1 FROM traces_service_def_versions WHERE service=?`, service).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"service":        service,
		"servicePurpose": servicePurpose,
		"owner":          owner,
		"tags":           tags,
		"category":       category,
		"sentiment":      sentiment,
		"updatedAt":      time.Now(),
	})
	if _, err := tx.ExecContext(ctx, `INSERT INTO traces_service_def_versions(service,version,payload,author) VALUES (?,?,?,?)`, service, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SchemaRepo) UpsertTraceOperationWithAuthor(ctx context.Context, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	tagsJSON, _ := json.Marshal(tags)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO traces_operation_def (service, operation, service_purpose, owner, tags, category, sentiment)
         VALUES (?,?,?,?,?,?,?)
         ON DUPLICATE KEY UPDATE service_purpose=VALUES(service_purpose), owner=VALUES(owner), tags=VALUES(tags), category=VALUES(category), sentiment=VALUES(sentiment), updated_at=CURRENT_TIMESTAMP`,
		service, operation, servicePurpose, owner, string(tagsJSON), category, sentiment); err != nil {
		return err
	}
	var ver int64
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(MAX(version),0)+1 FROM traces_operation_def_versions WHERE service=? AND operation=?`, service, operation).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"service":        service,
		"operation":      operation,
		"servicePurpose": servicePurpose,
		"owner":          owner,
		"tags":           tags,
		"category":       category,
		"sentiment":      sentiment,
		"updatedAt":      time.Now(),
	})
	if _, err := tx.ExecContext(ctx, `INSERT INTO traces_operation_def_versions(service,operation,version,payload,author) VALUES (?,?,?,?,?)`, service, operation, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

// Trace schema models and getters
type TraceServiceDef struct {
	Service        string    `json:"service"`
	ServicePurpose string    `json:"servicePurpose"`
	Owner          string    `json:"owner"`
	Tags           []string  `json:"tags"`
	Category       string    `json:"category"`
	Sentiment      string    `json:"sentiment"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type TraceOperationDef struct {
	Service        string    `json:"service"`
	Operation      string    `json:"operation"`
	ServicePurpose string    `json:"servicePurpose"`
	Owner          string    `json:"owner"`
	Tags           []string  `json:"tags"`
	Category       string    `json:"category"`
	Sentiment      string    `json:"sentiment"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Independent Label definition
type LabelDef struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Required    bool           `json:"required"`
	AllowedVals map[string]any `json:"allowedValues"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Sentiment   string         `json:"sentiment"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

func (r *SchemaRepo) GetTraceService(ctx context.Context, service string) (*TraceServiceDef, error) {

	row := r.DB.QueryRowContext(ctx, `SELECT service_purpose, owner, tags, category, sentiment, updated_at FROM traces_service_def WHERE service=?`, service)
	var servicePurpose, owner, category, sentiment string
	var tagsRaw sql.NullString
	var updated time.Time
	if err := row.Scan(&servicePurpose, &owner, &tagsRaw, &category, &sentiment, &updated); err != nil {
		return nil, err
	}
	var tags []string
	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &TraceServiceDef{Service: service, ServicePurpose: servicePurpose, Owner: owner, Tags: tags, Category: category, Sentiment: sentiment, UpdatedAt: updated}, nil
}

func (r *SchemaRepo) GetTraceOperation(ctx context.Context, service, operation string) (*TraceOperationDef, error) {

	row := r.DB.QueryRowContext(ctx, `SELECT service_purpose, owner, tags, category, sentiment, updated_at FROM traces_operation_def WHERE service=? AND operation=?`, service, operation)
	var servicePurpose, owner, category, sentiment string
	var tagsRaw sql.NullString
	var updated time.Time
	if err := row.Scan(&servicePurpose, &owner, &tagsRaw, &category, &sentiment, &updated); err != nil {
		return nil, err
	}
	var tags []string
	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &TraceOperationDef{Service: service, Operation: operation, ServicePurpose: servicePurpose, Owner: owner, Tags: tags, Category: category, Sentiment: sentiment, UpdatedAt: updated}, nil
}

func (r *SchemaRepo) ListTraceServiceVersions(ctx context.Context, service string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM traces_service_def_versions WHERE service=? ORDER BY version DESC`, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionInfo{}
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.Version, &v.Author, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *SchemaRepo) GetTraceServiceVersion(ctx context.Context, service string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM traces_service_def_versions WHERE service=? AND version=?`, service, version)
	var payloadStr sql.NullString
	var vi VersionInfo
	if err := row.Scan(&payloadStr, &vi.Author, &vi.CreatedAt); err != nil {
		return nil, VersionInfo{}, err
	}
	vi.Version = version
	var payload map[string]any
	if payloadStr.Valid {
		_ = json.Unmarshal([]byte(payloadStr.String), &payload)
	}
	return payload, vi, nil
}

func (r *SchemaRepo) ListTraceOperationVersions(ctx context.Context, service, operation string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM traces_operation_def_versions WHERE service=? AND operation=? ORDER BY version DESC`, service, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionInfo{}
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.Version, &v.Author, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *SchemaRepo) GetTraceOperationVersion(ctx context.Context, service, operation string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM traces_operation_def_versions WHERE service=? AND operation=? AND version=?`, service, operation, version)
	var payloadStr sql.NullString
	var vi VersionInfo
	if err := row.Scan(&payloadStr, &vi.Author, &vi.CreatedAt); err != nil {
		return nil, VersionInfo{}, err
	}
	vi.Version = version
	var payload map[string]any
	if payloadStr.Valid {
		_ = json.Unmarshal([]byte(payloadStr.String), &payload)
	}
	return payload, vi, nil
}

type MetricLabelDef struct {
	Metric      string         `json:"metric"`
	Label       string         `json:"label"`
	Type        string         `json:"type"`
	Required    bool           `json:"required"`
	AllowedVals map[string]any `json:"allowedValues"`
	Description string         `json:"description"`
}

// GetMetricLabelDefs returns label definitions for the given metric and subset of label names.
func (r *SchemaRepo) GetMetricLabelDefs(ctx context.Context, metric string, labels []string) (map[string]*MetricLabelDef, error) {
	if len(labels) == 0 {
		return map[string]*MetricLabelDef{}, nil
	}
	// build IN clause safely

	args := []interface{}{metric}
	placeholders := make([]string, 0, len(labels))
	for _, l := range labels {
		placeholders = append(placeholders, "?")
		args = append(args, l)
	}

	query := "SELECT label, type, required, allowed_values, description FROM metric_label_def WHERE metric=? AND label IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*MetricLabelDef{}
	for rows.Next() {
		var label, typ, desc string
		var req bool
		var allowed sql.NullString
		if err := rows.Scan(&label, &typ, &req, &allowed, &desc); err != nil {
			return nil, err
		}
		var allowedMap map[string]any
		if allowed.Valid {
			_ = json.Unmarshal([]byte(allowed.String), &allowedMap)
		}
		out[label] = &MetricLabelDef{Metric: metric, Label: label, Type: typ, Required: req, AllowedVals: allowedMap, Description: desc}
	}
	return out, nil
}

type VersionInfo struct {
	Version   int64     `json:"version"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *SchemaRepo) ListMetricVersions(ctx context.Context, metric string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM metric_def_versions WHERE metric=? ORDER BY version DESC`, metric)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionInfo{}
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.Version, &v.Author, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *SchemaRepo) ListLogFieldVersions(ctx context.Context, field string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM log_field_def_versions WHERE field=? ORDER BY version DESC`, field)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionInfo{}
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.Version, &v.Author, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *SchemaRepo) GetMetricVersion(ctx context.Context, metric string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM metric_def_versions WHERE metric=? AND version=?`, metric, version)
	var payloadStr sql.NullString
	var vi VersionInfo
	if err := row.Scan(&payloadStr, &vi.Author, &vi.CreatedAt); err != nil {
		return nil, VersionInfo{}, err
	}
	vi.Version = version
	var payload map[string]any
	if payloadStr.Valid {
		_ = json.Unmarshal([]byte(payloadStr.String), &payload)
	}
	return payload, vi, nil
}

func (r *SchemaRepo) GetLogFieldVersion(ctx context.Context, field string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM log_field_def_versions WHERE field=? AND version=?`, field, version)
	var payloadStr sql.NullString
	var vi VersionInfo
	if err := row.Scan(&payloadStr, &vi.Author, &vi.CreatedAt); err != nil {
		return nil, VersionInfo{}, err
	}
	vi.Version = version
	var payload map[string]any
	if payloadStr.Valid {
		_ = json.Unmarshal([]byte(payloadStr.String), &payload)
	}
	return payload, vi, nil
}

// ------------------- KPI Operations -------------------

func (r *SchemaRepo) UpsertKPI(kpi *models.KPIDefinition) error {
	kpiJSON, _ := json.Marshal(kpi)
	_, err := r.DB.ExecContext(context.Background(),
		`INSERT INTO kpi_definitions (id, name, definition, query, kind, sentiment, tags, created_at, updated_at) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) 
		 ON DUPLICATE KEY UPDATE name=VALUES(name), definition=VALUES(definition), query=VALUES(query), kind=VALUES(kind), sentiment=VALUES(sentiment), tags=VALUES(tags), updated_at=VALUES(updated_at)`,
		kpi.ID, kpi.Name, kpi.Definition, string(kpiJSON), kpi.Kind, kpi.Sentiment, string(kpiJSON), kpi.CreatedAt, kpi.UpdatedAt)
	return err
}

func (r *SchemaRepo) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {

	row := r.DB.QueryRowContext(context.Background(), `SELECT name, definition, query, kind, sentiment, tags, created_at, updated_at FROM kpi_definitions WHERE id=?`, id)
	var name, definition, kind, sentiment string
	var queryRaw, tagsRaw sql.NullString
	var createdAt, updatedAt time.Time
	if err := row.Scan(&name, &definition, &queryRaw, &kind, &sentiment, &tagsRaw, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var query map[string]interface{}
	if queryRaw.Valid {
		_ = json.Unmarshal([]byte(queryRaw.String), &query)
	}
	var tags []string
	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &models.KPIDefinition{
		ID:         id,
		Name:       name,
		Definition: definition,
		Query:      query,
		Kind:       kind,
		Sentiment:  sentiment,
		Tags:       tags,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}

func (r *SchemaRepo) ListKPIs(tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {

	query := `SELECT id, name, definition, query, kind, sentiment, tags, created_at, updated_at FROM kpi_definitions`
	args := []interface{}{}

	if len(tags) > 0 {
		placeholders := make([]string, len(tags))
		for i, tag := range tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		query += " AND JSON_CONTAINS(tags, JSON_ARRAY(" + strings.Join(placeholders, ",") + "))"
	}

	query += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.DB.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var kpis []*models.KPIDefinition
	for rows.Next() {
		var id, name, definition, kind, sentiment string
		var queryRaw, tagsRaw sql.NullString
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &definition, &queryRaw, &kind, &sentiment, &tagsRaw, &createdAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		var query map[string]interface{}
		if queryRaw.Valid {
			_ = json.Unmarshal([]byte(queryRaw.String), &query)
		}
		var kpiTags []string
		if tagsRaw.Valid {
			_ = json.Unmarshal([]byte(tagsRaw.String), &kpiTags)
		}
		kpis = append(kpis, &models.KPIDefinition{
			ID:         id,
			Name:       name,
			Definition: definition,
			Query:      query,
			Kind:       kind,
			Sentiment:  sentiment,
			Tags:       kpiTags,
			CreatedAt:  createdAt,
			UpdatedAt:  updatedAt,
		})
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM kpi_definitions`
	countArgs := []interface{}{}
	if len(tags) > 0 {
		placeholders := make([]string, len(tags))
		for i, tag := range tags {
			placeholders[i] = "?"
			countArgs = append(countArgs, tag)
		}
		countQuery += " AND JSON_CONTAINS(tags, JSON_ARRAY(" + strings.Join(placeholders, ",") + "))"
	}
	var total int
	if err := r.DB.QueryRowContext(context.Background(), countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return kpis, total, nil
}

func (r *SchemaRepo) DeleteKPI(ctx context.Context, id string) error {

	_, err := r.DB.ExecContext(ctx, `DELETE FROM kpi_definitions WHERE id=?`, id)
	return err
}
