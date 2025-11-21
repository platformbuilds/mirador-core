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
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// fakeRCAEngine implements services.RCAEngine for testing
type fakeRCAEngine struct {
	t              *testing.T
	seenOpts       *rca.RCAOptions
	resultToReturn *rca.RCAIncident
}

func (f *fakeRCAEngine) ComputeRCA(ctx context.Context, incident *rca.IncidentContext, opts rca.RCAOptions) (*rca.RCAIncident, error) {
	f.seenOpts = &opts
	if f.resultToReturn != nil {
		return f.resultToReturn, nil
	}
	return &rca.RCAIncident{
		GeneratedAt: time.Now(),
		Score:       0.5,
		Notes:       []string{"test note"},
		Chains:      []*rca.RCAChain{},
	}, nil
}

func TestRCAHandler_PassesDimensionConfigToEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")
	cch := cache.NewNoopValkeyCache(log)
	logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log)

	// Create handler with a stub engine (will be replaced)
	rh := NewRCAHandler(logs, nil, cch, log, &fakeRCAEngine{t: t})

	// Create fake engine to capture options
	fakeEngine := &fakeRCAEngine{t: t}
	rh.SetEngine(fakeEngine)

	// Set up Gin router
	r := gin.New()
	r.POST("/api/v1/unified/rca", rh.HandleComputeRCA)

	// Create request with dimensionConfig
	now := time.Now().UTC()
	start := now.Add(-10 * time.Minute).Format(time.RFC3339)
	end := now.Format(time.RFC3339)

	requestBody := map[string]interface{}{
		"impactService": "svc-a",
		"impactMetric":  "traces_span_metrics_calls_total",
		"timeStart":     start,
		"timeEnd":       end,
		"severity":      0.8,
		"dimensionConfig": map[string]interface{}{
			"extraDimensions": []string{"env", "region"},
			"dimensionWeights": map[string]float64{
				"env":    0.2,
				"region": 0.1,
			},
			"alignmentPenalty": 0.2,
			"alignmentBonus":   0.1,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	// Send POST request
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/unified/rca", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Assert response is successful
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
	}

	// Assert that the fake engine was called and captured the options
	if fakeEngine.seenOpts == nil {
		t.Fatal("Engine was not called or options not captured")
	}

	// Assert DimensionConfig was passed correctly
	if len(fakeEngine.seenOpts.DimensionConfig.ExtraDimensions) == 0 {
		t.Fatal("DimensionConfig was not set in options")
	}

	expectedExtraDims := []string{"env", "region"}
	if len(fakeEngine.seenOpts.DimensionConfig.ExtraDimensions) != len(expectedExtraDims) {
		t.Fatalf("Expected %d extra dimensions, got %d", len(expectedExtraDims), len(fakeEngine.seenOpts.DimensionConfig.ExtraDimensions))
	}

	for i, expected := range expectedExtraDims {
		if fakeEngine.seenOpts.DimensionConfig.ExtraDimensions[i] != expected {
			t.Errorf("Expected extra dimension %d to be %s, got %s", i, expected, fakeEngine.seenOpts.DimensionConfig.ExtraDimensions[i])
		}
	}

	// Check dimension weights
	expectedWeights := map[string]float64{"env": 0.2, "region": 0.1}
	for k, v := range expectedWeights {
		if actual, ok := fakeEngine.seenOpts.DimensionConfig.DimensionWeights[k]; !ok || actual != v {
			t.Errorf("Expected dimension weight for %s to be %f, got %f", k, v, actual)
		}
	}

	// Check alignment penalty and bonus
	if fakeEngine.seenOpts.DimensionConfig.AlignmentPenalty != 0.2 {
		t.Errorf("Expected alignment penalty 0.2, got %f", fakeEngine.seenOpts.DimensionConfig.AlignmentPenalty)
	}
	if fakeEngine.seenOpts.DimensionConfig.AlignmentBonus != 0.1 {
		t.Errorf("Expected alignment bonus 0.1, got %f", fakeEngine.seenOpts.DimensionConfig.AlignmentBonus)
	}
}
