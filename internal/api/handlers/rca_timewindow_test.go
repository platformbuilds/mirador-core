package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestHandleComputeRCA_TimeWindowOnly_Valid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var buf strings.Builder
	mockLogger := logger.NewMockLogger(&buf)
	mockRCA := &mockRCAEngine{score: 0.6}

	handler := &RCAHandler{
		logger:    mockLogger,
		rcaEngine: mockRCA,
	}

	now := time.Now().UTC()
	tw := models.TimeWindowRequest{StartTime: now.Add(-20 * time.Minute).Format(time.RFC3339), EndTime: now.Format(time.RFC3339)}
	body, _ := json.Marshal(tw)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 OK, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleComputeRCA_TimeWindowOnly_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var buf strings.Builder
	mockLogger := logger.NewMockLogger(&buf)
	handler := &RCAHandler{logger: mockLogger, rcaEngine: &mockRCAEngine{}}

	// invalid time strings
	tw := models.TimeWindowRequest{StartTime: "not-a-date", EndTime: "also-not-a-date"}
	body, _ := json.Marshal(tw)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)

	if w.Code != 400 {
		t.Fatalf("expected 400 Bad Request for invalid time format, got %d; body=%s", w.Code, w.Body.String())
	}
}
