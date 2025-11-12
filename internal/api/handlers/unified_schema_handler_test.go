package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
)

// mockLogger implements the Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Info(msg string, fields ...interface{})  {}
func (m *mockLogger) Error(msg string, fields ...interface{}) {}
func (m *mockLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockLogger) Debug(msg string, fields ...interface{}) {}
func (m *mockLogger) Fatal(msg string, fields ...interface{}) {}

// mockUnifiedSchemaRepo is a mock implementation for testing
type mockUnifiedSchemaRepo struct {
	labels          map[string]*repo.LabelDef
	metrics         map[string]*repo.MetricDef
	logFields       map[string]*repo.LogFieldDef
	traceServices   map[string]*repo.TraceServiceDef
	traceOperations map[string]map[string]*repo.TraceOperationDef // service -> operation -> def
}

func newMockUnifiedSchemaRepo() *mockUnifiedSchemaRepo {
	return &mockUnifiedSchemaRepo{
		labels:          make(map[string]*repo.LabelDef),
		metrics:         make(map[string]*repo.MetricDef),
		logFields:       make(map[string]*repo.LogFieldDef),
		traceServices:   make(map[string]*repo.TraceServiceDef),
		traceOperations: make(map[string]map[string]*repo.TraceOperationDef),
	}
}

// Implement SchemaStore interface methods
func (m *mockUnifiedSchemaRepo) UpsertLabel(ctx context.Context, tenantID, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	m.labels[tenantID+"|"+name] = &repo.LabelDef{
		TenantID:    tenantID,
		Name:        name,
		Type:        typ,
		Required:    required,
		AllowedVals: allowed,
		Description: description,
		Category:    category,
		Sentiment:   sentiment,
		UpdatedAt:   time.Now(),
	}
	return nil
}

func (m *mockUnifiedSchemaRepo) GetLabel(ctx context.Context, tenantID, name string) (*repo.LabelDef, error) {
	key := tenantID + "|" + name
	if label, exists := m.labels[key]; exists {
		return label, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockUnifiedSchemaRepo) UpsertMetric(ctx context.Context, metric repo.MetricDef, author string) error {
	m.metrics[metric.TenantID+"|"+metric.Metric] = &metric
	return nil
}

func (m *mockUnifiedSchemaRepo) GetMetric(ctx context.Context, tenantID, metric string) (*repo.MetricDef, error) {
	key := tenantID + "|" + metric
	if m, exists := m.metrics[key]; exists {
		return m, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockUnifiedSchemaRepo) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	m.logFields[f.TenantID+"|"+f.Field] = &f
	return nil
}

func (m *mockUnifiedSchemaRepo) GetLogField(ctx context.Context, tenantID, field string) (*repo.LogFieldDef, error) {
	key := tenantID + "|" + field
	if f, exists := m.logFields[key]; exists {
		return f, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockUnifiedSchemaRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	m.traceServices[tenantID+"|"+service] = &repo.TraceServiceDef{
		TenantID:       tenantID,
		Service:        service,
		ServicePurpose: servicePurpose,
		Owner:          owner,
		Tags:           tags,
		Category:       category,
		Sentiment:      sentiment,
		UpdatedAt:      time.Now(),
	}
	return nil
}

func (m *mockUnifiedSchemaRepo) GetTraceService(ctx context.Context, tenantID, service string) (*repo.TraceServiceDef, error) {
	key := tenantID + "|" + service
	if s, exists := m.traceServices[key]; exists {
		return s, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockUnifiedSchemaRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	if m.traceOperations[service] == nil {
		m.traceOperations[service] = make(map[string]*repo.TraceOperationDef)
	}
	m.traceOperations[service][operation] = &repo.TraceOperationDef{
		TenantID:       tenantID,
		Service:        service,
		Operation:      operation,
		ServicePurpose: servicePurpose,
		Owner:          owner,
		Tags:           tags,
		Category:       category,
		Sentiment:      sentiment,
		UpdatedAt:      time.Now(),
	}
	return nil
}

func (m *mockUnifiedSchemaRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*repo.TraceOperationDef, error) {
	if serviceOps, exists := m.traceOperations[service]; exists {
		if op, exists := serviceOps[operation]; exists {
			return op, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// Stub implementations for other required methods
func (m *mockUnifiedSchemaRepo) ListLabelVersions(ctx context.Context, tenantID, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockUnifiedSchemaRepo) GetLabelVersion(ctx context.Context, tenantID, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockUnifiedSchemaRepo) DeleteLabel(ctx context.Context, tenantID, name string) error {
	return nil
}
func (m *mockUnifiedSchemaRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}
func (m *mockUnifiedSchemaRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return nil, nil
}
func (m *mockUnifiedSchemaRepo) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockUnifiedSchemaRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockUnifiedSchemaRepo) DeleteMetric(ctx context.Context, tenantID, metric string) error {
	return nil
}
func (m *mockUnifiedSchemaRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockUnifiedSchemaRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockUnifiedSchemaRepo) DeleteLogField(ctx context.Context, tenantID, field string) error {
	return nil
}
func (m *mockUnifiedSchemaRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockUnifiedSchemaRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockUnifiedSchemaRepo) DeleteTraceService(ctx context.Context, tenantID, service string) error {
	return nil
}
func (m *mockUnifiedSchemaRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockUnifiedSchemaRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockUnifiedSchemaRepo) DeleteTraceOperation(ctx context.Context, tenantID, service, operation string) error {
	return nil
}
func (m *mockUnifiedSchemaRepo) UpsertSchemaAsKPI(ctx context.Context, schemaDef *models.SchemaDefinition, author string) error {
	return nil
}
func (m *mockUnifiedSchemaRepo) GetSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) (*models.SchemaDefinition, error) {
	switch schemaType {
	case "label":
		if label, exists := m.labels[tenantID+"|"+id]; exists {
			return &models.SchemaDefinition{
				ID:        id,
				Name:      label.Name,
				Type:      models.SchemaTypeLabel,
				TenantID:  tenantID,
				Category:  label.Category,
				Sentiment: label.Sentiment,
				Extensions: models.SchemaExtensions{
					Label: &models.LabelExtension{
						Type:        label.Type,
						Required:    label.Required,
						Description: label.Description,
					},
				},
				UpdatedAt: label.UpdatedAt,
			}, nil
		}
	case "metric":
		if metric, exists := m.metrics[tenantID+"|"+id]; exists {
			return &models.SchemaDefinition{
				ID:        id,
				Name:      metric.Metric,
				Type:      models.SchemaTypeMetric,
				TenantID:  tenantID,
				Category:  metric.Category,
				Sentiment: metric.Sentiment,
				Extensions: models.SchemaExtensions{
					Metric: &models.MetricExtension{
						Description: metric.Description,
						Owner:       metric.Owner,
					},
				},
				UpdatedAt: metric.UpdatedAt,
			}, nil
		}
	case "log_field":
		if logField, exists := m.logFields[tenantID+"|"+id]; exists {
			return &models.SchemaDefinition{
				ID:        id,
				Name:      logField.Field,
				Type:      models.SchemaTypeLogField,
				TenantID:  tenantID,
				Category:  logField.Category,
				Sentiment: logField.Sentiment,
				Extensions: models.SchemaExtensions{
					LogField: &models.LogFieldExtension{
						FieldType:   logField.Type,
						Description: logField.Description,
					},
				},
				UpdatedAt: logField.UpdatedAt,
			}, nil
		}
	case "trace_service":
		if traceService, exists := m.traceServices[tenantID+"|"+id]; exists {
			return &models.SchemaDefinition{
				ID:        id,
				Name:      traceService.Service,
				Type:      models.SchemaTypeTraceService,
				TenantID:  tenantID,
				Category:  traceService.Category,
				Sentiment: traceService.Sentiment,
				Extensions: models.SchemaExtensions{
					Trace: &models.TraceExtension{
						ServicePurpose: traceService.ServicePurpose,
						Owner:          traceService.Owner,
					},
				},
				UpdatedAt: traceService.UpdatedAt,
			}, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockUnifiedSchemaRepo) ListSchemasAsKPIs(ctx context.Context, tenantID, schemaType string, limit, offset int) ([]*models.SchemaDefinition, int, error) {
	return []*models.SchemaDefinition{}, 0, nil
}
func (m *mockUnifiedSchemaRepo) DeleteSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) error {
	return nil
}

func TestUnifiedSchemaHandler_UpsertSchemaDefinition_Label(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	req := models.SchemaDefinitionRequest{
		SchemaDefinition: &models.SchemaDefinition{
			ID:        "test-label",
			Name:      "test-label",
			Type:      models.SchemaTypeLabel,
			TenantID:  "test-tenant",
			Category:  "infrastructure",
			Sentiment: "NEUTRAL",
			Author:    "test@example.com",
			Extensions: models.SchemaExtensions{
				Label: &models.LabelExtension{
					Type:        "string",
					Required:    false,
					Description: "Test label",
				},
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/schema/label", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "type", Value: "label"}}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.UpsertSchemaDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "ok", response["status"])
	assert.NotEmpty(t, response["id"])
}

func TestUnifiedSchemaHandler_GetSchemaDefinition_Label(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()

	// Pre-populate with a label
	label := &repo.LabelDef{
		TenantID:    "test-tenant",
		Name:        "test-label",
		Type:        "string",
		Required:    false,
		Description: "Test label",
		Category:    "infrastructure",
		Sentiment:   "NEUTRAL",
		UpdatedAt:   time.Now(),
	}
	mockRepo.labels["test-tenant|test-label"] = label

	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	httpReq, _ := http.NewRequest("GET", "/schema/label/test-label", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{
		{Key: "type", Value: "label"},
		{Key: "id", Value: "test-label"},
	}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.GetSchemaDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.SchemaDefinitionResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	require.NotNil(t, response.SchemaDefinition)
	assert.Equal(t, models.SchemaTypeLabel, response.SchemaDefinition.Type)
	assert.Equal(t, "test-label", response.SchemaDefinition.Name)
	assert.Equal(t, "infrastructure", response.SchemaDefinition.Category)
	assert.Equal(t, "NEUTRAL", response.SchemaDefinition.Sentiment)
	assert.NotNil(t, response.SchemaDefinition.Extensions.Label)
	assert.Equal(t, "string", response.SchemaDefinition.Extensions.Label.Type)
	assert.Equal(t, false, response.SchemaDefinition.Extensions.Label.Required)
	assert.Equal(t, "Test label", response.SchemaDefinition.Extensions.Label.Description)
}

func TestUnifiedSchemaHandler_UpsertSchemaDefinition_Metric(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	req := models.SchemaDefinitionRequest{
		SchemaDefinition: &models.SchemaDefinition{
			ID:        "test-metric",
			Name:      "test-metric",
			Type:      models.SchemaTypeMetric,
			TenantID:  "test-tenant",
			Tags:      []string{"test", "metric"},
			Category:  "business",
			Sentiment: "POSITIVE",
			Author:    "test@example.com",
			Extensions: models.SchemaExtensions{
				Metric: &models.MetricExtension{
					Description: "Test metric",
					Owner:       "team@example.com",
				},
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/schema/metric", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "type", Value: "metric"}}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.UpsertSchemaDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "ok", response["status"])
}

func TestUnifiedSchemaHandler_UpsertSchemaDefinition_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	req := models.SchemaDefinitionRequest{
		SchemaDefinition: &models.SchemaDefinition{
			Name:     "test",
			Type:     models.SchemaType("invalid"),
			TenantID: "test-tenant",
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/schema/invalid", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "type", Value: "invalid"}}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.UpsertSchemaDefinition(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "unsupported schema type")
}

func TestUnifiedSchemaHandler_GetSchemaDefinition_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	httpReq, _ := http.NewRequest("GET", "/schema/label/nonexistent", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{
		{Key: "type", Value: "label"},
		{Key: "id", Value: "nonexistent"},
	}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.GetSchemaDefinition(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "not found")
}

func TestUnifiedSchemaHandler_UpsertSchemaDefinition_LogField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	req := models.SchemaDefinitionRequest{
		SchemaDefinition: &models.SchemaDefinition{
			ID:        "test-log-field",
			Name:      "test-log-field",
			Type:      models.SchemaTypeLogField,
			TenantID:  "test-tenant",
			Tags:      []string{"test", "log"},
			Category:  "application",
			Sentiment: "INFO",
			Author:    "test@example.com",
			Extensions: models.SchemaExtensions{
				LogField: &models.LogFieldExtension{
					FieldType:   "string",
					Description: "Test log field",
				},
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/schema/log_field", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "type", Value: "log_field"}}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.UpsertSchemaDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "ok", response["status"])
}

func TestUnifiedSchemaHandler_UpsertSchemaDefinition_TraceService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	req := models.SchemaDefinitionRequest{
		SchemaDefinition: &models.SchemaDefinition{
			ID:        "test-service",
			Name:      "test-service",
			Type:      models.SchemaTypeTraceService,
			TenantID:  "test-tenant",
			Tags:      []string{"test", "service"},
			Category:  "microservice",
			Sentiment: "POSITIVE",
			Author:    "test@example.com",
			Extensions: models.SchemaExtensions{
				Trace: &models.TraceExtension{
					ServicePurpose: "Test service",
					Owner:          "team@example.com",
				},
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/schema/trace_service", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "type", Value: "trace_service"}}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.UpsertSchemaDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "ok", response["status"])
}

func TestUnifiedSchemaHandler_ListSchemaDefinitions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	httpReq, _ := http.NewRequest("GET", "/schema/label?limit=10&offset=0", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "type", Value: "label"}}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.ListSchemaDefinitions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.SchemaListResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	// Should return empty list since we don't have listing implemented in mock
	assert.NotNil(t, response.SchemaDefinitions)
	assert.Equal(t, 0, len(response.SchemaDefinitions))
	assert.Equal(t, 0, response.Total)
	assert.Equal(t, 0, response.NextOffset)
}

func TestUnifiedSchemaHandler_DeleteSchemaDefinition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockUnifiedSchemaRepo()
	handler := NewUnifiedSchemaHandler(mockRepo, nil, nil, nil, &mockLogger{}, 1024*1024)

	httpReq, _ := http.NewRequest("DELETE", "/schema/label/test-label?confirm=1", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{
		{Key: "type", Value: "label"},
		{Key: "id", Value: "test-label"},
	}
	c.Set("tenant_id", "test-tenant") // Set tenant_id in context

	handler.DeleteSchemaDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "deleted", response["status"])
}
