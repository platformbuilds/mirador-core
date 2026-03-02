package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mirastacklabs-ai/mirador-core/internal/config"
	"github.com/mirastacklabs-ai/mirador-core/internal/models"
	"github.com/mirastacklabs-ai/mirador-core/internal/repo"
	"github.com/mirastacklabs-ai/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Phase 6 Integration Tests: Testing API endpoints with new fields

// mockRepoPhase6 implements repo.KPIRepo for Phase 6 integration testing
type mockRepoPhase6 struct {
	mockRepo
	storage map[string]*models.KPIDefinition
}

func (m *mockRepoPhase6) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	if m.storage == nil {
		m.storage = make(map[string]*models.KPIDefinition)
	}
	m.storage[k.ID] = k
	return k, "created", nil
}

func (m *mockRepoPhase6) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	if m.storage == nil {
		m.storage = make(map[string]*models.KPIDefinition)
	}
	m.storage[k.ID] = k
	return k, "updated", nil
}

func (m *mockRepoPhase6) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	if m.storage == nil {
		return nil, nil
	}
	return m.storage[id], nil
}

func (m *mockRepoPhase6) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	if m.storage == nil {
		return []*models.KPIDefinition{}, 0, nil
	}
	result := make([]*models.KPIDefinition, 0, len(m.storage))
	for _, kpi := range m.storage {
		result = append(result, kpi)
	}
	return result, int64(len(result)), nil
}

func (m *mockRepoPhase6) DeleteKPI(ctx context.Context, id string) (repo.DeleteResult, error) {
	if m.storage != nil {
		delete(m.storage, id)
	}
	return repo.DeleteResult{
		Weaviate: repo.DeleteStoreResult{Found: true, Deleted: true},
	}, nil
}

func setupPhase6Handler() (*KPIHandler, *mockRepoPhase6) {
	mr := &mockRepoPhase6{
		mockRepo: mockRepo{upserted: []*models.KPIDefinition{}},
		storage:  make(map[string]*models.KPIDefinition),
	}
	l := logger.NewMockLogger(&strings.Builder{})
	cfg := &config.Config{}
	h := &KPIHandler{repo: mr, cache: nil, logger: l, cfg: cfg}
	return h, mr
}

// P6-T2-S1: Test POST /api/v1/kpi/defs with new fields (create)
func TestPOST_CreateKPI_WithNewFields_Phase6(t *testing.T) {
	h, repo := setupPhase6Handler()

	kpi := &models.KPIDefinition{
		ID:              "test-kpi-create",
		Name:            "test_metric_create",
		Kind:            "tech",
		Layer:           "impact",
		Unit:            "ms",
		Format:          "number",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Definition:      "Test KPI for validating new fields in create operation",
		Description:     "Test KPI with new fields",
		DataType:        "timeseries",
		DataSourceID:    "550e8400-e29b-41d4-a716-446655440001",
		KPIDatastoreID:  "550e8400-e29b-41d4-a716-446655440002",
		RefreshInterval: 60,
		IsShared:        true,
		UserID:          "550e8400-e29b-41d4-a716-446655440003",
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	payload := map[string]interface{}{
		"kpiDefinition": kpi,
	}

	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateOrUpdateKPIDefinition(c)

	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
		"Expected 200 or 201, got %d. Body: %s", w.Code, w.Body.String())

	// Verify KPI was created with new fields
	stored, err := repo.GetKPI(context.Background(), "test-kpi-create")
	require.NoError(t, err)
	require.NotNil(t, stored)

	assert.Equal(t, "Test KPI with new fields", stored.Description)
	assert.Equal(t, "timeseries", stored.DataType)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440001", stored.DataSourceID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440002", stored.KPIDatastoreID)
	assert.Equal(t, 60, stored.RefreshInterval)
	assert.True(t, stored.IsShared)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440003", stored.UserID)

	t.Logf("Successfully created KPI with all new fields")
}

// P6-T2-S2: Test POST /api/v1/kpi/defs with new fields (update)
func TestPOST_UpdateKPI_WithNewFields_Phase6(t *testing.T) {
	h, repo := setupPhase6Handler()

	// Create initial KPI
	initial := &models.KPIDefinition{
		ID:              "test-kpi-update",
		Name:            "test_metric_update",
		Kind:            "tech",
		Layer:           "impact",
		Unit:            "ms",
		Format:          "number",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Definition:      "Initial definition for update test",
		Description:     "Initial description",
		DataType:        "gauge",
		RefreshInterval: 30,
		IsShared:        false,
	}
	_, _, err := repo.CreateKPI(context.Background(), initial)
	require.NoError(t, err)

	// Update with new values
	updated := &models.KPIDefinition{
		ID:              "test-kpi-update",
		Name:            "test_metric_update",
		Kind:            "tech",
		Layer:           "impact",
		Unit:            "ms",
		Format:          "number",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Definition:      "Updated definition for update test",
		Description:     "Updated description",
		DataType:        "timeseries",
		DataSourceID:    "550e8400-e29b-41d4-a716-446655440011",
		KPIDatastoreID:  "550e8400-e29b-41d4-a716-446655440012",
		RefreshInterval: 120,
		IsShared:        true,
		UserID:          "550e8400-e29b-41d4-a716-446655440013",
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	payload := map[string]interface{}{
		"kpiDefinition": updated,
	}

	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateOrUpdateKPIDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code, "Update should return 200. Body: %s", w.Body.String())

	// Verify KPI was updated with new field values
	stored, err := repo.GetKPI(context.Background(), "test-kpi-update")
	require.NoError(t, err)
	require.NotNil(t, stored)

	assert.Equal(t, "Updated description", stored.Description)
	assert.Equal(t, "timeseries", stored.DataType)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440011", stored.DataSourceID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440012", stored.KPIDatastoreID)
	assert.Equal(t, 120, stored.RefreshInterval)
	assert.True(t, stored.IsShared)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440013", stored.UserID)

	t.Logf("Successfully updated KPI with new field values")
}

// P6-T2-S3: Test GET /api/v1/kpi/defs/{id} returns new fields
func TestGET_KPIById_ReturnsNewFields_Phase6(t *testing.T) {
	h, repo := setupPhase6Handler()

	kpi := &models.KPIDefinition{
		ID:              "test-kpi-get",
		Name:            "test_metric_get",
		Kind:            "tech",
		Layer:           "cause",
		Unit:            "count",
		Format:          "number",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Definition:      "Test KPI for GET by ID operation",
		Description:     "KPI for GET test",
		DataType:        "counter",
		DataSourceID:    "550e8400-e29b-41d4-a716-446655440021",
		KPIDatastoreID:  "550e8400-e29b-41d4-a716-446655440022",
		RefreshInterval: 90,
		IsShared:        true,
		UserID:          "550e8400-e29b-41d4-a716-446655440023",
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	_, _, err := repo.CreateKPI(context.Background(), kpi)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/kpi/defs/test-kpi-get", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "test-kpi-get"}}

	h.GetKPIDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code, "GET should return 200. Body: %s", w.Body.String())

	var response models.KPIDefinition
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "KPI for GET test", response.Description)
	assert.Equal(t, "counter", response.DataType)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440021", response.DataSourceID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440022", response.KPIDatastoreID)
	assert.Equal(t, 90, response.RefreshInterval)
	assert.True(t, response.IsShared)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440023", response.UserID)

	t.Logf("Successfully retrieved KPI by ID with all new fields")
}

// P6-T2-S4: Test GET /api/v1/kpi/defs list includes new fields
func TestGET_ListKPIs_IncludesNewFields_Phase6(t *testing.T) {
	h, repo := setupPhase6Handler()

	kpi1 := &models.KPIDefinition{
		ID:              "list-kpi-1",
		Name:            "list_metric_1",
		Kind:            "tech",
		Layer:           "impact",
		Unit:            "ms",
		Format:          "number",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Definition:      "First KPI for list test",
		Description:     "First KPI",
		DataType:        "gauge",
		RefreshInterval: 30,
		IsShared:        false,
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	kpi2 := &models.KPIDefinition{
		ID:              "list-kpi-2",
		Name:            "list_metric_2",
		Kind:            "tech",
		Layer:           "cause",
		Unit:            "count",
		Format:          "number",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Definition:      "Second KPI for list test",
		Description:     "Second KPI",
		DataType:        "counter",
		DataSourceID:    "550e8400-e29b-41d4-a716-446655440031",
		KPIDatastoreID:  "550e8400-e29b-41d4-a716-446655440032",
		RefreshInterval: 60,
		IsShared:        true,
		UserID:          "550e8400-e29b-41d4-a716-446655440033",
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	_, _, err := repo.CreateKPI(context.Background(), kpi1)
	require.NoError(t, err)
	_, _, err = repo.CreateKPI(context.Background(), kpi2)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/kpi/defs", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.GetKPIDefinitions(c)

	assert.Equal(t, http.StatusOK, w.Code, "List should return 200. Body: %s", w.Body.String())

	var response models.KPIListResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(response.KPIDefinitions), 2, "Should have at least 2 KPIs")

	// Verify new fields are present in list response
	found := 0
	for _, kpi := range response.KPIDefinitions {
		if kpi.ID == "list-kpi-1" {
			assert.Equal(t, "First KPI", kpi.Description)
			assert.Equal(t, "gauge", kpi.DataType)
			assert.Equal(t, 30, kpi.RefreshInterval)
			assert.False(t, kpi.IsShared)
			found++
		}
		if kpi.ID == "list-kpi-2" {
			assert.Equal(t, "Second KPI", kpi.Description)
			assert.Equal(t, "counter", kpi.DataType)
			assert.Equal(t, "550e8400-e29b-41d4-a716-446655440031", kpi.DataSourceID)
			assert.Equal(t, "550e8400-e29b-41d4-a716-446655440032", kpi.KPIDatastoreID)
			assert.Equal(t, 60, kpi.RefreshInterval)
			assert.True(t, kpi.IsShared)
			assert.Equal(t, "550e8400-e29b-41d4-a716-446655440033", kpi.UserID)
			found++
		}
	}

	assert.Equal(t, 2, found, "Should find both KPIs with new fields")

	t.Logf("Successfully retrieved %d KPIs with new fields in list", len(response.KPIDefinitions))
}

// P6-T3-S1: Test existing KPIs without new fields can still be retrieved
func TestBackwardCompat_GetKPIWithoutNewFields_Phase6(t *testing.T) {
	h, repo := setupPhase6Handler()

	// Create a KPI without new fields (legacy)
	legacyKPI := &models.KPIDefinition{
		ID:         "legacy-kpi",
		Name:       "legacy_metric",
		Kind:       "tech",
		Layer:      "impact",
		Unit:       "ms",
		Format:     "number",
		SignalType: "metrics",
		Sentiment:  "negative",
		Definition: "Legacy KPI without new fields",
		// No new fields (Description, DataType, etc.)
	}

	_, _, err := repo.CreateKPI(context.Background(), legacyKPI)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/kpi/defs/legacy-kpi", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "legacy-kpi"}}

	h.GetKPIDefinition(c)

	assert.Equal(t, http.StatusOK, w.Code, "Legacy KPI should be retrievable. Body: %s", w.Body.String())

	var response models.KPIDefinition
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "legacy-kpi", response.ID)
	assert.Equal(t, "legacy_metric", response.Name)
	// New fields should have zero/empty values
	assert.Empty(t, response.Description)
	assert.Empty(t, response.DataType)
	assert.Empty(t, response.DataSourceID)
	assert.Empty(t, response.KPIDatastoreID)
	assert.Zero(t, response.RefreshInterval)
	assert.False(t, response.IsShared)
	assert.Empty(t, response.UserID)

	t.Logf("Successfully retrieved legacy KPI without new fields")
}

// P6-T3-S2: Test old API payloads (without new fields) still work
func TestBackwardCompat_CreateKPIWithoutNewFields_Phase6(t *testing.T) {
	h, repo := setupPhase6Handler()

	// Old-style payload without new fields
	oldKPI := &models.KPIDefinition{
		ID:         "old-style-kpi",
		Name:       "old_style_metric",
		Kind:       "tech",
		Layer:      "impact",
		Unit:       "ms",
		Format:     "number",
		SignalType: "metrics",
		Sentiment:  "negative",
		Definition: "Old-style KPI for backward compatibility test",
		// New fields intentionally omitted (will have zero values)
		Dashboard: "123e4567-e89b-52d3-a456-426614174000",
	}

	oldPayload := map[string]interface{}{
		"kpiDefinition": oldKPI,
	}

	payload, err := json.Marshal(oldPayload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateOrUpdateKPIDefinition(c)

	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
		"Old-style payload should be accepted, got %d. Body: %s", w.Code, w.Body.String())

	// Verify KPI was created with zero values for new fields
	stored, err := repo.GetKPI(context.Background(), "old-style-kpi")
	require.NoError(t, err)
	require.NotNil(t, stored)

	assert.Empty(t, stored.Description, "Old-style KPI should have empty Description")
	assert.Empty(t, stored.DataType, "Old-style KPI should have empty DataType")
	assert.Zero(t, stored.RefreshInterval, "Old-style KPI should have zero RefreshInterval")
	assert.False(t, stored.IsShared, "Old-style KPI should have false IsShared")

	t.Logf("Successfully created old-style KPI without new fields")
}

// P6-T3-S3: Test JSON serialization omits empty new fields (backward compatibility)
func TestBackwardCompat_JSONOmitsEmptyNewFields_Phase6(t *testing.T) {
	kpi := &models.KPIDefinition{
		ID:         "json-omit-test",
		Name:       "test_metric",
		Kind:       "tech",
		SignalType: "metric",
		// All new fields left empty (zero values)
	}

	jsonData, err := json.Marshal(kpi)
	require.NoError(t, err)

	jsonStr := string(jsonData)

	// Empty new fields should be omitted due to `omitempty` tags
	assert.NotContains(t, jsonStr, "\"description\"", "Empty description should be omitted")
	assert.NotContains(t, jsonStr, "\"dataType\"", "Empty dataType should be omitted")
	assert.NotContains(t, jsonStr, "\"dataSourceId\"", "Empty dataSourceId should be omitted")
	assert.NotContains(t, jsonStr, "\"kpiDatastoreId\"", "Empty kpiDatastoreId should be omitted")
	assert.NotContains(t, jsonStr, "\"userId\"", "Empty userId should be omitted")

	// Zero values for int and bool may or may not be omitted depending on omitempty behavior
	// but the API should handle both cases correctly
	t.Logf("Serialized JSON (should omit empty new fields): %s", jsonStr)
}

// P6-T3-S4: Test mixed list with both old and new style KPIs
func TestBackwardCompat_MixedKPIList_Phase6(t *testing.T) {
	h, repo := setupPhase6Handler()

	legacyKPI := &models.KPIDefinition{
		ID:         "mixed-legacy",
		Name:       "legacy_kpi",
		Kind:       "tech",
		Layer:      "impact",
		Unit:       "count",
		Format:     "number",
		SignalType: "metrics",
		Sentiment:  "negative",
		Definition: "Legacy KPI in mixed list",
		// No new fields
	}
	newKPI := &models.KPIDefinition{
		ID:              "mixed-new",
		Name:            "new_kpi",
		Kind:            "tech",
		Layer:           "cause",
		Unit:            "count",
		Format:          "number",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Definition:      "New style KPI in mixed list",
		Description:     "Has new fields",
		DataType:        "timeseries",
		RefreshInterval: 60,
		IsShared:        true,
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	_, _, err := repo.CreateKPI(context.Background(), legacyKPI)
	require.NoError(t, err)
	_, _, err = repo.CreateKPI(context.Background(), newKPI)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/kpi/defs", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.GetKPIDefinitions(c)

	assert.Equal(t, http.StatusOK, w.Code, "Mixed KPI list should work")

	var response models.KPIListResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(response.KPIDefinitions), 2, "Should have both KPIs")

	// Verify both KPIs are in the list with correct field values
	foundLegacy := false
	foundNew := false
	for _, kpi := range response.KPIDefinitions {
		if kpi.ID == "mixed-legacy" {
			foundLegacy = true
			assert.Empty(t, kpi.Description, "Legacy KPI should have empty description")
			assert.Zero(t, kpi.RefreshInterval, "Legacy KPI should have zero refresh interval")
		}
		if kpi.ID == "mixed-new" {
			foundNew = true
			assert.Equal(t, "Has new fields", kpi.Description, "New KPI should have description")
			assert.Equal(t, 60, kpi.RefreshInterval, "New KPI should have refresh interval")
			assert.True(t, kpi.IsShared, "New KPI should be shared")
		}
	}

	assert.True(t, foundLegacy, "Should find legacy KPI")
	assert.True(t, foundNew, "Should find new KPI")

	t.Logf("Successfully retrieved %d mixed KPIs", len(response.KPIDefinitions))
}
