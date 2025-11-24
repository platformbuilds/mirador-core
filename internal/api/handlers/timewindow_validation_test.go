package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// fakeUnifiedEngine implements only the method used by the handler in this test
type fakeUnifiedEngine struct{}

func (f *fakeUnifiedEngine) ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return &models.UnifiedResult{}, nil
}
func (f *fakeUnifiedEngine) ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return &models.UnifiedResult{}, nil
}
func (f *fakeUnifiedEngine) ExecuteUQLQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return &models.UnifiedResult{}, nil
}
func (f *fakeUnifiedEngine) GetQueryMetadata(ctx context.Context) (*models.QueryMetadata, error) {
	return &models.QueryMetadata{}, nil
}
func (f *fakeUnifiedEngine) HealthCheck(ctx context.Context) (*models.EngineHealthStatus, error) {
	return &models.EngineHealthStatus{OverallHealth: "healthy", EngineHealth: map[models.QueryType]string{}}, nil
}
func (f *fakeUnifiedEngine) InvalidateCache(ctx context.Context, queryPattern string) error {
	return nil
}
func (f *fakeUnifiedEngine) DetectComponentFailures(ctx context.Context, timeRange models.TimeRange, components []models.FailureComponent) (*models.FailureCorrelationResult, error) {
	return &models.FailureCorrelationResult{}, nil
}
func (f *fakeUnifiedEngine) CorrelateTransactionFailures(ctx context.Context, transactionIDs []string, timeRange models.TimeRange) (*models.FailureCorrelationResult, error) {
	return &models.FailureCorrelationResult{}, nil
}

func TestHandleUnifiedCorrelation_StrictEnforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")

	// Engine config: min window = 1m, strict = true
	cfg := config.EngineConfig{
		MinWindow:        1 * time.Minute,
		MaxWindow:        10 * time.Minute,
		StrictTimeWindow: true,
	}

	h := NewUnifiedQueryHandler(&fakeUnifiedEngine{}, log, nil, cfg)
	r := gin.New()
	r.POST("/correlation", h.HandleUnifiedCorrelation)

	// Create a time window shorter than min (30s)
	now := time.Now().UTC()
	start := now.Add(-30 * time.Second).Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	body := map[string]string{"startTime": start, "endTime": end}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/correlation", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when strict and window too small, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUnifiedCorrelation_LenientEnforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")

	// Engine config: min window = 1m, strict = false
	cfg := config.EngineConfig{
		MinWindow:        1 * time.Minute,
		MaxWindow:        10 * time.Minute,
		StrictTimeWindow: false,
	}

	h := NewUnifiedQueryHandler(&fakeUnifiedEngine{}, log, nil, cfg)
	r := gin.New()
	r.POST("/correlation", h.HandleUnifiedCorrelation)

	// Create a time window shorter than min (30s)
	now := time.Now().UTC()
	start := now.Add(-30 * time.Second).Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	body := map[string]string{"startTime": start, "endTime": end}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/correlation", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// In lenient mode the handler should allow the request to proceed and our fake engine returns 200
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when lenient and window too small, got %d; body=%s", w.Code, w.Body.String())
	}
}
