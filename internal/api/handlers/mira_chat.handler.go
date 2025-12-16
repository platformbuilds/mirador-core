package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	miraintent "github.com/platformbuilds/mirador-core/internal/mira/intent"
	miraorch "github.com/platformbuilds/mirador-core/internal/mira/orchestrator"
	mirasess "github.com/platformbuilds/mirador-core/internal/mira/session"
	mirasum "github.com/platformbuilds/mirador-core/internal/mira/summariser"
)

// MiraRequest shapes the incoming JSON
type MiraRequest struct {
	ConversationID string                 `json:"conversationId,omitempty"`
	Message        string                 `json:"message"`
	Context        map[string]interface{} `json:"context,omitempty"`
	Options        struct {
		Stream bool `json:"stream,omitempty"`
	} `json:"options,omitempty"`
}

// MiraHandler contains dependencies for mira endpoints.
type MiraHandler struct {
	store mirasess.SessionStore
	orch  *miraorch.Orchestrator
}

// NewMiraHandler constructs a new MiraHandler with store and orchestrator.
func NewMiraHandler(store mirasess.SessionStore, orch *miraorch.Orchestrator) *MiraHandler {
	return &MiraHandler{store: store, orch: orch}
}

// MiraAsk handles POST /api/v1/mira/ask with streaming and JSON mode.
// For Phase-1 this is deterministic and mocked.
func (h *MiraHandler) MiraAsk(c *gin.Context) {
	var req MiraRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Ensure conversation ID exists for session tracking
	if req.ConversationID == "" {
		req.ConversationID = fmt.Sprintf("conv-%d", time.Now().UnixNano())
	}

	// Session management using injected store
	sess := h.store.Ensure(req.ConversationID)
	sess.Scope = "default"
	h.store.Set(req.ConversationID, &sess)

	ir, err := miraintent.DetectIntent(req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "intent detection failed"})
		return
	}
	domain, err := h.orch.HandleIntent(ir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestration failed"})
		return
	}

	if req.Options.Stream {
		// SSE using Gin's writer
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(c.Writer)
		// text parts
		for _, p := range mirasum.GenerateTextParts(domain) {
			enc.Encode(map[string]interface{}{"event": "mira.text", "data": p})
			if f, ok := c.Writer.(interface{ Flush() }); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
		// card
		if card, err := mirasum.CardForDomain(domain); err == nil {
			enc.Encode(map[string]interface{}{"event": "mira.card", "data": card})
		}
		enc.Encode(map[string]interface{}{"event": "mira.end"})
		return
	}

	// Non-streaming: return assembled JSON
	c.JSON(http.StatusOK, gin.H{"conversationId": req.ConversationID, "capabilityId": string(ir.CapabilityID), "result": domain})
}
