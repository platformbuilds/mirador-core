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
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

// mockKPIRepo is a mock implementation for testing KPI operations
type mockKPIRepo struct {
	kpis       map[string]*models.KPIDefinition
	dashboards map[string]*models.Dashboard
	layouts    map[string]map[string]interface{} // dashboardID -> map[kpiId] -> layout
}

func newMockKPIRepo() *mockKPIRepo {
	return &mockKPIRepo{
		kpis:       make(map[string]*models.KPIDefinition),
		dashboards: make(map[string]*models.Dashboard),
		layouts:    make(map[string]map[string]interface{}),
	}
}

// KPI operations
func (m *mockKPIRepo) UpsertKPI(ctx context.Context, kpi *models.KPIDefinition) error {
	m.kpis[kpi.TenantID+"|"+kpi.ID] = kpi
	return nil
}

func (m *mockKPIRepo) GetKPI(ctx context.Context, tenantID, id string) (*models.KPIDefinition, error) {
	key := tenantID + "|" + id
	if kpi, exists := m.kpis[key]; exists {
		return kpi, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockKPIRepo) ListKPIs(ctx context.Context, tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {
	var kpis []*models.KPIDefinition
	total := 0

	for _, kpi := range m.kpis {
		if kpi.TenantID == tenantID {
			// Simple tag filtering (in real impl would be more sophisticated)
			if len(tags) == 0 || containsAny(kpi.Tags, tags) {
				total++
				if len(kpis) < limit && total > offset {
					kpis = append(kpis, kpi)
				}
			}
		}
	}

	return kpis, total, nil
}

func (m *mockKPIRepo) DeleteKPI(ctx context.Context, tenantID, id string) error {
	key := tenantID + "|" + id
	if _, exists := m.kpis[key]; exists {
		delete(m.kpis, key)
		return nil
	}
	return fmt.Errorf("not found")
}

// Layout operations
func (m *mockKPIRepo) GetKPILayoutsForDashboard(ctx context.Context, tenantID, dashboardID string) (map[string]interface{}, error) {
	if layouts, exists := m.layouts[dashboardID]; exists {
		return layouts, nil
	}
	return make(map[string]interface{}), nil
}

func (m *mockKPIRepo) BatchUpsertKPILayouts(ctx context.Context, tenantID, dashboardID string, layouts map[string]interface{}) error {
	m.layouts[dashboardID] = layouts
	return nil
}

// Dashboard operations
func (m *mockKPIRepo) UpsertDashboard(ctx context.Context, dashboard *models.Dashboard) error {
	m.dashboards[dashboard.TenantID+"|"+dashboard.ID] = dashboard
	return nil
}

func (m *mockKPIRepo) GetDashboard(ctx context.Context, tenantID, id string) (*models.Dashboard, error) {
	key := tenantID + "|" + id
	if dashboard, exists := m.dashboards[key]; exists {
		return dashboard, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockKPIRepo) ListDashboards(ctx context.Context, tenantID string, limit, offset int) ([]*models.Dashboard, int, error) {
	var dashboards []*models.Dashboard
	total := 0

	for _, dashboard := range m.dashboards {
		if dashboard.TenantID == tenantID {
			total++
			if len(dashboards) < limit && total > offset {
				dashboards = append(dashboards, dashboard)
			}
		}
	}

	return dashboards, total, nil
}

func (m *mockKPIRepo) DeleteDashboard(ctx context.Context, tenantID, id string) error {
	key := tenantID + "|" + id
	if _, exists := m.dashboards[key]; exists {
		delete(m.dashboards, key)
		return nil
	}
	return fmt.Errorf("not found")
}

// Implement SchemaStore interface methods (stubs for legacy schema types)
func (m *mockKPIRepo) UpsertMetric(ctx context.Context, metric repo.MetricDef, author string) error {
	return nil
}
func (m *mockKPIRepo) GetMetric(ctx context.Context, tenantID, metric string) (*repo.MetricDef, error) {
	return nil, nil
}
func (m *mockKPIRepo) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockKPIRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockKPIRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}
func (m *mockKPIRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return nil, nil
}
func (m *mockKPIRepo) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	return nil
}
func (m *mockKPIRepo) GetLogField(ctx context.Context, tenantID, field string) (*repo.LogFieldDef, error) {
	return nil, nil
}
func (m *mockKPIRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockKPIRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockKPIRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockKPIRepo) GetTraceService(ctx context.Context, tenantID, service string) (*repo.TraceServiceDef, error) {
	return nil, nil
}
func (m *mockKPIRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockKPIRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockKPIRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockKPIRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*repo.TraceOperationDef, error) {
	return nil, nil
}
func (m *mockKPIRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockKPIRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockKPIRepo) UpsertLabel(ctx context.Context, tenantID, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	return nil
}
func (m *mockKPIRepo) GetLabel(ctx context.Context, tenantID, name string) (*repo.LabelDef, error) {
	return nil, nil
}
func (m *mockKPIRepo) ListLabelVersions(ctx context.Context, tenantID, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockKPIRepo) GetLabelVersion(ctx context.Context, tenantID, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockKPIRepo) DeleteLabel(ctx context.Context, tenantID, name string) error     { return nil }
func (m *mockKPIRepo) DeleteMetric(ctx context.Context, tenantID, metric string) error  { return nil }
func (m *mockKPIRepo) DeleteLogField(ctx context.Context, tenantID, field string) error { return nil }
func (m *mockKPIRepo) DeleteTraceService(ctx context.Context, tenantID, service string) error {
	return nil
}
func (m *mockKPIRepo) DeleteTraceOperation(ctx context.Context, tenantID, service, operation string) error {
	return nil
}
func (m *mockKPIRepo) UpsertSchemaAsKPI(ctx context.Context, schemaDef *models.SchemaDefinition, author string) error {
	return nil
}
func (m *mockKPIRepo) GetSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) (*models.SchemaDefinition, error) {
	return &models.SchemaDefinition{ID: id, Type: models.SchemaType(schemaType), TenantID: tenantID}, nil
}
func (m *mockKPIRepo) ListSchemasAsKPIs(ctx context.Context, tenantID, schemaType string, limit, offset int) ([]*models.SchemaDefinition, int, error) {
	return []*models.SchemaDefinition{}, 0, nil
}
func (m *mockKPIRepo) DeleteSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) error {
	return nil
}

// Helper function
func containsAny(slice []string, items []string) bool {
	for _, item := range items {
		for _, s := range slice {
			if s == item {
				return true
			}
		}
	}
	return false
}

// mockCache implements the ValkeyCluster interface for testing
type mockCache struct{}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) { return nil, nil }
func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}
func (m *mockCache) Delete(ctx context.Context, key string) error { return nil }
func (m *mockCache) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return true, nil
}
func (m *mockCache) ReleaseLock(ctx context.Context, key string) error { return nil }
func (m *mockCache) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
	return nil, nil
}
func (m *mockCache) SetSession(ctx context.Context, session *models.UserSession) error { return nil }
func (m *mockCache) InvalidateSession(ctx context.Context, sessionID string) error     { return nil }
func (m *mockCache) GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error) {
	return nil, nil
}
func (m *mockCache) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
	return nil
}
func (m *mockCache) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
	return nil, nil
}
func (m *mockCache) AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error {
	return nil
}
func (m *mockCache) GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error) {
	return nil, nil
}
func (m *mockCache) DeletePatternIndex(ctx context.Context, patternKey string) error { return nil }
func (m *mockCache) DeleteMultiple(ctx context.Context, keys []string) error         { return nil }
func (m *mockCache) GetMemoryInfo(ctx context.Context) (*cache.CacheMemoryInfo, error) {
	return nil, nil
}
func (m *mockCache) AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL time.Duration) error {
	return nil
}
func (m *mockCache) CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error) {
	return 0, nil
}

// TestKPIHandler_GetKPIDefinitions tests the GetKPIDefinitions endpoint
func TestKPIHandler_GetKPIDefinitions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	// Cast to repo.KPIRepo so NewKPIHandler type assertion succeeds
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})
	require.NotNil(t, handler, "KPIHandler should not be nil")

	// Pre-populate with test KPIs
	kpi1 := &models.KPIDefinition{
		ID:        "kpi-1",
		Name:      "Test KPI 1",
		TenantID:  "test-tenant",
		Tags:      []string{"test", "performance"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	kpi2 := &models.KPIDefinition{
		ID:        "kpi-2",
		Name:      "Test KPI 2",
		TenantID:  "test-tenant",
		Tags:      []string{"test", "quality"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.kpis["test-tenant|kpi-1"] = kpi1
	mockRepo.kpis["test-tenant|kpi-2"] = kpi2

	httpReq, _ := http.NewRequest("GET", "/api/v1/kpi/defs?limit=10&offset=0", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.GetKPIDefinitions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.KPIListResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, 2, len(response.KPIDefinitions))
	assert.Equal(t, 2, response.Total)
	assert.Equal(t, 0, response.NextOffset)
}

// TestKPIHandler_CreateOrUpdateKPIDefinition tests the CreateOrUpdateKPIDefinition endpoint
func TestKPIHandler_CreateOrUpdateKPIDefinition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	req := models.KPIDefinitionRequest{
		KPIDefinition: &models.KPIDefinition{
			ID:       "test-kpi",
			Name:     "Test KPI",
			TenantID: "test-tenant",
			Tags:     []string{"test"},
			Query: map[string]interface{}{
				"query": "SELECT COUNT(*) FROM logs",
			},
			Thresholds: []models.Threshold{
				{Level: "warning", Operator: "gt", Value: 100.0},
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/api/v1/kpi/defs", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.CreateOrUpdateKPIDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "ok", response["status"])
	assert.NotEmpty(t, response["id"])
}

// TestKPIHandler_CreateOrUpdateKPIDefinition_InvalidSentiment tests sentiment validation
func TestKPIHandler_CreateOrUpdateKPIDefinition_InvalidSentiment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	req := models.KPIDefinitionRequest{
		KPIDefinition: &models.KPIDefinition{
			ID:        "test-kpi",
			Name:      "Test KPI",
			TenantID:  "test-tenant",
			Sentiment: "INVALID", // Invalid sentiment value
			Tags:      []string{"test"},
			Query: map[string]interface{}{
				"query": "SELECT COUNT(*) FROM logs",
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/api/v1/kpi/defs", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.CreateOrUpdateKPIDefinition(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "sentiment must be either 'NEGATIVE' or 'POSITIVE'", response["error"])
}

// TestKPIHandler_CreateOrUpdateKPIDefinition_ValidSentiment tests valid sentiment values
func TestKPIHandler_CreateOrUpdateKPIDefinition_ValidSentiment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	// Test NEGATIVE sentiment
	req := models.KPIDefinitionRequest{
		KPIDefinition: &models.KPIDefinition{
			ID:         "test-kpi-negative",
			Name:       "Test KPI Negative",
			TenantID:   "test-tenant",
			Sentiment:  "NEGATIVE",
			Definition: "This KPI measures error rate - higher values are bad",
			Tags:       []string{"test"},
			Query: map[string]interface{}{
				"query": "SELECT COUNT(*) FROM logs WHERE level = 'error'",
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/api/v1/kpi/defs", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.CreateOrUpdateKPIDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test POSITIVE sentiment
	req.KPIDefinition.ID = "test-kpi-positive"
	req.KPIDefinition.Name = "Test KPI Positive"
	req.KPIDefinition.Sentiment = "POSITIVE"
	req.KPIDefinition.Definition = "This KPI measures throughput - higher values are good"

	body, _ = json.Marshal(req)
	httpReq, _ = http.NewRequest("POST", "/api/v1/kpi/defs", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.CreateOrUpdateKPIDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestKPIHandler_DeleteKPIDefinition tests the DeleteKPIDefinition endpoint
func TestKPIHandler_DeleteKPIDefinition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	// Pre-populate with a KPI
	kpi := &models.KPIDefinition{
		ID:        "test-kpi",
		Name:      "Test KPI",
		TenantID:  "test-tenant",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.kpis["test-tenant|test-kpi"] = kpi

	httpReq, _ := http.NewRequest("DELETE", "/api/v1/kpi/defs/test-kpi?confirm=1", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "id", Value: "test-kpi"}}
	c.Set("tenant_id", "test-tenant")

	handler.DeleteKPIDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "deleted", response["status"])
}

// TestKPIHandler_GetKPILayouts tests the GetKPILayouts endpoint
func TestKPIHandler_GetKPILayouts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	// Pre-populate with layouts
	layouts := map[string]interface{}{
		"kpi-1": map[string]interface{}{"x": 0, "y": 0, "w": 4, "h": 2},
		"kpi-2": map[string]interface{}{"x": 4, "y": 0, "w": 4, "h": 2},
	}
	mockRepo.layouts["dashboard-1"] = layouts

	httpReq, _ := http.NewRequest("GET", "/api/v1/kpi/layouts/batch?dashboard=dashboard-1", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.GetKPILayouts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.NotNil(t, response["layouts"])
	layoutsResp := response["layouts"].(map[string]interface{})
	assert.Equal(t, 2, len(layoutsResp))
}

// TestKPIHandler_BatchUpdateKPILayouts tests the BatchUpdateKPILayouts endpoint
func TestKPIHandler_BatchUpdateKPILayouts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	req := struct {
		DashboardID string                 `json:"dashboardId"`
		Layouts     map[string]interface{} `json:"layouts"`
	}{
		DashboardID: "dashboard-1",
		Layouts: map[string]interface{}{
			"kpi-1": map[string]interface{}{"x": 0, "y": 0, "w": 6, "h": 3},
			"kpi-2": map[string]interface{}{"x": 6, "y": 0, "w": 6, "h": 3},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/api/v1/kpi/layouts/batch", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.BatchUpdateKPILayouts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "ok", response["status"])
}

// TestKPIHandler_GetDashboards tests the GetDashboards endpoint
func TestKPIHandler_GetDashboards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	// Pre-populate with dashboards
	dashboard1 := &models.Dashboard{
		ID:         "dashboard-1",
		Name:       "Test Dashboard 1",
		TenantID:   "test-tenant",
		Visibility: "private",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	dashboard2 := &models.Dashboard{
		ID:         "dashboard-2",
		Name:       "Test Dashboard 2",
		TenantID:   "test-tenant",
		Visibility: "public",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	mockRepo.dashboards["test-tenant|dashboard-1"] = dashboard1
	mockRepo.dashboards["test-tenant|dashboard-2"] = dashboard2

	httpReq, _ := http.NewRequest("GET", "/api/v1/dashboards?limit=10&offset=0", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.GetDashboards(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.DashboardListResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, 2, len(response.Dashboards))
	assert.Equal(t, 2, response.Total)
	assert.Equal(t, 0, response.NextOffset)
}

// TestKPIHandler_CreateDashboard tests the CreateDashboard endpoint
func TestKPIHandler_CreateDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	req := models.DashboardRequest{
		Dashboard: &models.Dashboard{
			Name:       "New Test Dashboard",
			TenantID:   "test-tenant",
			Visibility: "private",
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "/api/v1/dashboards", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Set("tenant_id", "test-tenant")

	handler.CreateDashboard(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.DashboardResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	require.NotNil(t, response.Dashboard)
	assert.Equal(t, "New Test Dashboard", response.Dashboard.Name)
	assert.Equal(t, "private", response.Dashboard.Visibility)
	assert.NotEmpty(t, response.Dashboard.ID)
}

// TestKPIHandler_UpdateDashboard tests the UpdateDashboard endpoint
func TestKPIHandler_UpdateDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	// Pre-populate with a dashboard
	dashboard := &models.Dashboard{
		ID:         "test-dashboard",
		Name:       "Original Name",
		TenantID:   "test-tenant",
		Visibility: "private",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	mockRepo.dashboards["test-tenant|test-dashboard"] = dashboard

	req := models.DashboardRequest{
		Dashboard: &models.Dashboard{
			Name:       "Updated Name",
			Visibility: "public",
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("PUT", "/api/v1/dashboards/test-dashboard", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "id", Value: "test-dashboard"}}
	c.Set("tenant_id", "test-tenant")

	handler.UpdateDashboard(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.DashboardResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	require.NotNil(t, response.Dashboard)
	assert.Equal(t, "Updated Name", response.Dashboard.Name)
	assert.Equal(t, "public", response.Dashboard.Visibility)
	assert.Equal(t, "test-dashboard", response.Dashboard.ID)
}

// TestKPIHandler_DeleteDashboard tests the DeleteDashboard endpoint
func TestKPIHandler_DeleteDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := newMockKPIRepo()
	handler := NewKPIHandler(mockRepo, &mockCache{}, &mockLogger{})

	// Pre-populate with a dashboard
	dashboard := &models.Dashboard{
		ID:        "test-dashboard",
		Name:      "Test Dashboard",
		TenantID:  "test-tenant",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.dashboards["test-tenant|test-dashboard"] = dashboard

	httpReq, _ := http.NewRequest("DELETE", "/api/v1/dashboards/test-dashboard?confirm=1", http.NoBody)
	httpReq.Header.Set("tenant_id", "test-tenant")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq
	c.Params = gin.Params{{Key: "id", Value: "test-dashboard"}}
	c.Set("tenant_id", "test-tenant")

	handler.DeleteDashboard(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "deleted", response["status"])
}
