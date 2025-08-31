package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/platformbuilds/miradorstack/internal/metrics"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	logger     logger.Logger
	mu         sync.RWMutex
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	tenantID string
	userID   string
	streams  map[string]bool // metrics, alerts, predictions, correlations
}

type Message struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	TenantID  string      `json:"tenant_id,omitempty"`
}

func NewHub(logger logger.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
		logger:     logger,
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			
			// Update metrics
			for stream := range client.streams {
				metrics.ActiveWebSocketConnections.WithLabelValues(stream, client.tenantID).Inc()
			}
			
			h.logger.Info("WebSocket client connected", 
				"clientId", client.userID, 
				"tenant", client.tenantID,
				"streams", getStreamNames(client.streams),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				
				// Update metrics
				for stream := range client.streams {
					metrics.ActiveWebSocketConnections.WithLabelValues(stream, client.tenantID).Dec()
				}
			}
			h.mu.Unlock()
			
			h.logger.Info("WebSocket client disconnected", "clientId", client.userID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}
			h.mu.RUnlock()

		case <-ctx.Done():
			return
		}
	}
}

// BroadcastAlert sends alerts to subscribed clients
func (h *Hub) BroadcastAlert(alert *models.Alert) {
	message := Message{
		Type:      "alert",
		Data:      alert,
		Timestamp: time.Now(),
		TenantID:  alert.TenantID,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal alert message", "alertId", alert.ID, "error", err)
		return
	}

	h.mu.RLock()
	for client := range h.clients {
		if client.tenantID == alert.TenantID && client.streams["alerts"] {
			select {
			case client.send <- messageBytes:
			default:
				// Client send buffer is full, disconnect
				delete(h.clients, client)
				close(client.send)
			}
		}
	}
	h.mu.RUnlock()
}

// BroadcastPrediction sends AI predictions to subscribed clients
func (h *Hub) BroadcastPrediction(prediction *models.SystemFracture) {
	message := Message{
		Type:      "prediction",
		Data:      prediction,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal prediction message", "predictionId", prediction.ID, "error", err)
		return
	}

	h.mu.RLock()
	for client := range h.clients {
		if client.streams["predictions"] {
			select {
			case client.send <- messageBytes:
			default:
				delete(h.clients, client)
				close(client.send)
			}
		}
	}
	h.mu.RUnlock()
}
