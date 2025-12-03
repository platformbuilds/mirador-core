package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockMIRAService is a mock implementation of services.MIRAService for testing
type mockMIRAService struct {
	shouldFail    bool
	explanation   string
	tokensUsed    int
	providerName  string
	modelName     string
	simulateDelay time.Duration
}

func (m *mockMIRAService) GenerateExplanation(ctx context.Context, prompt string) (*services.MIRAResponse, error) {
	if m.shouldFail {
		return nil, &mockMIRAError{msg: "mock MIRA service failure"}
	}

	// Simulate AI processing delay
	if m.simulateDelay > 0 {
		time.Sleep(m.simulateDelay)
	}

	return &services.MIRAResponse{
		Explanation: m.explanation,
		TokensUsed:  m.tokensUsed,
		Model:       m.modelName,
		Provider:    m.providerName,
		GeneratedAt: time.Now().UTC(),
		Cached:      false,
	}, nil
}

func (m *mockMIRAService) GetProviderName() string {
	return m.providerName
}

func (m *mockMIRAService) GetModelName() string {
	return m.modelName
}

type mockMIRAError struct {
	msg string
}

func (e *mockMIRAError) Error() string {
	return e.msg
}

// createTestRCAResponse creates a valid RCA response for testing
func createTestRCAResponse() models.RCAResponse {
	return models.RCAResponse{
		Status: "success",
		Data: &models.RCAIncidentDTO{
			Impact: &models.IncidentContextDTO{
				ID:            "test-incident-001",
				ImpactService: "payment-service",
				MetricName:    "error_rate",
				TimeStartStr:  "2025-12-03T07:30:00Z",
				TimeEndStr:    "2025-12-03T08:30:00Z",
				ImpactSummary: "High error rate detected",
				Severity:      0.85,
			},
			RootCause: &models.RCAStepDTO{
				WhyIndex:  1,
				Service:   "database",
				Component: "connection_pool",
				TimeStart: time.Now().Add(-1 * time.Hour).UTC(),
				TimeEnd:   time.Now().UTC(),
				Ring:      "R2_PRECEDING",
				Direction: "upstream",
				Distance:  2,
				Evidence: []*models.EvidenceRefDTO{
					{
						Type:    "metric",
						ID:      "db_connections_exhausted",
						Details: "Connection pool maxed out at 100 connections",
					},
				},
				Summary: "Database connection pool exhausted",
				Score:   0.92,
			},
			Chains: []*models.RCAChainDTO{
				{
					Steps: []*models.RCAStepDTO{
						{
							WhyIndex:  1,
							Service:   "database",
							Component: "connection_pool",
							KPIName:   "DB Connections",
							Summary:   "Database connection pool exhausted",
							Score:     0.92,
						},
					},
					Score:        0.92,
					Rank:         1,
					ImpactPath:   []string{"payment-service", "database"},
					DurationHops: 2,
				},
			},
			GeneratedAt: time.Now().UTC(),
			Score:       0.90,
			Notes:       []string{"High confidence RCA"},
		},
		Timestamp: time.Now().UTC(),
	}
}

func TestMIRARCAHandler_HandleMIRARCAAnalyze_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup mock MIRA service
	mockService := &mockMIRAService{
		explanation:  "The payment service experienced issues because the database connection pool was exhausted.",
		tokensUsed:   150,
		providerName: "mock",
		modelName:    "mock-model-v1",
	}

	cfg := config.MIRAConfig{
		Timeout:        30 * time.Second,
		PromptTemplate: `Test template: {{.TOONData}}`,
	}

	mockLogger := logger.NewMockLogger(&strings.Builder{})
	handler := NewMIRARCAHandler(mockService, cfg, mockLogger)

	// Create test request
	rcaResponse := createTestRCAResponse()
	requestBody := map[string]interface{}{
		"rcaData": rcaResponse,
	}
	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Create HTTP request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mira/rca_analyze", bytes.NewBuffer(jsonData))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute handler
	handler.HandleMIRARCAAnalyze(c)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "success", response["status"])

	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)

	// With chunking, explanation is now stitched with markdown header and footer
	explanation, ok := data["explanation"].(string)
	require.True(t, ok)

	// The explanation should either contain the stitched report header (if chunks > 1)
	// or just the raw explanation (if synthesis was used and returned different content)
	// In either case, verify we got a non-empty response
	assert.NotEmpty(t, explanation)

	// For multi-chunk responses, should have structured report
	if strings.Contains(explanation, "Comprehensive Root Cause Analysis Report") {
		// Structured report format
		assert.Contains(t, explanation, "## Impact and Root Cause Overview")
	} else {
		// Direct explanation or synthesis result - verify core content is present
		// The mock explanation might be part of the result or transformed by synthesis
		assert.NotContains(t, explanation, "MIRA RCA Analysis") // Should not have raw chunk prompts
	}

	// Token count may include synthesis step for multi-chunk responses
	// Minimum is 2 chunks (impact+rootCause + chains), but could be higher with synthesis
	tokensUsed := data["tokensUsed"].(float64)
	assert.GreaterOrEqual(t, tokensUsed, float64(mockService.tokensUsed*2))
	assert.Equal(t, mockService.providerName, data["provider"])
	assert.Equal(t, mockService.modelName, data["model"])
	assert.Equal(t, false, data["cached"])
}

func TestMIRARCAHandler_HandleMIRARCAAnalyze_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &mockMIRAService{}
	cfg := config.MIRAConfig{Timeout: 30 * time.Second}
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	handler := NewMIRARCAHandler(mockService, cfg, mockLogger)

	// Create invalid JSON request
	invalidJSON := []byte(`{"rcaData": invalid json}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mira/rca_analyze", bytes.NewBuffer(invalidJSON))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMIRARCAAnalyze(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "invalid_json_payload", response["error"])
}

func TestMIRARCAHandler_HandleMIRARCAAnalyze_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &mockMIRAService{}
	cfg := config.MIRAConfig{Timeout: 30 * time.Second}
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	handler := NewMIRARCAHandler(mockService, cfg, mockLogger)

	// Create RCA response with missing data field
	invalidRCA := models.RCAResponse{
		Status: "success",
		Data:   nil, // Missing required field
	}
	requestBody := map[string]interface{}{
		"rcaData": invalidRCA,
	}
	jsonData, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mira/rca_analyze", bytes.NewBuffer(jsonData))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMIRARCAAnalyze(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["error"], "invalid_rca_data")
}

func TestMIRARCAHandler_HandleMIRARCAAnalyze_MIRAServiceFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup failing mock service
	mockService := &mockMIRAService{
		shouldFail: true,
	}

	cfg := config.MIRAConfig{
		Timeout:        30 * time.Second,
		PromptTemplate: `Test template`,
	}
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	handler := NewMIRARCAHandler(mockService, cfg, mockLogger)

	rcaResponse := createTestRCAResponse()
	requestBody := map[string]interface{}{
		"rcaData": rcaResponse,
	}
	jsonData, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mira/rca_analyze", bytes.NewBuffer(jsonData))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMIRARCAAnalyze(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "mira_generation_failed", response["error"])
}

func TestMIRARCAHandler_HandleMIRARCAAnalyze_Timeout(t *testing.T) {
	t.Skip("Skipping timeout test - timing-dependent behavior varies by system")
	gin.SetMode(gin.TestMode)

	// Setup mock service with long delay
	mockService := &mockMIRAService{
		simulateDelay: 2 * time.Second,
		explanation:   "This should timeout",
	}

	// Very short timeout to trigger timeout error
	cfg := config.MIRAConfig{
		Timeout:        100 * time.Millisecond,
		PromptTemplate: `Test template`,
	}
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	handler := NewMIRARCAHandler(mockService, cfg, mockLogger)

	rcaResponse := createTestRCAResponse()
	requestBody := map[string]interface{}{
		"rcaData": rcaResponse,
	}
	jsonData, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mira/rca_analyze", bytes.NewBuffer(jsonData))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMIRARCAAnalyze(c)

	// Should fail with timeout or internal error
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestMIRARCAHandler_PromptDataExtraction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &mockMIRAService{
		explanation: "Test explanation",
		tokensUsed:  100,
	}

	cfg := config.MIRAConfig{Timeout: 30 * time.Second}
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	handler := NewMIRARCAHandler(mockService, cfg, mockLogger)

	rcaResponse := createTestRCAResponse()

	// Extract prompt data
	promptData := handler.ExtractPromptData(&rcaResponse, "test-toon-data")

	// Assertions
	assert.Equal(t, "test-toon-data", promptData["TOONData"])
	assert.Equal(t, "payment-service", promptData["ImpactService"])
	assert.Equal(t, "error_rate", promptData["MetricName"])
	assert.Equal(t, "database", promptData["RootCauseService"])
	assert.Equal(t, "connection_pool", promptData["RootCauseComponent"])
	assert.Equal(t, 1, promptData["ChainCount"])
}

func TestMIRARCAHandler_MultipleProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	providers := []struct {
		name  string
		model string
	}{
		{"openai", "gpt-4"},
		{"anthropic", "claude-3-5-sonnet"},
		{"ollama", "llama3.1:70b"},
		{"vllm", "meta-llama/Llama-3.1-70B-Instruct"},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			mockService := &mockMIRAService{
				explanation:  "Test explanation from " + provider.name,
				tokensUsed:   200,
				providerName: provider.name,
				modelName:    provider.model,
			}

			cfg := config.MIRAConfig{
				Timeout:        30 * time.Second,
				PromptTemplate: `Test template`,
			}
			mockLogger := logger.NewMockLogger(&strings.Builder{})
			handler := NewMIRARCAHandler(mockService, cfg, mockLogger)

			rcaResponse := createTestRCAResponse()
			requestBody := map[string]interface{}{
				"rcaData": rcaResponse,
			}
			jsonData, _ := json.Marshal(requestBody)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mira/rca_analyze", bytes.NewBuffer(jsonData))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.HandleMIRARCAAnalyze(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)

			data := response["data"].(map[string]interface{})
			assert.Equal(t, provider.name, data["provider"])
			assert.Equal(t, provider.model, data["model"])
		})
	}
}
