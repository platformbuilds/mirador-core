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
func (f *fakeRCA) ListCorrelations(ctx context.Context, req *models.ListCorrelationsRequest) (*models.ListCorrelationsResponse, error) {
	return &models.ListCorrelationsResponse{Correlations: []models.CorrelationResult{}}, nil
}
func (f *fakeRCA) GetPatterns(ctx context.Context, req *models.GetPatternsRequest) (*models.GetPatternsResponse, error) {
	return &models.GetPatternsResponse{Patterns: []models.Pattern{}}, nil
}
func (f *fakeRCA) SubmitFeedback(ctx context.Context, req *models.FeedbackRequest) (*models.FeedbackResponse, error) {
	return &models.FeedbackResponse{CorrelationID: req.CorrelationID, Accepted: true}, nil
}
func (f *fakeRCA) HealthCheck() error { return nil }

type stubServiceGraph struct {
	data       *models.ServiceGraphData
	err        error
	lastTenant string
	lastReq    *models.ServiceGraphRequest
}

func (s *stubServiceGraph) FetchServiceGraph(ctx context.Context, tenantID string, req *models.ServiceGraphRequest) (*models.ServiceGraphData, error) {
	s.lastTenant = tenantID
	if req != nil {
		clone := *req
		s.lastReq = &clone
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.data, nil
}

func TestRCA_and_OpenRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")
	cch := cache.NewNoopValkeyCache(log)
	logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log)

	// RCA
	rh := NewRCAHandler(&fakeRCA{}, logs, nil, cch, log)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("tenant_id", "t1"); c.Next() })
	r.POST("/rca/investigate", rh.StartInvestigation)
	tr := models.TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	body, _ := json.Marshal(models.RCAInvestigationRequest{IncidentID: "i1", Symptoms: []string{"s"}, TimeRange: tr})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rca/investigate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rca investigate=%d", w.Code)
	}
}

func TestRCA_ServiceGraph_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")
	cch := cache.NewNoopValkeyCache(log)
	logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log)

	window := models.ServiceGraphWindow{Start: time.Unix(0, 0).UTC(), End: time.Unix(300, 0).UTC(), DurationSeconds: 300}
	sg := &stubServiceGraph{data: &models.ServiceGraphData{
		Window: window,
		Edges: []models.ServiceGraphEdge{{
			Source: "checkout", Target: "payments", CallCount: 60, CallRate: 12, ErrorRate: 5,
		}},
	}}

	rh := NewRCAHandler(&fakeRCA{}, logs, sg, cch, log)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("tenant_id", "tenant-1"); c.Next() })
	r.POST("/rca/service-graph", rh.GetServiceGraph)

	body := []byte(`{"start":"1970-01-01T00:00:00Z","end":"1970-01-01T00:05:00Z"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rca/service-graph", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("service graph status=%d body=%s", w.Code, w.Body.String())
	}
	if sg.lastTenant != "tenant-1" {
		t.Fatalf("expected tenant tenant-1, got %s", sg.lastTenant)
	}
	if sg.lastReq == nil || sg.lastReq.Start.IsZero() || sg.lastReq.End.IsZero() {
		t.Fatalf("expected request to propagate start/end")
	}

	var resp struct {
		Status string                    `json:"status"`
		Edges  []models.ServiceGraphEdge `json:"edges"`
		Window models.ServiceGraphWindow `json:"window"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "success" || len(resp.Edges) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Edges[0].Source != "checkout" {
		t.Fatalf("unexpected edge payload: %+v", resp.Edges[0])
	}
	if resp.Window.DurationSeconds != 300 {
		t.Fatalf("unexpected window: %+v", resp.Window)
	}
}

func TestRCA_ServiceGraph_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")
	cch := cache.NewNoopValkeyCache(log)
	logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log)

	rh := NewRCAHandler(&fakeRCA{}, logs, nil, cch, log)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("tenant_id", "tenant-err"); c.Next() })
	r.POST("/rca/service-graph", rh.GetServiceGraph)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rca/service-graph", bytes.NewBufferString(`{"start":"1970-01-01T00:00:00Z","end":"1970-01-01T00:01:00Z"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	sg := &stubServiceGraph{}
	rh = NewRCAHandler(&fakeRCA{}, logs, sg, cch, log)
	r = gin.New()
	r.Use(func(c *gin.Context) { c.Set("tenant_id", "tenant-err"); c.Next() })
	r.POST("/rca/service-graph", rh.GetServiceGraph)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/rca/service-graph", bytes.NewBufferString(`{"start":"not-a-time"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	sg.err = fmt.Errorf("boom")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/rca/service-graph", bytes.NewBufferString(`{"start":"1970-01-01T00:00:00Z","end":"1970-01-01T00:01:00Z"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
