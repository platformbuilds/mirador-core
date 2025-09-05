package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/platformbuilds/miradorstack/internal/api"
	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/internal/grpc/clients"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ---- Mocks ---------------------------------------------------------------
func NewMockGRPCClients() *clients.GRPCClients {
    gc := &clients.GRPCClients{}

	// Common field names we see in codebases; change to match yours.
	// Example A:
	//   type GRPCClients struct {
	//       Predict predictiface // interface with AnalyzeFractures
	//       RCA     rciface      // interface with InvestigateIncident
	//   }
	// Example B:
	//   PredictEngine PredictClient
	//   RCAEngine     RCAClient

	// Try these names first; if compile error, rename to your actual fields.
	// ------------------------------------------------------------------
	// gc.Predict = &MockPredictClient{}         // <-- adjust name if needed
	// gc.RCA = &MockRCAClient{}                 // <-- adjust name if needed
	// ------------------------------------------------------------------

    // Fallback option (uncomment if your struct uses different names):
    gc.PredictEngine = &MockPredictClient{}
    gc.RCAEngine     = &MockRCAClient{}

	return gc
}

// ---- Test Suite ----------------------------------------------------------

type APITestSuite struct {
	suite.Suite
	server     *api.Server
	testServer *httptest.Server
	client     *http.Client

	token string
	jti   string
	sub   string
	cfg   *config.Config
}

// ---- Test doubles for gRPC clients ----------------------------------------

type MockPredictClient struct{}

func (m *MockPredictClient) AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
    now := time.Now()
    return &models.FractureAnalysisResponse{
        Fractures: []*models.SystemFracture{
            {
                ID:             "fx-1",
                Component:      req.Component,
                FractureType:   "fatigue",
                TimeToFracture: 30 * time.Minute,
                Severity:       "medium",
                Probability:    0.82,
                Confidence:     0.76,
                PredictedAt:    now,
            },
        },
        ModelsUsed:       append([]string{"mock_model"}, req.ModelTypes...),
        ProcessingTimeMs: 12,
    }, nil
}

func (m *MockPredictClient) GetActiveModels(ctx context.Context, req *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
    return &models.ActiveModelsResponse{
        Models: []models.PredictionModel{
            {Name: "mock_model", Type: "fracture", Status: "active", Accuracy: 0.9},
        },
        LastUpdated: time.Now().Format(time.RFC3339),
    }, nil
}

func (m *MockPredictClient) HealthCheck() error { return nil }

type MockRCAClient struct{}

func (m *MockRCAClient) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
    return &models.CorrelationResult{
        CorrelationID:    "corr-1",
        IncidentID:       req.IncidentID,
        RootCause:        "database-connection-pool-exhaustion",
        Confidence:       0.88,
        AffectedServices: []string{"database", "payment-service"},
        Timeline: []models.TimelineEvent{
            {Event: "error_rate_spike", Severity: "high", Time: time.Now().Add(-20 * time.Minute), DataSource: "metrics"},
        },
        RedAnchors: []*models.RedAnchor{
            {Service: "database", Metric: "timeouts", Score: 0.92, Threshold: 0.8, Timestamp: time.Now(), DataType: "metrics"},
        },
        Recommendations: []string{"scale connection pool", "optimize queries"},
        CreatedAt:        time.Now(),
    }, nil
}

func (m *MockRCAClient) HealthCheck() error { return nil }

// If your real code uses interfaces, we still satisfy them; if it uses concrete
// types, the server’s GRPCClients bundle must expose fields we can replace.

// authJSON: always sends Authorization Bearer token (middleware picks this first)
func (suite *APITestSuite) authJSON(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+suite.token)
}

// Optional: create a JWT string (not strictly needed once we seed session)
func mintTestJWT(secret, sub, jti string, exp time.Time) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    "miradorstack",
		Subject:   sub,
		Audience:  jwt.ClaimStrings{"miradorstack-api"},
		ID:        jti,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

func (suite *APITestSuite) SetupSuite() {
	// Minimal test config
	cfg := &config.Config{
		Environment: "test",
		LogLevel:    "error",
	}

	// Ensure server reads a JWT secret if it needs it
	_ = os.Setenv("JWT_SECRET", "development-secret-key-not-for-production")
	if err := config.LoadSecrets(cfg); err != nil {
		panic(err)
	}

	suite.sub = "test-user"
	suite.jti = "test-jti"

	// We will use the JWT string itself as the session token/key
	token, err := mintTestJWT(cfg.Auth.JWT.Secret, suite.sub, suite.jti, time.Now().Add(1*time.Hour))
	if err != nil {
		panic(err)
	}
	suite.token = token
	suite.cfg = cfg

	log := logger.New("error")

	// Mocks
	mockCache := NewMockValkeyCluster()
	mockGRPCClients := NewMockGRPCClients()
	mockVMServices := NewMockVMServices()

	// *** CRITICAL ***
	// Seed a REAL session with KEY == token so validateSessionToken() passes.
	// middleware.validateSessionToken(c, token, cache) calls cache.GetSession(ctx, token)
	// so SetSession must store it under session.ID == token.
	session := &models.UserSession{
		ID:           suite.token, // <--- KEY MATCHES THE TOKEN
		UserID:       suite.sub,
		TenantID:     "default",
		Roles:        []string{"admin"},
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Settings:     map[string]interface{}{"email": "test@local"},
	}
	if err := mockCache.SetSession(context.Background(), session); err != nil {
		panic(err)
	}
	// ****************

	// Start server
	suite.server = api.NewServer(cfg, log, mockCache, mockGRPCClients, mockVMServices)
	suite.testServer = httptest.NewServer(suite.server.Handler())
	suite.client = &http.Client{Timeout: 10 * time.Second}
}

func (suite *APITestSuite) TearDownSuite() {
	suite.testServer.Close()
}

func (suite *APITestSuite) TestHealthEndpoint() {
	resp, err := suite.client.Get(suite.testServer.URL + "/health")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	assert.Equal(suite.T(), "healthy", body["status"])
}

// Dump exact auth errors if any
func dumpOnFailure(t *testing.T, resp *http.Response) {
	if resp.StatusCode == http.StatusOK {
		return
	}
	b, _ := io.ReadAll(resp.Body)
	t.Fatalf("status=%d body=%s", resp.StatusCode, string(b))
}

func (suite *APITestSuite) TestMetricsQLQuery() {
	payload := map[string]any{
		"query": "up",
		"time":  time.Now().Format(time.RFC3339),
	}
	jsonData, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", suite.testServer.URL+"/api/v1/query", bytes.NewReader(jsonData))
	suite.authJSON(req)

	resp, err := suite.client.Do(req)
	assert.NoError(suite.T(), err)
	dumpOnFailure(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	assert.Equal(suite.T(), "success", out["status"])
	assert.NotNil(suite.T(), out["data"])
}

func (suite *APITestSuite) TestPredictFractureAnalysis() {
	payload := map[string]any{
		"component":   "payment-service",
		"time_range":  "24h",
		"model_types": []string{"isolation_forest", "lstm_trend"},
	}
	jsonData, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", suite.testServer.URL+"/api/v1/predict/analyze", bytes.NewReader(jsonData))
	suite.authJSON(req)

	resp, err := suite.client.Do(req)
	assert.NoError(suite.T(), err)
	dumpOnFailure(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	assert.Equal(suite.T(), "success", out["status"])

	data := out["data"].(map[string]any)
	assert.NotNil(suite.T(), data["fractures"])
	assert.NotNil(suite.T(), data["metadata"])
}

func (suite *APITestSuite) TestRCAInvestigation() {
	payload := map[string]any{
		"incident_id": "INC-2025-0831-001",
		"symptoms":    []string{"high_cpu", "connection_timeouts", "error_rate_spike"},
		"time_range": map[string]any{
			"start": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			"end":   time.Now().Format(time.RFC3339),
		},
		"affected_services": []string{"payment-service", "database"},
	}
	jsonData, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", suite.testServer.URL+"/api/v1/rca/investigate", bytes.NewReader(jsonData))
	suite.authJSON(req)

	resp, err := suite.client.Do(req)
	assert.NoError(suite.T(), err)
	dumpOnFailure(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	assert.Equal(suite.T(), "success", out["status"])

	correlation := out["data"].(map[string]any)["correlation"].(map[string]any)
	assert.NotNil(suite.T(), correlation["root_cause"])
	assert.NotNil(suite.T(), correlation["red_anchors"])
}

func TestAPITestSuite(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}
