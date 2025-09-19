package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type MetricDef struct {
	TenantID    string    `json:"tenantId"`
	Metric      string    `json:"metric"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	Tags        []string  `json:"tags"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type LogFieldDef struct {
    TenantID    string         `json:"tenantId"`
    Field       string         `json:"field"`
    Type        string         `json:"type"`
    Description string         `json:"description"`
    Tags        []string       `json:"tags"`
    Examples    []string       `json:"examples"`
    UpdatedAt   time.Time      `json:"updatedAt"`
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
		`INSERT INTO metric_def (tenant_id, metric, description, owner, tags, current_version) VALUES (?,?,?,?,?,current_version)
         ON DUPLICATE KEY UPDATE description=VALUES(description), owner=VALUES(owner), tags=VALUES(tags), updated_at=CURRENT_TIMESTAMP`,
		m.TenantID, m.Metric, m.Description, m.Owner, string(tagsJSON)); err != nil {
		return err
	}
	// bump version counter
	if _, err := tx.ExecContext(ctx, `UPDATE metric_def SET current_version = current_version + 1 WHERE tenant_id=? AND metric=?`, m.TenantID, m.Metric); err != nil {
		return err
	}
	// insert version row from current
	var ver int64
	if err := tx.QueryRowContext(ctx, `SELECT current_version FROM metric_def WHERE tenant_id=? AND metric=?`, m.TenantID, m.Metric).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(m)
	if _, err := tx.ExecContext(ctx, `INSERT INTO metric_def_versions(tenant_id,metric,version,payload,author) VALUES (?,?,?,?,?)`, m.TenantID, m.Metric, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SchemaRepo) GetMetric(ctx context.Context, tenantID, metric string) (*MetricDef, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT description, owner, tags, updated_at FROM metric_def WHERE tenant_id=? AND metric=?`, tenantID, metric)
	var desc, owner string
	var tagsRaw sql.NullString
	var updated time.Time
	if err := row.Scan(&desc, &owner, &tagsRaw, &updated); err != nil {
		return nil, err
	}
	var tags []string
	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &MetricDef{TenantID: tenantID, Metric: metric, Description: desc, Owner: owner, Tags: tags, UpdatedAt: updated}, nil
}

func (r *SchemaRepo) UpsertLogField(ctx context.Context, f LogFieldDef, author string) error {
    tagsJSON, _ := json.Marshal(f.Tags)
    exJSON, _ := json.Marshal(f.Examples)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO log_field_def (tenant_id, field, type, description, tags, examples) VALUES (?,?,?,?,?,?)
         ON DUPLICATE KEY UPDATE type=VALUES(type), description=VALUES(description), tags=VALUES(tags), examples=VALUES(examples), updated_at=CURRENT_TIMESTAMP`,
		f.TenantID, f.Field, f.Type, f.Description, string(tagsJSON), string(exJSON)); err != nil {
		return err
	}
	// bump & version
	var ver int64
	// implement version table by selecting MAX(version)+1
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(MAX(version),0)+1 FROM log_field_def_versions WHERE tenant_id=? AND field=?`, f.TenantID, f.Field).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(f)
	if _, err := tx.ExecContext(ctx, `INSERT INTO log_field_def_versions(tenant_id,field,version,payload,author) VALUES (?,?,?,?,?)`, f.TenantID, f.Field, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SchemaRepo) GetLogField(ctx context.Context, tenantID, field string) (*LogFieldDef, error) {
    row := r.DB.QueryRowContext(ctx, `SELECT type, description, tags, examples, updated_at FROM log_field_def WHERE tenant_id=? AND field=?`, tenantID, field)
    var typ, desc string
    var tagsRaw, exRaw sql.NullString
    var updated time.Time
    if err := row.Scan(&typ, &desc, &tagsRaw, &exRaw, &updated); err != nil {
        return nil, err
    }
    var tags []string
    var ex []string

	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
    if exRaw.Valid {
        _ = json.Unmarshal([]byte(exRaw.String), &ex)
    }
    return &LogFieldDef{TenantID: tenantID, Field: field, Type: typ, Description: desc, Tags: tags, Examples: ex, UpdatedAt: updated}, nil
}

// UpsertMetricLabel inserts or updates a metric label definition.
func (r *SchemaRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	allowedJSON, _ := json.Marshal(allowed)
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO metric_label_def (tenant_id, metric, label, type, required, allowed_values, description)
         VALUES (?,?,?,?,?,?,?)
         ON DUPLICATE KEY UPDATE type=VALUES(type), required=VALUES(required), allowed_values=VALUES(allowed_values), description=VALUES(description)`,
		tenantID, metric, label, typ, required, string(allowedJSON), description)
	return err
}

// Versioned upserts with author for traces service/operation
func (r *SchemaRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, purpose, owner string, tags []string, author string) error {
	tagsJSON, _ := json.Marshal(tags)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO traces_service_def (tenant_id, service, purpose, owner, tags)
         VALUES (?,?,?,?,?)
         ON DUPLICATE KEY UPDATE purpose=VALUES(purpose), owner=VALUES(owner), tags=VALUES(tags), updated_at=CURRENT_TIMESTAMP`,
		tenantID, service, purpose, owner, string(tagsJSON)); err != nil {
		return err
	}
	var ver int64
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(MAX(version),0)+1 FROM traces_service_def_versions WHERE tenant_id=? AND service=?`, tenantID, service).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"tenantId":  tenantID,
		"service":   service,
		"purpose":   purpose,
		"owner":     owner,
		"tags":      tags,
		"updatedAt": time.Now(),
	})
	if _, err := tx.ExecContext(ctx, `INSERT INTO traces_service_def_versions(tenant_id,service,version,payload,author) VALUES (?,?,?,?,?)`, tenantID, service, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SchemaRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, purpose, owner string, tags []string, author string) error {
	tagsJSON, _ := json.Marshal(tags)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO traces_operation_def (tenant_id, service, operation, purpose, owner, tags)
         VALUES (?,?,?,?,?,?)
         ON DUPLICATE KEY UPDATE purpose=VALUES(purpose), owner=VALUES(owner), tags=VALUES(tags), updated_at=CURRENT_TIMESTAMP`,
		tenantID, service, operation, purpose, owner, string(tagsJSON)); err != nil {
		return err
	}
	var ver int64
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(MAX(version),0)+1 FROM traces_operation_def_versions WHERE tenant_id=? AND service=? AND operation=?`, tenantID, service, operation).Scan(&ver); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"tenantId":  tenantID,
		"service":   service,
		"operation": operation,
		"purpose":   purpose,
		"owner":     owner,
		"tags":      tags,
		"updatedAt": time.Now(),
	})
	if _, err := tx.ExecContext(ctx, `INSERT INTO traces_operation_def_versions(tenant_id,service,operation,version,payload,author) VALUES (?,?,?,?,?,?)`, tenantID, service, operation, ver, string(payload), author); err != nil {
		return err
	}
	return tx.Commit()
}

// Trace schema models and getters
type TraceServiceDef struct {
	TenantID  string    `json:"tenantId"`
	Service   string    `json:"service"`
	Purpose   string    `json:"purpose"`
	Owner     string    `json:"owner"`
	Tags      []string  `json:"tags"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TraceOperationDef struct {
	TenantID  string    `json:"tenantId"`
	Service   string    `json:"service"`
	Operation string    `json:"operation"`
	Purpose   string    `json:"purpose"`
	Owner     string    `json:"owner"`
	Tags      []string  `json:"tags"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Independent Label definition
type LabelDef struct {
    TenantID    string            `json:"tenantId"`
    Name        string            `json:"name"`
    Type        string            `json:"type"`
    Required    bool              `json:"required"`
    AllowedVals map[string]any    `json:"allowedValues"`
    Description string            `json:"description"`
    UpdatedAt   time.Time         `json:"updatedAt"`
}

func (r *SchemaRepo) GetTraceService(ctx context.Context, tenantID, service string) (*TraceServiceDef, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT purpose, owner, tags, updated_at FROM traces_service_def WHERE tenant_id=? AND service=?`, tenantID, service)
	var purpose, owner string
	var tagsRaw sql.NullString
	var updated time.Time
	if err := row.Scan(&purpose, &owner, &tagsRaw, &updated); err != nil {
		return nil, err
	}
	var tags []string
	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &TraceServiceDef{TenantID: tenantID, Service: service, Purpose: purpose, Owner: owner, Tags: tags, UpdatedAt: updated}, nil
}

func (r *SchemaRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*TraceOperationDef, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT purpose, owner, tags, updated_at FROM traces_operation_def WHERE tenant_id=? AND service=? AND operation=?`, tenantID, service, operation)
	var purpose, owner string
	var tagsRaw sql.NullString
	var updated time.Time
	if err := row.Scan(&purpose, &owner, &tagsRaw, &updated); err != nil {
		return nil, err
	}
	var tags []string
	if tagsRaw.Valid {
		_ = json.Unmarshal([]byte(tagsRaw.String), &tags)
	}
	return &TraceOperationDef{TenantID: tenantID, Service: service, Operation: operation, Purpose: purpose, Owner: owner, Tags: tags, UpdatedAt: updated}, nil
}

func (r *SchemaRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM traces_service_def_versions WHERE tenant_id=? AND service=? ORDER BY version DESC`, tenantID, service)
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

func (r *SchemaRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM traces_service_def_versions WHERE tenant_id=? AND service=? AND version=?`, tenantID, service, version)
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

func (r *SchemaRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM traces_operation_def_versions WHERE tenant_id=? AND service=? AND operation=? ORDER BY version DESC`, tenantID, service, operation)
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

func (r *SchemaRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM traces_operation_def_versions WHERE tenant_id=? AND service=? AND operation=? AND version=?`, tenantID, service, operation, version)
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
	TenantID    string         `json:"tenantId"`
	Metric      string         `json:"metric"`
	Label       string         `json:"label"`
	Type        string         `json:"type"`
	Required    bool           `json:"required"`
	AllowedVals map[string]any `json:"allowedValues"`
	Description string         `json:"description"`
}

// GetMetricLabelDefs returns label definitions for the given metric and subset of label names.
func (r *SchemaRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*MetricLabelDef, error) {
	if len(labels) == 0 {
		return map[string]*MetricLabelDef{}, nil
	}
	// build IN clause safely
	args := []interface{}{tenantID, metric}
	placeholders := make([]string, 0, len(labels))
	for _, l := range labels {
		placeholders = append(placeholders, "?")
		args = append(args, l)
	}
	query := "SELECT label, type, required, allowed_values, description FROM metric_label_def WHERE tenant_id=? AND metric=? AND label IN (" + strings.Join(placeholders, ",") + ")"
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
		out[label] = &MetricLabelDef{TenantID: tenantID, Metric: metric, Label: label, Type: typ, Required: req, AllowedVals: allowedMap, Description: desc}
	}
	return out, nil
}

type VersionInfo struct {
	Version   int64     `json:"version"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *SchemaRepo) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM metric_def_versions WHERE tenant_id=? AND metric=? ORDER BY version DESC`, tenantID, metric)
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

func (r *SchemaRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]VersionInfo, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT version, author, created_at FROM log_field_def_versions WHERE tenant_id=? AND field=? ORDER BY version DESC`, tenantID, field)
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

func (r *SchemaRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM metric_def_versions WHERE tenant_id=? AND metric=? AND version=?`, tenantID, metric, version)
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

func (r *SchemaRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, VersionInfo, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT payload, author, created_at FROM log_field_def_versions WHERE tenant_id=? AND field=? AND version=?`, tenantID, field, version)
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
