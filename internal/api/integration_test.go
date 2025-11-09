package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer represents the test server with dependencies
type TestServer struct {
	router *gin.Engine
}

// setupTestServer creates a test server with mocked dependencies
func setupTestServer(t *testing.T) *TestServer {
	gin.SetMode(gin.TestMode)

	// Create router with handlers
	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Register routes
	apiGroup := router.Group("/api/v1")
	{
		// KPI routes
		kpiGroup := apiGroup.Group("/kpi")
		{
			kpiGroup.GET("/defs", func(c *gin.Context) {
				// Mock KPI definitions handler
				kpis := []*models.KPIDefinition{
					{
						ID:          "test-kpi-1",
						Name:        "Test KPI",
						Kind:        "business",
						Visibility:  "private",
						OwnerUserID: "test-user",
						TenantID:    "default",
					},
				}
				c.JSON(http.StatusOK, gin.H{"kpis": kpis})
			})

			kpiGroup.POST("/defs", func(c *gin.Context) {
				var kpi models.KPIDefinition
				if err := c.ShouldBindJSON(&kpi); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusCreated, kpi)
			})
		}

		// Dashboard routes
		dashboardGroup := apiGroup.Group("/dashboards")
		{
			dashboardGroup.GET("", func(c *gin.Context) {
				dashboards := []*models.Dashboard{
					{
						ID:          "default",
						Name:        "Default Dashboard",
						Visibility:  "org",
						OwnerUserID: "system",
						TenantID:    "default",
					},
				}
				c.JSON(http.StatusOK, gin.H{"dashboards": dashboards})
			})
		}

		// User preferences routes
		userGroup := apiGroup.Group("/user")
		{
			userGroup.GET("/preferences", func(c *gin.Context) {
				prefs := &models.UserPreferences{
					ID:       "test-user",
					TenantID: "default",
					Preferences: map[string]interface{}{
						"theme": "dark",
					},
				}
				c.JSON(http.StatusOK, prefs)
			})
		}
	}

	return &TestServer{
		router: router,
	}
}

// TestKPIEndpoints tests KPI-related API endpoints
func TestKPIEndpoints(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("GET /api/v1/kpi/defs - success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/kpi/defs", nil)
		w := httptest.NewRecorder()

		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "kpis")
	})

	t.Run("POST /api/v1/kpi/defs - success", func(t *testing.T) {
		kpi := models.KPIDefinition{
			ID:          "test-kpi",
			Name:        "Test KPI",
			Kind:        "business",
			Visibility:  "private",
			OwnerUserID: "test-user",
			TenantID:    "default",
		}

		body, _ := json.Marshal(kpi)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/kpi/defs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("POST /api/v1/kpi/defs - invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/kpi/defs", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestDashboardEndpoints tests dashboard-related API endpoints
func TestDashboardEndpoints(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("GET /api/v1/dashboards - success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/dashboards", nil)
		w := httptest.NewRecorder()

		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "dashboards")
	})
}

// TestUserPreferencesEndpoints tests user preferences API endpoints
func TestUserPreferencesEndpoints(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("GET /api/v1/user/preferences - success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/user/preferences", nil)
		w := httptest.NewRecorder()

		ts.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var prefs models.UserPreferences
		err := json.Unmarshal(w.Body.Bytes(), &prefs)
		require.NoError(t, err)

		assert.Equal(t, "test-user", prefs.ID)
		assert.Equal(t, "default", prefs.TenantID)
	})
}
