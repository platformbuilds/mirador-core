package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockUnifiedEngine implements services.UnifiedQueryEngine minimally for tests
type mockUnifiedEngine struct{}

func (m *mockUnifiedEngine) ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return &models.UnifiedResult{QueryID: query.ID, Type: query.Type, Status: "success"}, nil
}
func (m *mockUnifiedEngine) ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	// Return a result that reflects the provided time window
	return &models.UnifiedResult{QueryID: query.ID, Type: query.Type, Status: "success", ExecutionTime: 1}, nil
}
func (m *mockUnifiedEngine) ExecuteUQLQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return m.ExecuteQuery(ctx, query)
}
func (m *mockUnifiedEngine) GetQueryMetadata(ctx context.Context) (*models.QueryMetadata, error) {
	return &models.QueryMetadata{}, nil
}
func (m *mockUnifiedEngine) HealthCheck(ctx context.Context) (*models.EngineHealthStatus, error) {
	return &models.EngineHealthStatus{OverallHealth: "healthy"}, nil
}
func (m *mockUnifiedEngine) InvalidateCache(ctx context.Context, queryPattern string) error {
	return nil
}
func (m *mockUnifiedEngine) DetectComponentFailures(ctx context.Context, timeRange models.TimeRange, components []models.FailureComponent, services []string) (*models.FailureCorrelationResult, error) {
	return &models.FailureCorrelationResult{}, nil
}
func (m *mockUnifiedEngine) CorrelateTransactionFailures(ctx context.Context, transactionIDs []string, timeRange models.TimeRange) (*models.FailureCorrelationResult, error) {
	return &models.FailureCorrelationResult{}, nil
}

func TestHandleUnifiedCorrelation_TimeWindowOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(nil)
	handler := &UnifiedQueryHandler{
		unifiedEngine: &mockUnifiedEngine{},
		logger:        mockLogger,
		kpiRepo:       nil,
	}

	now := time.Now().UTC()
	tw := models.TimeWindowRequest{StartTime: now.Add(-15 * time.Minute).Format(time.RFC3339), EndTime: now.Format(time.RFC3339)}
	body, _ := json.Marshal(tw)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/correlation", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleUnifiedCorrelation(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 OK, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUnifiedCorrelation_TimeWindow_InvalidOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(nil)
	handler := &UnifiedQueryHandler{
		unifiedEngine: &mockUnifiedEngine{},
		logger:        mockLogger,
		kpiRepo:       nil,
	}

	now := time.Now().UTC()
	// endTime before startTime
	tw := models.TimeWindowRequest{StartTime: now.Format(time.RFC3339), EndTime: now.Add(-5 * time.Minute).Format(time.RFC3339)}
	body, _ := json.Marshal(tw)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/correlation", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleUnifiedCorrelation(c)

	if w.Code != 400 {
		t.Fatalf("expected 400 Bad Request for invalid window order, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUnifiedCorrelation_TimeWindow_BadTimestamp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(nil)
	handler := &UnifiedQueryHandler{
		unifiedEngine: &mockUnifiedEngine{},
		logger:        mockLogger,
		kpiRepo:       nil,
	}

	tw := models.TimeWindowRequest{StartTime: "not-a-date", EndTime: "also-not-a-date"}
	body, _ := json.Marshal(tw)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/correlation", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleUnifiedCorrelation(c)

	if w.Code != 400 {
		t.Fatalf("expected 400 Bad Request for bad timestamp format, got %d; body=%s", w.Code, w.Body.String())
	}
}
