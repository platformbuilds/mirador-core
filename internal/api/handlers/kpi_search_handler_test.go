package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Add a SearchKPIs method to the existing mockRepo defined in kpi_bulk_handler_test.go
func (m *mockRepo) SearchKPIs(ctx context.Context, req models.KPISearchRequest) ([]models.KPISearchResult, int64, error) {
	// Return a simple mock result
	r := models.KPISearchResult{
		ID:    "kpi-1",
		Name:  "payment latency",
		Score: 0.95,
	}
	return []models.KPISearchResult{r}, 1, nil
}

func setupSearchHandler() (*KPIHandler, *mockRepo) {
	mr := &mockRepo{upserted: []*models.KPIDefinition{}}
	l := logger.NewMockLogger(nil)
	cfg := &config.Config{}
	h := &KPIHandler{repo: mr, cache: nil, logger: l, cfg: cfg}
	return h, mr
}

func TestSearchKPIs_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupSearchHandler()

	body := map[string]any{"query": "payment latency"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/kpi/search", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.SearchKPIs(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSearchKPIs_MissingQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupSearchHandler()

	body := map[string]any{"query": ""}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/kpi/search", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.SearchKPIs(c)

	if w.Code != 400 {
		t.Fatalf("expected 400 got %d body=%s", w.Code, w.Body.String())
	}
}
