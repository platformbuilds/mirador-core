package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type WebSocketHandler struct {
	upgrader websocket.Upgrader
	logger   logger.Logger
	clients  map[string]*WebSocketClient
}

type WebSocketClient struct {
	conn     *websocket.Conn
	tenantID string
	userID   string
	streams  []string // metrics, alerts, predictions
}

func NewWebSocketHandler(logger logger.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			// TODO: tighten in prod (check Origin/Host, tenant, auth)
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		logger:  logger,
		clients: make(map[string]*WebSocketClient),
	}
}

// HandleMetricsStream - WebSocket endpoint for real-time metrics
func (h *WebSocketHandler) HandleMetricsStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	clientID := generateClientID()
	client := &WebSocketClient{
		conn:     conn,
		tenantID: c.GetString("tenant_id"),
		userID:   c.GetString("user_id"),
		streams:  []string{"metrics"},
	}
	h.clients[clientID] = client
	defer delete(h.clients, clientID)

	h.logger.Info("WebSocket client connected", "clientId", clientID, "stream", "metrics")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// basic heartbeat so idle proxies don't drop us
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ticker.C:
			metrics, err := h.getLatestMetrics(client.tenantID)
			if err != nil {
				h.logger.Error("Failed to get latest metrics", "error", err)
				continue
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteJSON(map[string]interface{}{
				"type":      "metrics_update",
				"data":      metrics,
				"timestamp": time.Now().Format(time.RFC3339),
			}); err != nil {
				h.logger.Error("WebSocket write failed", "clientId", clientID, "error", err)
				return
			}

		case <-heartbeat.C:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_ = conn.WriteJSON(map[string]interface{}{
				"type": "heartbeat",
				"data": map[string]any{"ts": time.Now().UnixMilli()},
			})

		case <-c.Request.Context().Done():
			return
		}
	}
}

// HandleAlertsStream - WebSocket endpoint for real-time alerts
func (h *WebSocketHandler) HandleAlertsStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed (alerts)", "error", err)
		return
	}
	defer conn.Close()

	clientID := generateClientID()
	client := &WebSocketClient{
		conn:     conn,
		tenantID: c.GetString("tenant_id"),
		userID:   c.GetString("user_id"),
		streams:  []string{"alerts"},
	}
	h.clients[clientID] = client
	defer delete(h.clients, clientID)

	h.streamAlerts(c.Request.Context(), client)
}

// HandlePredictionsStream - WebSocket endpoint for real-time AI predictions
func (h *WebSocketHandler) HandlePredictionsStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed (predictions)", "error", err)
		return
	}
	defer conn.Close()

	clientID := generateClientID()
	client := &WebSocketClient{
		conn:     conn,
		tenantID: c.GetString("tenant_id"),
		userID:   c.GetString("user_id"),
		streams:  []string{"predictions"},
	}
	h.clients[clientID] = client
	defer delete(h.clients, clientID)

	h.streamPredictions(c.Request.Context(), client)
}

// BroadcastAlert sends an alert to all connected WebSocket clients
func (h *WebSocketHandler) BroadcastAlert(alert *models.Alert) {
	message := map[string]interface{}{
		"type":      "alert",
		"data":      alert,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	for clientID, client := range h.clients {
		if client.tenantID == alert.TenantID && contains(client.streams, "alerts") {
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteJSON(message); err != nil {
				h.logger.Error("Failed to broadcast alert", "clientId", clientID, "error", err)
				delete(h.clients, clientID)
			}
		}
	}
}

// ======== helpers / placeholders ========

// generateClientID returns a random 16-byte hex id.
func generateClientID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// getLatestMetrics currently returns a placeholder structure.
// Wire this to Valkey (or another source) later.
func (h *WebSocketHandler) getLatestMetrics(tenantID string) (interface{}, error) {
	return map[string]any{
		"tenantId": tenantID,
		"series":   []any{}, // fill with real data points
		"ts":       time.Now().UnixMilli(),
	}, nil
}

// streamAlerts keeps the connection alive with heartbeats until ctx is done.
// Replace this with a real subscription to your ALERT-ENGINE.
func (h *WebSocketHandler) streamAlerts(ctx context.Context, client *WebSocketClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_ = client.conn.WriteJSON(map[string]any{
				"type": "heartbeat",
				"data": map[string]any{"ts": time.Now().UnixMilli(), "stream": "alerts"},
			})
		case <-ctx.Done():
			return
		}
	}
}

// streamPredictions keeps the connection alive with heartbeats until ctx is done.
// Replace with a real feed from PREDICT-ENGINE when available.
func (h *WebSocketHandler) streamPredictions(ctx context.Context, client *WebSocketClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_ = client.conn.WriteJSON(map[string]any{
				"type": "heartbeat",
				"data": map[string]any{"ts": time.Now().UnixMilli(), "stream": "predictions"},
			})
		case <-ctx.Done():
			return
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
