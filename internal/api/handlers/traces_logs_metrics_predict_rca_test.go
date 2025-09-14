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
    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/services"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// fakePredict implements clients.PredictClient
type fakePredict struct{ fail bool }
func (f *fakePredict) AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
    if f.fail { return nil, fmt.Errorf("failed") }
    return &models.FractureAnalysisResponse{ModelsUsed: []string{"m1"}, ProcessingTimeMs: 1, Fractures: []*models.SystemFracture{{ID: "f1", Component: "c1", PredictedAt: time.Now(), Probability: 0.8}}}, nil
}
func (f *fakePredict) GetActiveModels(ctx context.Context, req *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
    return &models.ActiveModelsResponse{Models: []models.PredictionModel{}}, nil
}
func (f *fakePredict) HealthCheck() error { return nil }

// fakeRCA implements clients.RCAClient
type fakeRCA struct{}
func (f *fakeRCA) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
    return &models.CorrelationResult{
        CorrelationID:    "c1",
        IncidentID:       req.IncidentID,
        RootCause:        "service-A",
        Confidence:       0.9,
        AffectedServices: []string{"service-A"},
        Timeline:         []models.TimelineEvent{},
        RedAnchors:       []*models.RedAnchor{},
        Recommendations:  []string{"restart service-A"},
        CreatedAt:        time.Now(),
    }, nil
}
func (f *fakeRCA) HealthCheck() error { return nil }

func TestPredict_RCA_and_OpenRoutes(t *testing.T) {
    gin.SetMode(gin.TestMode)
    log := logger.New("error")
    cch := cache.NewNoopValkeyCache(log)
    logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log)

    // Predict
    ph := NewPredictHandler(&fakePredict{}, logs, cch, log)
    r := gin.New()
    r.Use(func(c *gin.Context){ c.Set("tenant_id", "t1"); c.Next() })
    r.POST("/predict/analyze", ph.AnalyzeFractures)

    reqBody, _ := json.Marshal(models.FractureAnalysisRequest{Component: "comp", TimeRange: "1h"})
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/predict/analyze", bytes.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("predict analyze=%d body=%s", w.Code, w.Body.String()) }

    // RCA
    rh := NewRCAHandler(&fakeRCA{}, logs, cch, log)
    r = gin.New()
    r.Use(func(c *gin.Context){ c.Set("tenant_id", "t1"); c.Next() })
    r.POST("/rca/investigate", rh.StartInvestigation)
    tr := models.TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
    body, _ := json.Marshal(models.RCAInvestigationRequest{IncidentID: "i1", Symptoms: []string{"s"}, TimeRange: tr})
    w = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/rca/investigate", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("rca investigate=%d", w.Code) }
}
