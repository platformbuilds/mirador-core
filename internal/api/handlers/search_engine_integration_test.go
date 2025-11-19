package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TestSearchEngineIntegration tests the search_engine parameter functionality
func TestSearchEngineIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")
	cch := cache.NewNoopValkeyCache(log)
	logsSvc := services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log)
	tracesSvc := services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log)

	// Create test config with Bleve enabled
	testConfig := &config.Config{
		Environment: "test",
		Search: config.SearchConfig{
			DefaultEngine: "lucene",
			EnableBleve:   true,
			EnableLucene:  true,
		},
	}

	// Create search router with both engines enabled
	searchConfig := &search.SearchConfig{
		DefaultEngine: "lucene",
		EnableBleve:   true,
		EnableLucene:  true,
	}
	router, _ := search.NewSearchRouter(searchConfig, log)

	// Test LogsQL Handler
	t.Run("LogsQLHandler_LuceneEngine", func(t *testing.T) {
		lh := NewLogsQLHandler(logsSvc, cch, log, router, testConfig)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("default", "test"); c.Next() })
		r.POST("/logs/query", lh.ExecuteQuery)

		reqBody := models.LogsQLQueryRequest{
			Query:        "error",
			SearchEngine: "lucene",
			Start:        1704067200000, // 2024-01-01T00:00:00Z in milliseconds
			End:          1704153600000, // 2024-01-02T00:00:00Z in milliseconds
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/logs/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// Should succeed (even if backend fails, the routing should work)
		if w.Code == http.StatusBadRequest {
			// Check if it's a routing error vs backend error
			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if errMsg, ok := resp["error"].(string); ok && errMsg == "Unsupported search engine: lucene" {
				t.Fatalf("Search engine routing failed: %s", errMsg)
			}
		}
	})

	t.Run("LogsQLHandler_BleveEngine", func(t *testing.T) {
		lh := NewLogsQLHandler(logsSvc, cch, log, router, testConfig)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("default", "test"); c.Next() })
		r.POST("/logs/query", lh.ExecuteQuery)

		reqBody := models.LogsQLQueryRequest{
			Query:        "error",
			SearchEngine: "bleve",
			Start:        1704067200000,
			End:          1704153600000,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/logs/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// Should succeed (even if backend fails, the routing should work)
		if w.Code == http.StatusBadRequest {
			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if errMsg, ok := resp["error"].(string); ok && errMsg == "Unsupported search engine: bleve" {
				t.Fatalf("Search engine routing failed: %s", errMsg)
			}
		}
	})

	t.Run("LogsQLHandler_DefaultEngine", func(t *testing.T) {
		lh := NewLogsQLHandler(logsSvc, cch, log, router, testConfig)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("default", "test"); c.Next() })
		r.POST("/logs/query", lh.ExecuteQuery)

		reqBody := models.LogsQLQueryRequest{
			Query: "error",
			// No SearchEngine specified - should default to lucene
			Start: 1704067200000,
			End:   1704153600000,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/logs/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// Should succeed with default engine
		if w.Code == http.StatusBadRequest {
			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if errMsg, ok := resp["error"].(string); ok && errMsg == "Unsupported search engine: lucene" {
				t.Fatalf("Default search engine routing failed: %s", errMsg)
			}
		}
	})

	t.Run("LogsQLHandler_InvalidEngine", func(t *testing.T) {
		lh := NewLogsQLHandler(logsSvc, cch, log, router, testConfig)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("default", "test"); c.Next() })
		r.POST("/logs/query", lh.ExecuteQuery)

		reqBody := models.LogsQLQueryRequest{
			Query:        "error",
			SearchEngine: "invalid",
			Start:        1704067200000,
			End:          1704153600000,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/logs/query", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// Should return 400 for invalid engine
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 for invalid engine, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if errMsg, ok := resp["error"].(string); !ok || errMsg != "Unsupported search engine: invalid" {
			t.Fatalf("Expected 'Unsupported search engine: invalid' error, got: %s", w.Body.String())
		}
	})

	// Test Traces Handler
	t.Run("TracesHandler_LuceneEngine", func(t *testing.T) {
		th := NewTracesHandler(tracesSvc, cch, log, router, testConfig)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("default", "test"); c.Next() })
		r.POST("/traces/search", th.SearchTraces)

		reqBody := models.TraceSearchRequest{
			Query:        "service:checkout",
			SearchEngine: "lucene",
			Start:        models.FlexibleTime{Time: time.UnixMilli(1704067200000)},
			End:          models.FlexibleTime{Time: time.UnixMilli(1704153600000)},
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/traces/search", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// Should succeed (even if backend fails, the routing should work)
		if w.Code == http.StatusBadRequest {
			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if errMsg, ok := resp["error"].(string); ok && errMsg == "Unsupported search engine: lucene" {
				t.Fatalf("Search engine routing failed: %s", errMsg)
			}
		}
	})

	t.Run("TracesHandler_BleveEngine", func(t *testing.T) {
		th := NewTracesHandler(tracesSvc, cch, log, router, testConfig)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("default", "test"); c.Next() })
		r.POST("/traces/search", th.SearchTraces)

		reqBody := models.TraceSearchRequest{
			Query:        "service:checkout",
			SearchEngine: "bleve",
			Start:        models.FlexibleTime{Time: time.UnixMilli(1704067200000)},
			End:          models.FlexibleTime{Time: time.UnixMilli(1704153600000)},
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/traces/search", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// Should succeed (even if backend fails, the routing should work)
		if w.Code == http.StatusBadRequest {
			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if errMsg, ok := resp["error"].(string); ok && errMsg == "Unsupported search engine: bleve" {
				t.Fatalf("Search engine routing failed: %s", errMsg)
			}
		}
	})

	t.Run("TracesHandler_InvalidEngine", func(t *testing.T) {
		th := NewTracesHandler(tracesSvc, cch, log, router, testConfig)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("default", "test"); c.Next() })
		r.POST("/traces/search", th.SearchTraces)

		reqBody := models.TraceSearchRequest{
			Query:        "service:checkout",
			SearchEngine: "invalid",
			Start:        models.FlexibleTime{Time: time.UnixMilli(1704067200000)},
			End:          models.FlexibleTime{Time: time.UnixMilli(1704153600000)},
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/traces/search", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// Should return 400 for invalid engine
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400 for invalid engine, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if errMsg, ok := resp["error"].(string); !ok || errMsg != "Unsupported search engine: invalid" {
			t.Fatalf("Expected 'Unsupported search engine: invalid' error, got: %s", w.Body.String())
		}
	})
}
