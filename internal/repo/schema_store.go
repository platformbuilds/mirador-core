package repo

import "context"

// SchemaStore defines the storage contract used by handlers and metrics.
// Implemented by the Weaviate-backed repository.
type SchemaStore interface {
	// Metrics
	UpsertMetric(ctx context.Context, m MetricDef, author string) error
	GetMetric(ctx context.Context, tenantID, metric string) (*MetricDef, error)
	ListMetricVersions(ctx context.Context, tenantID, metric string) ([]VersionInfo, error)
	GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, VersionInfo, error)

	// Metric labels
	UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error
	GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*MetricLabelDef, error)

	// Logs
	UpsertLogField(ctx context.Context, f LogFieldDef, author string) error
	GetLogField(ctx context.Context, tenantID, field string) (*LogFieldDef, error)
	ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]VersionInfo, error)
	GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, VersionInfo, error)

	// Traces
	UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, purpose, owner string, tags []string, author string) error
	GetTraceService(ctx context.Context, tenantID, service string) (*TraceServiceDef, error)
	ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]VersionInfo, error)
	GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, VersionInfo, error)

	UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, purpose, owner string, tags []string, author string) error
	GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*TraceOperationDef, error)
	ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]VersionInfo, error)
	GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, VersionInfo, error)
}
