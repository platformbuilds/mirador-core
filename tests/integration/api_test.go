package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/platformbuilds/miradorstack/internal/api"
	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/internal/grpc/clients"
	"github.com/platformbuilds/miradorstack/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// NewMockGRPCClients returns an empty gRPC client bundle suitable for these
// integration tests (VM services are mocked; we don't hit gRPC paths here).
// If you later add tests that call predict/rca/alert handlers, replace the
// zero fields with proper test doubles.
func NewMockGRPCClients() *clients.GRPCClients {
	return &clients.GRPCClients{}
}

type APITestSuite struct {
	suite.Suite
	server     *api.Server
	testServer *httptest.Server
	client     *http.Client
}

func (suite *APITestSuite) SetupSuite() {
	// Setup test configuration
	cfg := &config.Config{
		Environment: "test",
		LogLevel:    "error",
		// ... any other test config fields you need
	}

	log := logger.New("error")

	// Mock dependencies for testing
	mockCache := NewMockValkeyCluster()
	mockGRPCClients := NewMockGRPCClients()
	mockVMServices := NewMockVMServices()

	// Create test server
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

	var healthResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&healthResponse)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "healthy", healthResponse["status"])
}

func (suite *APITestSuite) TestMetricsQLQuery() {
	queryRequest := map[string]interface{}{
		"query": "up",
		"time":  time.Now().Format(time.RFC3339),
	}

	jsonData, _ := json.Marshal(queryRequest)

	req, _ := http.NewRequest("POST", suite.testServer.URL+"/api/v1/query", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-session-token")

	resp, err := suite.client.Do(req)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var queryResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&queryResponse)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "success", queryResponse["status"])
	assert.NotNil(suite.T(), queryResponse["data"])
}

func (suite *APITestSuite) TestPredictFractureAnalysis() {
	analysisRequest := map[string]interface{}{
		"component":   "payment-service",
		"time_range":  "24h",
		"model_types": []string{"isolation_forest", "lstm_trend"},
	}

	jsonData, _ := json.Marshal(analysisRequest)

	req, _ := http.NewRequest("POST", suite.testServer.URL+"/api/v1/predict/analyze", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-session-token")

	resp, err := suite.client.Do(req)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "success", response["status"])

	data := response["data"].(map[string]interface{})
	assert.NotNil(suite.T(), data["fractures"])
	assert.NotNil(suite.T(), data["metadata"])
}

func (suite *APITestSuite) TestRCAInvestigation() {
	investigationRequest := map[string]interface{}{
		"incident_id": "INC-2025-0831-001",
		"symptoms":    []string{"high_cpu", "connection_timeouts", "error_rate_spike"},
		"time_range": map[string]interface{}{
			"start": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			"end":   time.Now().Format(time.RFC3339),
		},
		"affected_services": []string{"payment-service", "database"},
	}

	jsonData, _ := json.Marshal(investigationRequest)

	req, _ := http.NewRequest("POST", suite.testServer.URL+"/api/v1/rca/investigate", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-session-token")

	resp, err := suite.client.Do(req)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "success", response["status"])

	correlation := response["data"].(map[string]interface{})["correlation"].(map[string]interface{})
	assert.NotNil(suite.T(), correlation["root_cause"])
	assert.NotNil(suite.T(), correlation["red_anchors"]) // Verify red anchors pattern
}

func TestAPITestSuite(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}
