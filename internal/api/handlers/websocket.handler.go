package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/pkg/logger"
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

	// Real-time metrics streaming loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Get latest metrics from Valley cluster cache
			metrics, err := h.getLatestMetrics(client.tenantID)
			if err != nil {
				h.logger.Error("Failed to get latest metrics", "error", err)
				continue
			}

			// Send to client
			if err := conn.WriteJSON(map[string]interface{}{
				"type":      "metrics_update",
				"data":      metrics,
				"timestamp": time.Now().Format(time.RFC3339),
			}); err != nil {
				h.logger.Error("WebSocket write failed", "clientId", clientID, "error", err)
				return
			}

		case <-c.Request.Context().Done():
			return
		}
	}
}

// HandleAlertsStream - WebSocket endpoint for real-time alerts
func (h *WebSocketHandler) HandleAlertsStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
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

	// Listen for incoming alert events from ALERT-ENGINE
	h.streamAlerts(c.Request.Context(), client)
}

// HandlePredictionsStream - WebSocket endpoint for real-time AI predictions
func (h *WebSocketHandler) HandlePredictionsStream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
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

	// Stream AI predictions in real-time
	h.streamPredictions(c.Request.Context(), client)
}

// BroadcastAlert sends alert to all connected WebSocket clients
func (h *WebSocketHandler) BroadcastAlert(alert *models.Alert) {
	message := map[string]interface{}{
		"type":      "alert",
		"data":      alert,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	for clientID, client := range h.clients {
		if client.tenantID == alert.TenantID && contains(client.streams, "alerts") {
			if err := client.conn.WriteJSON(message); err != nil {
				h.logger.Error("Failed to broadcast alert", "clientId", clientID, "error", err)
				delete(h.clients, clientID)
			}
		}
	}
}
