package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Ensure strict payload mode rejects extra fields for the correlation endpoint
func TestHandleUnifiedCorrelation_StrictRejectsExtraFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(nil)

	// Build handler with strict payload enabled
	handler := &UnifiedQueryHandler{
		unifiedEngine: &mockUnifiedEngine{},
		logger:        mockLogger,
		kpiRepo:       nil,
		engineCfg:     config.EngineConfig{StrictTimeWindowPayload: true},
	}

	now := time.Now().UTC()
	// canonical fields plus an unexpected extra field
	payload := map[string]interface{}{
		"startTime":  now.Add(-10 * time.Minute).Format(time.RFC3339),
		"endTime":    now.Format(time.RFC3339),
		"unexpected": "value",
	}
	body, _ := json.Marshal(payload)

	c.Request = httptest.NewRequest("POST", "/api/v1/unified/correlation", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleUnifiedCorrelation(c)

	if w.Code != 400 {
		t.Fatalf("expected 400 Bad Request for extra fields in strict mode, got %d; body=%s", w.Code, w.Body.String())
	}
}
