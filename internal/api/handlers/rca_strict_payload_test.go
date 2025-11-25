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

// Ensure RCA strict payload mode rejects extra fields
func TestHandleComputeRCA_StrictRejectsExtraFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	mockLogger := logger.NewMockLogger(nil)

	handler := &RCAHandler{
		logger:    mockLogger,
		rcaEngine: &mockRCAEngine{},
		engineCfg: config.EngineConfig{StrictTimeWindowPayload: true},
		// ensure strict flag used by handler constructor path
		strictTimeWindowPayload: true,
	}

	now := time.Now().UTC()
	payload := map[string]interface{}{
		"startTime":  now.Add(-5 * time.Minute).Format(time.RFC3339),
		"endTime":    now.Format(time.RFC3339),
		"extraField": 123,
	}
	body, _ := json.Marshal(payload)

	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)

	if w.Code != 400 {
		t.Fatalf("expected 400 Bad Request for extra fields in strict mode, got %d; body=%s", w.Code, w.Body.String())
	}
}
