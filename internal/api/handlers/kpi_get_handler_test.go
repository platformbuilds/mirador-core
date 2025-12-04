package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRepoGet embeds the existing mockRepo and overrides GetKPI to return
// a programmable value for tests.
type mockRepoGet struct {
	mockRepo
	ret *models.KPIDefinition
	err error
}

func (m *mockRepoGet) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	return m.ret, m.err
}

// Ensure other required methods are satisfied by embedding mockRepo (defined in
// kpi_bulk_handler_test.go).

func setupGetHandler(ret *models.KPIDefinition, err error) (*KPIHandler, *mockRepoGet) {
	mr := &mockRepoGet{mockRepo: mockRepo{upserted: []*models.KPIDefinition{}}, ret: ret, err: err}
	l := logger.NewMockLogger(&strings.Builder{})
	cfg := &config.Config{}
	h := &KPIHandler{repo: mr, cache: nil, logger: l, cfg: cfg}
	return h, mr
}

func TestGetKPIDefinition_Found(t *testing.T) {
	gin.SetMode(gin.TestMode)
	want := &models.KPIDefinition{ID: "kpi-1", Name: "kpi-1", Definition: "an example"}
	h, _ := setupGetHandler(want, nil)

	req := httptest.NewRequest("GET", "/api/v1/kpi/defs/kpi-1", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	// set path param
	c.Params = gin.Params{{Key: "id", Value: "kpi-1"}}

	h.GetKPIDefinition(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var got models.KPIDefinition
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if got.ID != want.ID || got.Name != want.Name {
		t.Fatalf("unexpected kpi returned: %+v", got)
	}
}

func TestGetKPIDefinition_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := setupGetHandler(nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/kpi/defs/not-exist", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "not-exist"}}

	h.GetKPIDefinition(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", w.Code, w.Body.String())
	}
}
