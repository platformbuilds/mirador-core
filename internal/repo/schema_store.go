package repo

import (
	"context"
)

// SchemaStore defines the storage contract used by handlers and metrics.
// Implemented by the Weaviate-backed repository.
type SchemaStore interface {
	// Metrics
	UpsertMetric(ctx context.Context, m MetricDef, author string) error
	GetMetric(ctx context.Context, metric string) (*MetricDef, error)
	ListMetricVersions(ctx context.Context, metric string) ([]VersionInfo, error)
	GetMetricVersion(ctx context.Context, metric string, version int64) (map[string]any, VersionInfo, error)

	// Metric labels
	UpsertMetricLabel(ctx context.Context, metric, label, typ string, required bool, allowed map[string]any, description string) error
	GetMetricLabelDefs(ctx context.Context, metric string, labels []string) (map[string]*MetricLabelDef, error)

	// Logs
	UpsertLogField(ctx context.Context, f LogFieldDef, author string) error
	GetLogField(ctx context.Context, field string) (*LogFieldDef, error)
	ListLogFieldVersions(ctx context.Context, field string) ([]VersionInfo, error)
	GetLogFieldVersion(ctx context.Context, field string, version int64) (map[string]any, VersionInfo, error)

	// Traces
	UpsertTraceServiceWithAuthor(ctx context.Context, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error
	GetTraceService(ctx context.Context, service string) (*TraceServiceDef, error)
	ListTraceServiceVersions(ctx context.Context, service string) ([]VersionInfo, error)
	GetTraceServiceVersion(ctx context.Context, service string, version int64) (map[string]any, VersionInfo, error)

	UpsertTraceOperationWithAuthor(ctx context.Context, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error
	GetTraceOperation(ctx context.Context, service, operation string) (*TraceOperationDef, error)
	ListTraceOperationVersions(ctx context.Context, service, operation string) ([]VersionInfo, error)
	GetTraceOperationVersion(ctx context.Context, service, operation string, version int64) (map[string]any, VersionInfo, error)

	// Label definitions (independent of metric)
	UpsertLabel(ctx context.Context, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error
	GetLabel(ctx context.Context, name string) (*LabelDef, error)
	ListLabelVersions(ctx context.Context, name string) ([]VersionInfo, error)
	GetLabelVersion(ctx context.Context, name string, version int64) (map[string]any, VersionInfo, error)
	DeleteLabel(ctx context.Context, name string) error

	// Deletes for other schema types
	DeleteMetric(ctx context.Context, metric string) error
	DeleteLogField(ctx context.Context, field string) error
	DeleteTraceService(ctx context.Context, service string) error
	DeleteTraceOperation(ctx context.Context, service, operation string) error
}
