package handlers

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	miraorch "github.com/platformbuilds/mirador-core/internal/mira/orchestrator"
	mirasess "github.com/platformbuilds/mirador-core/internal/mira/session"
)

func TestMiraAskHandler_NonStream(t *testing.T) {
	reqBody := map[string]interface{}{
		"message": "How is checkout doing?",
		"options": map[string]bool{"stream": false},
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/mira/ask", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	router := gin.New()
	store := mirasess.NewStore(30 * time.Minute)
	orch := miraorch.New()
	h := NewMiraHandler(store, orch)
	router.POST("/api/v1/mira/ask", h.MiraAsk)
	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200; got %d", res.StatusCode)
	}
	body, _ := ioutil.ReadAll(res.Body)
	if !strings.Contains(string(body), "conversationId") {
		t.Fatalf("response body missing conversationId: %s", string(body))
	}
	if !strings.Contains(string(body), "capabilityId") {
		t.Fatalf("response body missing capabilityId: %s", string(body))
	}
}

func TestMiraAskHandler_Stream(t *testing.T) {
	reqBody := map[string]interface{}{
		"message": "Is anything failing right now?",
		"options": map[string]bool{"stream": true},
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/mira/ask", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	router := gin.New()
	store := mirasess.NewStore(30 * time.Minute)
	orch := miraorch.New()
	h := NewMiraHandler(store, orch)
	router.POST("/api/v1/mira/ask", h.MiraAsk)
	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200; got %d", res.StatusCode)
	}
	body, _ := ioutil.ReadAll(res.Body)
	out := string(body)
	// Expect SSE event markers (simple JSON-based events encoded by our handler)
	if !strings.Contains(out, "mira.text") {
		t.Fatalf("stream output missing mira.text event: %s", out)
	}
	if !strings.Contains(out, "mira.card") {
		t.Fatalf("stream output missing mira.card event: %s", out)
	}
	if !strings.Contains(out, "mira.end") {
		t.Fatalf("stream output missing mira.end event: %s", out)
	}
}
