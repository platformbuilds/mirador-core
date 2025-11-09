package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/storage/weaviate"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contract validation test structures matching the API contract

type KPIDefinitionResponse struct {
	Defs []KPIDef `json:"defs"`
}

type KPIDef struct {
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	Unit        *string     `json:"unit,omitempty"`
	Format      *string     `json:"format,omitempty"`
	Query       KpiQuery    `json:"query"`
	Thresholds  []KpiThreshold `json:"thresholds,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Sparkline   *Sparkline  `json:"sparkline,omitempty"`
	OwnerUserID *string     `json:"ownerUserId,omitempty"`
	Visibility  *string     `json:"visibility,omitempty"`
}

type KpiQuery struct {
	Type   string                 `json:"type"`
	Ref    *string                `json:"ref,omitempty"`
	UQL    *UQLQuerySpec          `json:"uql,omitempty"`
	Expr   *string                `json:"expr,omitempty"`
	Inputs map[string]KpiQuery    `json:"inputs,omitempty"`
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
	Dashboards []Dashboard `json:"dashboards"`
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

// Mock repo for testing
type mockKPIRepo struct{}

func (m *mockKPIRepo) UpsertKPI(kpi *models.KPIDefinition) error {
	return nil
}

func (m *mockKPIRepo) GetKPI(tenantID, id string) (*models.KPIDefinition, error) {
	return &models.KPIDefinition{
		ID:     id,
		TenantID: tenantID,
		Kind:   "business",
		Name:   "Test KPI",
		Query: models.KpiQuery{
			Type: "metric",
			Ref:  stringPtr("test.metric"),
		},
		Visibility: stringPtr("org"),
	}, nil
}

func (m *mockKPIRepo) ListKPIs(tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {
	return []*models.KPIDefinition{
		{
			ID:     "test-kpi",
			TenantID: tenantID,
			Kind:   "business",
			Name:   "Test KPI",
			Query: models.KpiQuery{
				Type: "metric",
				Ref:  stringPtr("test.metric"),
			},
			Visibility: stringPtr("org"),
		},
	}, 1, nil
}

func (m *mockKPIRepo) DeleteKPI(tenantID, id string) error {
	return nil
}

func (m *mockKPIRepo) GetKPILayoutsForDashboard(tenantID, dashboardID string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"test-kpi": map[string]interface{}{
			"x": 0,
			"y": 0,
			"w": 4,
			"h": 3,
		},
	}, nil
}

func (m *mockKPIRepo) BatchUpsertKPILayouts(tenantID, dashboardID string, layouts map[string]interface{}) error {
	return nil
}

func (m *mockKPIRepo) UpsertDashboard(dashboard *models.Dashboard) error {
	return nil
}

func (m *mockKPIRepo) GetDashboard(tenantID, id string) (*models.Dashboard, error) {
	return &models.Dashboard{
		ID:          id,
		TenantID:    tenantID,
		Name:        "Test Dashboard",
		OwnerUserID: "test-user",
		Visibility:  "org",
	}, nil
}

func (m *mockKPIRepo) ListDashboards(tenantID string, limit, offset int) ([]*models.Dashboard, int, error) {
	return []*models.Dashboard{
		{
			ID:          "default",
			TenantID:    tenantID,
			Name:        "Default Dashboard",
			OwnerUserID: "system",
			Visibility:  "org",
		},
	}, 1, nil
}

func (m *mockKPIRepo) DeleteDashboard(tenantID, id string) error {
	return nil
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func TestContractValidation_KPIDefs_Get(t *testing.T) {
	// Setup test server with mock repo
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockTransport := weaviate.NewMockTransport()
	mockTransport.EnsureClasses(nil, []map[string]any{}) // Mock schema setup

	weaviateRepo := repo.NewWeaviateRepoFromTransport(mockTransport)
	weaviateRepo.EnsureSchema(nil) // Mock schema

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, weaviateRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Make request
	resp, err := http.Get(ts.URL + "/api/v1/kpi/defs")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response KPIDefinitionResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Validate contract
	assert.NotNil(t, response.Defs)
	for _, def := range response.Defs {
		assert.NotEmpty(t, def.ID)
		assert.NotEmpty(t, def.Kind)
		assert.NotEmpty(t, def.Name)
		assert.NotNil(t, def.Query)
		assert.Equal(t, "metric", def.Query.Type) // Based on our mock
	}
}

func TestContractValidation_Layouts_Get(t *testing.T) {
	// Setup similar to above
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockTransport := weaviate.NewMockTransport()
	weaviateRepo := repo.NewWeaviateRepoFromTransport(mockTransport)

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, weaviateRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Make request
	resp, err := http.Get(ts.URL + "/api/v1/kpi/layouts")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var response LayoutResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Validate contract - layouts can be empty object
	assert.NotNil(t, response.Layouts)
}

func TestContractValidation_Dashboards_Get(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockTransport := weaviate.NewMockTransport()
	weaviateRepo := repo.NewWeaviateRepoFromTransport(mockTransport)

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, weaviateRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/dashboards")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response DashboardResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.NotNil(t, response.Dashboards)
	for _, dash := range response.Dashboards {
		assert.NotEmpty(t, dash.ID)
		assert.NotEmpty(t, dash.Name)
		assert.NotEmpty(t, dash.OwnerUserID)
		assert.NotEmpty(t, dash.Visibility)
		assert.NotEmpty(t, dash.CreatedAt)
		assert.NotEmpty(t, dash.UpdatedAt)
	}
}

func TestContractValidation_UserPreferences_Get(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockTransport := weaviate.NewMockTransport()
	weaviateRepo := repo.NewWeaviateRepoFromTransport(mockTransport)

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, weaviateRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/user/preferences")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response UserPreferencesResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.UserID)
	// Other fields are optional
}

func TestContractValidation_KPIDefs_Post(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}

	mockTransport := weaviate.NewMockTransport()
	weaviateRepo := repo.NewWeaviateRepoFromTransport(mockTransport)

	vms := &services.VictoriaMetricsServices{}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, weaviateRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Test request body matching contract
	requestBody := map[string]interface{}{
		"id":     "test-kpi",
		"kind":   "business",
		"name":   "Test KPI",
		"query": map[string]interface{}{
			"type": "metric",
			"ref":  "test.metric",
		},
		"visibility": "org",
	}

	jsonBody, _ := json.Marshal(requestBody)

	resp, err := http.Post(ts.URL+"/api/v1/kpi/defs", "application/json", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed with mock
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response KPIDef
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "test-kpi", response.ID)
	assert.Equal(t, "business", response.Kind)
	assert.Equal(t, "Test KPI", response.Name)
}