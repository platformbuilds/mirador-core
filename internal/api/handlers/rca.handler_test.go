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
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
)

type fakeRCAClient struct{}

func (f *fakeRCAClient) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	return &models.CorrelationResult{
		CorrelationID:    "corr-xyz",
		IncidentID:       req.IncidentID,
		RootCause:        "db-connection-exhaustion",
		Confidence:       0.9,
		AffectedServices: []string{"db", "payments"},
		RedAnchors:       []*models.RedAnchor{{Service: "db", Metric: "timeouts", Score: 0.95, Threshold: 0.8, Timestamp: time.Now(), DataType: "metrics"}},
		CreatedAt:        time.Now(),
	}, nil
}
func (f *fakeRCAClient) HealthCheck() error { return nil }

// Minimal fake cache to satisfy constructor
type dummyCache struct{ cache.ValkeyCluster }

func TestRCA_StartInvestigation_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{"http://127.0.0.1:9"}, Timeout: 500}, logger.New("error")) // not used here
	h := NewRCAHandler(clients.RCAClient(&fakeRCAClient{}), logs, &dummyCache{}, logger.New("error"))

	payload := map[string]any{
		"incident_id": "INC-1",
		"symptoms":    []string{"errors"},
		"time_range":  map[string]any{"start": time.Now().Add(-1 * time.Hour).Format(time.RFC3339), "end": time.Now().Format(time.RFC3339)},
	}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rca/investigate", bytes.NewReader(b))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("tenant_id", "t1")

	h.StartInvestigation(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	assert.Equal(t, "success", out["status"])
	corr := out["data"].(map[string]any)["correlation"].(map[string]any)
	assert.NotEmpty(t, corr["root_cause"])
}
