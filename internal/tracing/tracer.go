package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerProvider manages the lifecycle of the OpenTelemetry tracer
type TracerProvider struct {
	tp *sdktrace.TracerProvider
}

// QueryTracer provides distributed tracing for unified queries
type QueryTracer struct {
	tracer trace.Tracer
}

// NewTracerProvider creates a new OpenTelemetry tracer provider
func NewTracerProvider(serviceName, serviceVersion, otlpEndpoint string) (*TracerProvider, error) {
	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(), // TODO: Add TLS configuration
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
			semconv.ServiceNamespaceKey.String("mirador-core"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // TODO: Configure sampling
	)

	otel.SetTracerProvider(tp)

	return &TracerProvider{tp: tp}, nil
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	return tp.tp.Shutdown(ctx)
}

// NewQueryTracer creates a new query tracer
func NewQueryTracer(serviceName string) *QueryTracer {
	tracer := otel.Tracer(serviceName)
	return &QueryTracer{tracer: tracer}
}

// StartQuerySpan starts a new span for a unified query
func (qt *QueryTracer) StartQuerySpan(ctx context.Context, queryID, queryType, query string) (context.Context, trace.Span) {
	ctx, span := qt.tracer.Start(ctx, "unified_query",
		trace.WithAttributes(
			attribute.String("query.id", queryID),
			attribute.String("query.type", queryType),
			attribute.String("query.text", query),
			attribute.String("component", "unified-query-engine"),
		),
	)
	return ctx, span
}

// StartEngineQuerySpan starts a span for an individual engine query within a unified query
func (qt *QueryTracer) StartEngineQuerySpan(ctx context.Context, engineType, query string) (context.Context, trace.Span) {
	ctx, span := qt.tracer.Start(ctx, "engine_query",
		trace.WithAttributes(
			attribute.String("engine.type", engineType),
			attribute.String("engine.query", query),
			attribute.String("component", "query-engine"),
		),
	)
	return ctx, span
}

// StartCorrelationSpan starts a span for correlation operations
func (qt *QueryTracer) StartCorrelationSpan(ctx context.Context, correlationID, correlationType string, enginesCount int) (context.Context, trace.Span) {
	ctx, span := qt.tracer.Start(ctx, "correlation_query",
		trace.WithAttributes(
			attribute.String("correlation.id", correlationID),
			attribute.String("correlation.type", correlationType),
			attribute.Int("correlation.engines_count", enginesCount),
			attribute.String("component", "correlation-engine"),
		),
	)
	return ctx, span
}

// StartCacheOperationSpan starts a span for cache operations
func (qt *QueryTracer) StartCacheOperationSpan(ctx context.Context, operation, key string) (context.Context, trace.Span) {
	ctx, span := qt.tracer.Start(ctx, "cache_operation",
		trace.WithAttributes(
			attribute.String("cache.operation", operation),
			attribute.String("cache.key", key),
			attribute.String("component", "cache"),
		),
	)
	return ctx, span
}

// StartUQLProcessingSpan starts a span for UQL processing
func (qt *QueryTracer) StartUQLProcessingSpan(ctx context.Context, stage string) (context.Context, trace.Span) {
	ctx, span := qt.tracer.Start(ctx, "uql_processing",
		trace.WithAttributes(
			attribute.String("uql.stage", stage),
			attribute.String("component", "uql-processor"),
		),
	)
	return ctx, span
}

// AddQueryAttributes adds common query attributes to a span
func (qt *QueryTracer) AddQueryAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

// RecordQueryMetrics records query performance metrics on a span
func (qt *QueryTracer) RecordQueryMetrics(span trace.Span, duration time.Duration, recordCount int64, success bool) {
	span.SetAttributes(
		attribute.Int64("query.duration_ms", duration.Milliseconds()),
		attribute.Int64("query.record_count", recordCount),
		attribute.Bool("query.success", success),
	)

	if !success {
		span.SetStatus(codes.Error, "query failed")
	}
}

// RecordEngineMetrics records engine-specific metrics on a span
func (qt *QueryTracer) RecordEngineMetrics(span trace.Span, engineType string, duration time.Duration, recordCount int64, success bool) {
	span.SetAttributes(
		attribute.String("engine.type", engineType),
		attribute.Int64("engine.duration_ms", duration.Milliseconds()),
		attribute.Int64("engine.record_count", recordCount),
		attribute.Bool("engine.success", success),
	)

	if !success {
		span.SetStatus(codes.Error, "engine query failed")
	}
}

// RecordCacheMetrics records cache operation metrics on a span
func (qt *QueryTracer) RecordCacheMetrics(span trace.Span, hit bool, duration time.Duration) {
	span.SetAttributes(
		attribute.Bool("cache.hit", hit),
		attribute.Int64("cache.duration_ms", duration.Milliseconds()),
	)
}

// RecordError records an error on a span
func (qt *QueryTracer) RecordError(span trace.Span, err error, attrs ...attribute.KeyValue) {
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attrs...)
	span.RecordError(err)
}

// Global tracer instance
var globalQueryTracer *QueryTracer

// InitGlobalTracer initializes the global query tracer
func InitGlobalTracer(serviceName string) {
	globalQueryTracer = NewQueryTracer(serviceName)
}

// GetGlobalTracer returns the global query tracer
func GetGlobalTracer() *QueryTracer {
	return globalQueryTracer
}
