package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/proto/predict"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

/* -------------------------------------------------------------------------- */
/*                           Mock Valkey (in-memory)                           */
/* -------------------------------------------------------------------------- */

type mockValkey struct {
	kv       map[string][]byte
	sessions map[string]*models.UserSession
}

func NewMockValkeyCluster() cache.ValkeyCluster {
	return &mockValkey{
		kv:       make(map[string][]byte),
		sessions: make(map[string]*models.UserSession),
	}
}

func (m *mockValkey) Get(ctx context.Context, key string) ([]byte, error) {
	if v, ok := m.kv[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockValkey) Set(ctx context.Context, key string, value interface{}, _ time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.kv[key] = b
	return nil
}

func (m *mockValkey) Delete(ctx context.Context, key string) error {
	delete(m.kv, key)
	return nil
}

func (m *mockValkey) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (m *mockValkey) SetSession(ctx context.Context, s *models.UserSession) error {
	if s.ID == "" {
		s.ID = "test-session-token"
	}
	m.sessions[s.ID] = s
	return nil
}

func (m *mockValkey) InvalidateSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockValkey) GetActiveSessions(ctx context.Context, _ string) ([]*models.UserSession, error) {
	out := make([]*models.UserSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out, nil
}

func (m *mockValkey) CacheQueryResult(ctx context.Context, key string, v interface{}, _ time.Duration) error {
	return m.Set(ctx, "query_cache:"+key, v, 0)
}

func (m *mockValkey) GetCachedQueryResult(ctx context.Context, key string) ([]byte, error) {
	return m.Get(ctx, "query_cache:"+key)
}

/* -------------------------------------------------------------------------- */
/*                   Mock gRPC clients (Predict/RCA engines)                  */
/* -------------------------------------------------------------------------- */

// mockPredictEngineClient implements the Predict client used by your handlers.
type mockPredictEngineClient struct{}

// Returns a small, realistic inventory of models
func (m *mockPredictEngineClient) GetModels(ctx context.Context) ([]*predict.MLModel, error) {
	return []*predict.MLModel{
		{
			Name:        "isolation_forest",
			Type:        "anomaly",
			Status:      "active",
			Accuracy:    0.93,
			LastTrained: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		},
		{
			Name:        "lstm_trend",
			Type:        "forecast",
			Status:      "active",
			Accuracy:    0.90,
			LastTrained: time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
		},
	}, nil
}

// If your code calls the convenience wrapper, return a mapped version
func (m *mockPredictEngineClient) GetActiveModels(ctx context.Context, _ *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
	// Reuse GetModels to keep things in sync
	ml, _ := m.GetModels(ctx)

	out := make([]models.PredictionModel, 0, len(ml))
	for _, mm := range ml {
		out = append(out, models.PredictionModel{
			ID:          "", // not present in proto; fine for tests
			Name:        mm.Name,
			Version:     "", // not present in proto
			Type:        mm.Type,
			Status:      mm.Status,
			Accuracy:    mm.Accuracy,
			CreatedAt:   mm.LastTrained, // we map last_trained -> created/updated
			UpdatedAt:   mm.LastTrained,
			Description: "",
			Parameters:  map[string]interface{}{},
			Metrics:     models.ModelMetrics{}, // empty test metrics
		})
	}
	return &models.ActiveModelsResponse{
		Models:      out,
		LastUpdated: time.Now().Format(time.RFC3339),
	}, nil
}

/* -------------------------------------------------------------------------- */
/*                 Mock Victoria* services via httptest servers               */
/* -------------------------------------------------------------------------- */

// NewMockVMServices returns a *services.VictoriaMetricsServices whose HTTP clients
// talk to local httptest servers that emulate VM / VL APIs used by handlers.
func NewMockVMServices() *services.VictoriaMetricsServices {
	// ---- metrics (Prometheus-compatible) /api/v1/query ----
	metricsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		// minimal valid response
		resp := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{
						"metric": map[string]string{"__name__": "up"},
						"value":  []any{time.Now().Unix(), "1"},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	// ---- logs simple endpoints we use in tests (none heavily used) ----
	logsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// return a tiny but valid json envelope for whatever path is queried
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   []any{},
		})
	}))

	// build real services pointing to those fake endpoints
	vmSvc := services.NewVictoriaMetricsService(
		config.VictoriaMetricsConfig{
			Endpoints: []string{metricsSrv.URL},
			Timeout:   2000,
		},
		// no explicit logger needed here
		loggerNoop{},
	)

	vlSvc := services.NewVictoriaLogsService(
		config.VictoriaLogsConfig{
			Endpoints: []string{logsSrv.URL},
			Timeout:   2000,
		},
		loggerNoop{},
	)

	// Traces isn’t exercised by current tests; a nil or zero value is fine
	// if your NewTracesHandler guards against nil. If not, add another httptest.Server
	// identical to logs one and a tiny wrapper service.

	return &services.VictoriaMetricsServices{
		Metrics: vmSvc,
		Logs:    vlSvc,
		Traces:  nil,
	}
}

/* ----------------------------- tiny test logger ---------------------------- */

type loggerNoop struct{}

func (loggerNoop) Info(msg string, kv ...interface{})  {}
func (loggerNoop) Warn(msg string, kv ...interface{})  {}
func (loggerNoop) Error(msg string, kv ...interface{}) {}
func (loggerNoop) Debug(msg string, kv ...interface{}) {}
func (loggerNoop) Fatal(msg string, kv ...interface{}) {}
