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
    "github.com/platformbuilds/miradorstack/internal/config"
    "github.com/platformbuilds/miradorstack/internal/models"
    "github.com/platformbuilds/miradorstack/internal/services"
    "github.com/platformbuilds/miradorstack/pkg/cache"
    "github.com/platformbuilds/miradorstack/pkg/logger"
    "github.com/stretchr/testify/assert"
)

// ---- Fakes / Mocks ---------------------------------------------------------

type fakePredictClient struct{}

func (f *fakePredictClient) AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
    now := time.Now()
    return &models.FractureAnalysisResponse{
        Fractures: []*models.SystemFracture{{
            ID:             "fx-test",
            Component:      req.Component,
            FractureType:   "fatigue",
            TimeToFracture: 10 * time.Minute,
            Severity:       "high",
            Probability:    0.91,
            Confidence:     0.8,
            PredictedAt:    now,
        }},
        ModelsUsed:       []string{"mock"},
        ProcessingTimeMs: 5,
    }, nil
}

func (f *fakePredictClient) GetActiveModels(ctx context.Context, req *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
    return &models.ActiveModelsResponse{Models: []models.PredictionModel{{Name: "m1", Type: "fracture", Status: "active"}}}, nil
}
func (f *fakePredictClient) HealthCheck() error { return nil }

type fakeCache struct{ m map[string][]byte }

func (c *fakeCache) Get(ctx context.Context, key string) ([]byte, error) {
    if v, ok := c.m[key]; ok {
        return v, nil
    }
    return nil, assert.AnError
}
func (c *fakeCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    if c.m == nil {
        c.m = map[string][]byte{}
    }
    b, _ := json.Marshal(value)
    c.m[key] = b
    return nil
}
func (c *fakeCache) Delete(ctx context.Context, key string) error                                  { return nil }
func (c *fakeCache) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) { return nil, nil }
func (c *fakeCache) SetSession(ctx context.Context, session *models.UserSession) error             { return nil }
func (c *fakeCache) InvalidateSession(ctx context.Context, sessionID string) error                 { return nil }
func (c *fakeCache) GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error) {
    return nil, nil
}
func (c *fakeCache) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
    return nil
}
func (c *fakeCache) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) { return nil, nil }

// ---- Tests -----------------------------------------------------------------

func TestPredict_AnalyzeFractures_StoresEventsAndRespondsOK(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // Fake VictoriaLogs HTTP server to accept StoreJSONEvent
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.Method == http.MethodPost && r.URL.Path == "/insert/jsonline":
            w.WriteHeader(http.StatusOK)
        default:
            http.NotFound(w, r)
        }
    }))
    defer srv.Close()

    // Build logs service pointing to our fake server
    logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))

    // Handler under test
    h := NewPredictHandler(&fakePredictClient{}, logs, cache.ValkeyCluster(&fakeCache{}), logger.New("error"))

    // Request
    body := map[string]any{"component": "payments", "time_range": "24h"}
    b, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/predict/analyze", bytes.NewReader(b))
    req = req.WithContext(context.WithValue(req.Context(), struct{}{}, nil))
    w := httptest.NewRecorder()

    c, _ := gin.CreateTestContext(w)
    c.Request = req
    // emulate tenant injection middleware
    c.Set("tenant_id", "t1")

    h.AnalyzeFractures(c)

    assert.Equal(t, http.StatusOK, w.Code)
    var out map[string]any
    _ = json.Unmarshal(w.Body.Bytes(), &out)
    assert.Equal(t, "success", out["status"])
    data := out["data"].(map[string]any)
    _, ok := data["fractures"].([]any)
    assert.True(t, ok)
}

func TestPredict_GetActiveModels_CacheMissThenHit(t *testing.T) {
    gin.SetMode(gin.TestMode)
    logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{"http://127.0.0.1:9"}, Timeout: 500}, logger.New("error")) // unused
    fcache := &fakeCache{m: map[string][]byte{}}
    h := NewPredictHandler(&fakePredictClient{}, logs, fcache, logger.New("error"))

    // First request: MISS
    req := httptest.NewRequest(http.MethodGet, "/api/v1/predict/models", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Set("tenant_id", "t1")
    h.GetActiveModels(c)
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Equal(t, "MISS", w.Header().Get("X-Cache"))

    // Second request: HIT
    req2 := httptest.NewRequest(http.MethodGet, "/api/v1/predict/models", nil)
    w2 := httptest.NewRecorder()
    c2, _ := gin.CreateTestContext(w2)
    c2.Request = req2
    c2.Set("tenant_id", "t1")
    h.GetActiveModels(c2)
    assert.Equal(t, http.StatusOK, w2.Code)
    assert.Equal(t, "HIT", w2.Header().Get("X-Cache"))
}
