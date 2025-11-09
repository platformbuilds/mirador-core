package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contract validation test structures matching the API contract

type KPIDefinitionResponse struct {
	KPIDefinitions []*KPIDef `json:"kpiDefinitions"`
	Total          int       `json:"total"`
	NextOffset     int       `json:"nextOffset,omitempty"`
}

type KPIDef struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	Unit        *string        `json:"unit,omitempty"`
	Format      *string        `json:"format,omitempty"`
	Query       KpiQuery       `json:"query"`
	Thresholds  []KpiThreshold `json:"thresholds,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Sparkline   *Sparkline     `json:"sparkline,omitempty"`
	OwnerUserID *string        `json:"ownerUserId,omitempty"`
	Visibility  *string        `json:"visibility,omitempty"`
}

type KpiQuery struct {
	Type   string              `json:"type"`
	Ref    *string             `json:"ref,omitempty"`
	UQL    *UQLQuerySpec       `json:"uql,omitempty"`
	Expr   *string             `json:"expr,omitempty"`
	Inputs map[string]KpiQuery `json:"inputs,omitempty"`
}

type UQLQuerySpec struct {
	Engine string `json:"engine"`
	Query  string `json:"query"`
}

type KpiThreshold struct {
	When   string      `json:"when"`
	Value  interface{} `json:"value"`
	Status string      `json:"status"`
}

type Sparkline struct {
	WindowMins int `json:"windowMins"`
}

type LayoutResponse struct {
	Layouts map[string]Layout `json:"layouts"`
}

type Layout struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type DashboardResponse struct {
	Dashboards []*Dashboard `json:"dashboards"`
	Total      int          `json:"total"`
	NextOffset int          `json:"nextOffset,omitempty"`
}

type Dashboard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	OwnerUserID string `json:"ownerUserId"`
	Visibility  string `json:"visibility"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type UserPreferencesResponse struct {
	UserID              string                 `json:"userId"`
	Theme               *string                `json:"theme,omitempty"`
	SidebarCollapsed    *bool                  `json:"sidebarCollapsed,omitempty"`
	DefaultDashboard    *string                `json:"defaultDashboard,omitempty"`
	Timezone            *string                `json:"timezone,omitempty"`
	KeyboardHintSeen    *bool                  `json:"keyboardHintSeen,omitempty"`
	MiradorCoreEndpoint *string                `json:"miradorCoreEndpoint,omitempty"`
	Preferences         map[string]interface{} `json:"preferences,omitempty"`
}

// Mock repo for testing - implements both SchemaStore and KPIRepo interfaces
type mockRepo struct {
	kpis       map[string]*models.KPIDefinition
	dashboards map[string]*models.Dashboard
	layouts    map[string]map[string]interface{} // dashboardID -> map[kpiId] -> layout
}

func newMockRepo() *mockRepo {
	repo := &mockRepo{
		kpis:       make(map[string]*models.KPIDefinition),
		dashboards: make(map[string]*models.Dashboard),
		layouts:    make(map[string]map[string]interface{}),
	}

	// Initialize with test data
	testKPI := &models.KPIDefinition{
		TenantID: "default",
		ID:       "test-kpi-1",
		Kind:     "business",
		Name:     "Test KPI",
		Query: map[string]interface{}{
			"type": "metric",
			"ref":  "test.metric",
		},
		Visibility: "org",
	}
	repo.kpis["default|test-kpi-1"] = testKPI

	testDashboard := &models.Dashboard{
		TenantID:    "default",
		ID:          "test-dashboard-1",
		Name:        "Test Dashboard",
		OwnerUserID: "anonymous",
		Visibility:  "org",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.dashboards["default|test-dashboard-1"] = testDashboard

	return repo
}

// Implement SchemaStore interface methods (minimal implementation for testing)
func (m *mockRepo) UpsertMetric(ctx context.Context, metric repo.MetricDef, author string) error {
	return nil
}

func (m *mockRepo) GetMetric(ctx context.Context, tenantID, metric string) (*repo.MetricDef, error) {
	return nil, nil
}

func (m *mockRepo) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}

func (m *mockRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}

func (m *mockRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}

func (m *mockRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return nil, nil
}

func (m *mockRepo) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	return nil
}

func (m *mockRepo) GetLogField(ctx context.Context, tenantID, field string) (*repo.LogFieldDef, error) {
	return nil, nil
}

func (m *mockRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}

func (m *mockRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}

func (m *mockRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}

func (m *mockRepo) GetTraceService(ctx context.Context, tenantID, service string) (*repo.TraceServiceDef, error) {
	return nil, nil
}

func (m *mockRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}

func (m *mockRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}

func (m *mockRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}

func (m *mockRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*repo.TraceOperationDef, error) {
	return nil, nil
}

func (m *mockRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}

func (m *mockRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}

func (m *mockRepo) UpsertLabel(ctx context.Context, tenantID, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	return nil
}

func (m *mockRepo) GetLabel(ctx context.Context, tenantID, name string) (*repo.LabelDef, error) {
	return nil, nil
}

func (m *mockRepo) ListLabelVersions(ctx context.Context, tenantID, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}

func (m *mockRepo) GetLabelVersion(ctx context.Context, tenantID, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}

func (m *mockRepo) DeleteLabel(ctx context.Context, tenantID, name string) error {
	return nil
}

func (m *mockRepo) DeleteMetric(ctx context.Context, tenantID, metric string) error {
	return nil
}

func (m *mockRepo) DeleteLogField(ctx context.Context, tenantID, field string) error {
	return nil
}

func (m *mockRepo) DeleteTraceService(ctx context.Context, tenantID, service string) error {
	return nil
}

func (m *mockRepo) DeleteTraceOperation(ctx context.Context, tenantID, service, operation string) error {
	return nil
}

func (m *mockRepo) DeleteSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) error {
	return nil
}

func (m *mockRepo) UpsertSchemaAsKPI(ctx context.Context, schemaDef *models.SchemaDefinition, author string) error {
	return nil
}

func (m *mockRepo) GetSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) (*models.SchemaDefinition, error) {
	return nil, nil
}

func (m *mockRepo) ListSchemasAsKPIs(ctx context.Context, tenantID, schemaType string, limit, offset int) ([]*models.SchemaDefinition, int, error) {
	return nil, 0, nil
}

// Implement KPIRepo interface methods
func (m *mockRepo) UpsertKPI(kpi *models.KPIDefinition) error {
	m.kpis[kpi.TenantID+"|"+kpi.ID] = kpi
	return nil
}

func (m *mockRepo) GetKPI(tenantID, id string) (*models.KPIDefinition, error) {
	key := tenantID + "|" + id
	if kpi, exists := m.kpis[key]; exists {
		return kpi, nil
	}
	return nil, nil
}

func (m *mockRepo) ListKPIs(tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {
	var kpis []*models.KPIDefinition
	for _, kpi := range m.kpis {
		if kpi.TenantID == tenantID {
			kpis = append(kpis, kpi)
		}
	}
	return kpis, len(kpis), nil
}

func (m *mockRepo) DeleteKPI(tenantID, id string) error {
	key := tenantID + "|" + id
	delete(m.kpis, key)
	return nil
}

func (m *mockRepo) GetKPILayoutsForDashboard(tenantID, dashboardID string) (map[string]interface{}, error) {
	if layouts, exists := m.layouts[dashboardID]; exists {
		return layouts, nil
	}
	return map[string]interface{}{}, nil
}

func (m *mockRepo) BatchUpsertKPILayouts(tenantID, dashboardID string, layouts map[string]interface{}) error {
	m.layouts[dashboardID] = layouts
	return nil
}

func (m *mockRepo) UpsertDashboard(dashboard *models.Dashboard) error {
	m.dashboards[dashboard.TenantID+"|"+dashboard.ID] = dashboard
	return nil
}

func (m *mockRepo) GetDashboard(tenantID, id string) (*models.Dashboard, error) {
	key := tenantID + "|" + id
	if dashboard, exists := m.dashboards[key]; exists {
		return dashboard, nil
	}
	return nil, nil
}

func (m *mockRepo) ListDashboards(tenantID string, limit, offset int) ([]*models.Dashboard, int, error) {
	var dashboards []*models.Dashboard
	for _, dashboard := range m.dashboards {
		if dashboard.TenantID == tenantID {
			dashboards = append(dashboards, dashboard)
		}
	}
	return dashboards, len(dashboards), nil
}

func (m *mockRepo) DeleteDashboard(tenantID, id string) error {
	key := tenantID + "|" + id
	delete(m.dashboards, key)
	return nil
}

func TestContractValidation_KPIDefs_Get(t *testing.T) {
	// Setup test server with mock repo
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockRepo := newMockRepo()

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, mockRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Make request
	resp, err := http.Get(ts.URL + "/api/v1/kpi/defs")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response models.KPIListResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Validate contract
	assert.NotNil(t, response.KPIDefinitions)
	for _, def := range response.KPIDefinitions {
		assert.NotEmpty(t, def.ID)
		assert.NotEmpty(t, def.Kind)
		assert.NotEmpty(t, def.Name)
		assert.NotNil(t, def.Query)
		assert.IsType(t, map[string]interface{}{}, def.Query)
	}
}

func TestContractValidation_Layouts_Get(t *testing.T) {
	// Setup similar to above
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockRepo := newMockRepo()

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, mockRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Make request
	resp, err := http.Get(ts.URL + "/api/v1/kpi/layouts?dashboard=default")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Validate contract - layouts can be empty object
	assert.NotNil(t, response["layouts"])
}

func TestContractValidation_Dashboards_Get(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockRepo := newMockRepo()

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, mockRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/kpi/dashboards")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response models.DashboardListResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.NotNil(t, response.Dashboards)
	for _, dash := range response.Dashboards {
		assert.NotEmpty(t, dash.ID)
		assert.NotEmpty(t, dash.Name)
		assert.NotEmpty(t, dash.OwnerUserID)
		assert.NotEmpty(t, dash.Visibility)
		assert.NotZero(t, dash.CreatedAt)
		assert.NotZero(t, dash.UpdatedAt)
	}
}

func TestContractValidation_UserPreferences_Get(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockRepo := newMockRepo()

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, mockRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/config/user-preferences")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response models.UserPreferencesResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.NotNil(t, response.UserPreferences)
	assert.NotEmpty(t, response.UserPreferences.ID)
}

func TestContractValidation_KPIDefs_Post(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockRepo := newMockRepo()

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, mockRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Test request body matching contract
	requestBody := map[string]interface{}{
		"kpiDefinition": map[string]interface{}{
			"id":   "test-kpi",
			"kind": "business",
			"name": "Test KPI",
			"query": map[string]interface{}{
				"type": "metric",
				"ref":  "test.metric",
			},
			"visibility": "org",
		},
	}

	jsonBody, _ := json.Marshal(requestBody)

	resp, err := http.Post(ts.URL+"/api/v1/kpi/defs", "application/json", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed with mock
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.NotEmpty(t, response["id"])
}
