package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/models"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// mock engine that returns a correlation result with resolved KPI info
type mockResolvedUnifiedEngine struct{}

func (m *mockResolvedUnifiedEngine) ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return &models.UnifiedResult{QueryID: query.ID, Type: query.Type, Status: "success"}, nil
}

func (m *mockResolvedUnifiedEngine) ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	// Build a CorrelationResult with a resolved CauseCandidate
	rr := &models.CorrelationResult{
		CorrelationID: "corr_test",
		Confidence:    0.9,
		Causes: []models.CauseCandidate{
			{KPI: "Transaction Success Rate", KPIUUID: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", KPIFormula: "sum(rate(transactions_total{status=\"success\"}[5m]))", SuspicionScore: 0.8},
		},
	}
	return &models.UnifiedResult{QueryID: query.ID, Type: query.Type, Status: "success", Data: rr}, nil
}

func (m *mockResolvedUnifiedEngine) ExecuteUQLQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return m.ExecuteQuery(ctx, query)
}
func (m *mockResolvedUnifiedEngine) GetQueryMetadata(ctx context.Context) (*models.QueryMetadata, error) {
	return &models.QueryMetadata{}, nil
}
func (m *mockResolvedUnifiedEngine) HealthCheck(ctx context.Context) (*models.EngineHealthStatus, error) {
	return &models.EngineHealthStatus{OverallHealth: "healthy"}, nil
}
func (m *mockResolvedUnifiedEngine) InvalidateCache(ctx context.Context, queryPattern string) error {
	return nil
}
func (m *mockResolvedUnifiedEngine) DetectComponentFailures(ctx context.Context, timeRange models.TimeRange, components []models.FailureComponent) (*models.FailureCorrelationResult, error) {
	return &models.FailureCorrelationResult{}, nil
}
func (m *mockResolvedUnifiedEngine) CorrelateTransactionFailures(ctx context.Context, transactionIDs []string, timeRange models.TimeRange) (*models.FailureCorrelationResult, error) {
	return &models.FailureCorrelationResult{}, nil
}

func TestHandleUnifiedCorrelation_HandlerReturnsResolvedKPINames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := &mockResolvedUnifiedEngine{}

	handler := &UnifiedQueryHandler{unifiedEngine: engine, logger: corelogger.NewMockLogger(nil), kpiRepo: nil}

	now := time.Now().UTC()
	tw := models.TimeWindowRequest{StartTime: now.Add(-15 * time.Minute).Format(time.RFC3339), EndTime: now.Format(time.RFC3339)}
	body, _ := json.Marshal(tw)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/correlation", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleUnifiedCorrelation(c)

	require.Equal(t, 200, w.Code)

	var resp models.UnifiedQueryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Verify Data contains CorrelationResult with resolved KPI name and uuid
	dataMap, ok := resp.Result.Data.(map[string]interface{})
	require.True(t, ok, "expected UnifiedResult.Data to be JSON object")
	causes, ok := dataMap["causes"].([]interface{})
	require.True(t, ok && len(causes) > 0, "expected causes array in result data")
	first := causes[0].(map[string]interface{})

	require.Equal(t, "Transaction Success Rate", first["kpi"])
	require.Equal(t, "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", first["kpiUuid"])
	require.Equal(t, "sum(rate(transactions_total{status=\"success\"}[5m]))", first["kpiFormula"])
}
