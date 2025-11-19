package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRCAEngine is a mock implementation of services.RCAEngine for testing
type mockRCAEngine struct {
	shouldFail bool
	score      float64
}

func (m *mockRCAEngine) ComputeRCA(
	ctx context.Context,
	incident *rca.IncidentContext,
	opts rca.RCAOptions,
) (*rca.RCAIncident, error) {
	if m.shouldFail {
		return nil, &errFailure{}
	}

	result := rca.NewRCAIncident(incident)

	// Create a basic chain
	chain := rca.NewRCAChain()
	step := rca.NewRCAStep(1, incident.ImpactService, "test_component")
	step.Ring = rca.RingImmediate
	step.Direction = rca.DirectionSame
	step.TimeRange = rca.TimeRange{
		Start: incident.TimeBounds.TStart,
		End:   incident.TimeBounds.TEnd,
	}
	step.Score = m.score

	chain.AddStep(step)
	chain.Score = m.score
	chain.Rank = 1

	result.AddChain(chain)
	result.SetRootCauseFromBestChain()
	result.Score = m.score

	return result, nil
}

type errFailure struct{}

func (e *errFailure) Error() string   { return "mock RCA engine failure" }
func (e *errFailure) Timeout() bool   { return false }
func (e *errFailure) Temporary() bool { return false }

func TestHandleComputeRCA_ValidRequest(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", nil)

	mockLogger := logger.NewMockLogger(&strings.Builder{})
	mockRCA := &mockRCAEngine{score: 0.75}

	handler := &RCAHandler{
		logger:             mockLogger,
		rcaEngine:          mockRCA,
		logsService:        nil,
		serviceGraph:       nil,
		cache:              nil,
		featureFlagService: nil,
	}

	// Create request body
	now := time.Now()
	req := models.RCARequest{
		ImpactService:   "api-gateway",
		ImpactMetric:    "error_rate",
		MetricDirection: "higher_is_worse",
		TimeStart:       now.Add(-10 * time.Minute).Format(time.RFC3339),
		TimeEnd:         now.Format(time.RFC3339),
		Severity:        0.8,
		ImpactSummary:   "Test incident",
		MaxChains:       5,
	}

	body, _ := json.Marshal(req)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Call handler
	handler.HandleComputeRCA(c)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.RCAResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got %s", resp.Status)
	}

	if resp.Data == nil {
		t.Fatal("Expected non-nil data in response")
	}

	if resp.Data.Impact.ImpactService != "api-gateway" {
		t.Errorf("Expected impact service 'api-gateway', got %s", resp.Data.Impact.ImpactService)
	}

	if resp.Data.Score != 0.75 {
		t.Errorf("Expected score 0.75, got %f", resp.Data.Score)
	}

	if len(resp.Data.Chains) == 0 {
		t.Error("Expected at least one chain in response")
	}

	t.Logf("Response score: %f", resp.Data.Score)
}

func TestHandleComputeRCA_MissingImpactService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(&strings.Builder{})
	mockRCA := &mockRCAEngine{}

	handler := &RCAHandler{
		logger:    mockLogger,
		rcaEngine: mockRCA,
	}

	// Create request with missing impactService
	now := time.Now()
	req := models.RCARequest{
		ImpactService: "", // Missing!
		TimeStart:     now.Add(-10 * time.Minute).Format(time.RFC3339),
		TimeEnd:       now.Format(time.RFC3339),
	}

	body, _ := json.Marshal(req)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "error" {
		t.Errorf("Expected error status, got %s", resp["status"])
	}

	t.Logf("Correctly rejected missing impactService")
}

func TestHandleComputeRCA_InvalidTimeFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(&strings.Builder{})
	mockRCA := &mockRCAEngine{}

	handler := &RCAHandler{
		logger:    mockLogger,
		rcaEngine: mockRCA,
	}

	// Create request with invalid time format
	req := models.RCARequest{
		ImpactService: "api-gateway",
		TimeStart:     "not-a-date",
		TimeEnd:       "also-not-a-date",
	}

	body, _ := json.Marshal(req)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	t.Logf("Correctly rejected invalid time format")
}

func TestHandleComputeRCA_TimeOrderError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(&strings.Builder{})
	mockRCA := &mockRCAEngine{}

	handler := &RCAHandler{
		logger:    mockLogger,
		rcaEngine: mockRCA,
	}

	// Create request with start > end
	now := time.Now()
	req := models.RCARequest{
		ImpactService: "api-gateway",
		TimeStart:     now.Format(time.RFC3339),
		TimeEnd:       now.Add(-10 * time.Minute).Format(time.RFC3339), // Earlier than start!
	}

	body, _ := json.Marshal(req)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	t.Logf("Correctly rejected invalid time order")
}

func TestHandleComputeRCA_RCAEngineFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(&strings.Builder{})
	mockRCA := &mockRCAEngine{shouldFail: true}

	handler := &RCAHandler{
		logger:    mockLogger,
		rcaEngine: mockRCA,
	}

	now := time.Now()
	req := models.RCARequest{
		ImpactService: "api-gateway",
		TimeStart:     now.Add(-10 * time.Minute).Format(time.RFC3339),
		TimeEnd:       now.Format(time.RFC3339),
	}

	body, _ := json.Marshal(req)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	t.Logf("Correctly handled RCA engine failure")
}

func TestConvertRCAIncidentToDTO(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	handler := &RCAHandler{logger: mockLogger}

	// Create a test RCA incident
	now := time.Now()
	incident := &rca.IncidentContext{
		ID:            "test_incident",
		ImpactService: "api-gateway",
		ImpactSignal: rca.ImpactSignal{
			ServiceName: "api-gateway",
			MetricName:  "error_rate",
		},
		TimeBounds: rca.IncidentTimeWindow{
			TStart: now.Add(-10 * time.Minute),
			TPeak:  now.Add(-5 * time.Minute),
			TEnd:   now,
		},
		ImpactSummary: "Test incident",
		Severity:      0.8,
	}

	rcaIncident := rca.NewRCAIncident(incident)

	// Add a chain
	chain := rca.NewRCAChain()
	step := rca.NewRCAStep(1, "cassandra", "database")
	step.Ring = rca.RingImmediate
	step.TimeRange = rca.TimeRange{Start: now, End: now.Add(5 * time.Minute)}
	step.Score = 0.85
	chain.AddStep(step)
	chain.Score = 0.85
	chain.Rank = 1

	rcaIncident.AddChain(chain)
	rcaIncident.SetRootCauseFromBestChain()

	// Convert to DTO
	dto := handler.convertRCAIncidentToDTO(rcaIncident)

	// Verify
	if dto == nil {
		t.Fatal("Expected non-nil DTO")
	}

	if dto.Impact.ImpactService != "api-gateway" {
		t.Errorf("Expected impact service 'api-gateway', got %s", dto.Impact.ImpactService)
	}

	if len(dto.Chains) != 1 {
		t.Errorf("Expected 1 chain, got %d", len(dto.Chains))
	}

	if dto.RootCause == nil {
		t.Error("Expected non-nil root cause")
	} else {
		if dto.RootCause.Service != "cassandra" {
			t.Errorf("Expected root cause service 'cassandra', got %s", dto.RootCause.Service)
		}
	}

	t.Logf("DTO conversion successful")
}
