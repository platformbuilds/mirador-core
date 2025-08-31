package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
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

// ---- Hub main loop ---------------------------------------------------------

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

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
					// slow consumer; drop and unregister
					delete(h.clients, client)
					close(client.send)
					for stream := range client.streams {
						metrics.ActiveWebSocketConnections.WithLabelValues(stream, client.tenantID).Dec()
					}
				}
			}
			h.mu.RUnlock()

		case <-ctx.Done():
			return
		}
	}
}

// ---- Public APIs -----------------------------------------------------------

// ServeWS upgrades an HTTP request to a WebSocket and registers a client.
//
// Query params (optional):
//   - streams: comma-separated list (e.g. "metrics,alerts")
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, upgrader websocket.Upgrader, tenantID, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	streams := parseStreams(r.URL.Query().Get("streams"))
	if len(streams) == 0 {
		// default to all known streams if none requested explicitly
		streams = map[string]bool{"metrics": true, "alerts": true, "predictions": true, "correlations": true}
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256), // bounded buffer for backpressure
		tenantID: tenantID,
		userID:   firstNonEmpty(userID, randomID()),
		streams:  streams,
	}

	h.register <- client

	// Start pumps
	go client.writePump()
	go client.readPump()
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

// ---- Client pumps (read/write) --------------------------------------------

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 << 10 // 64KiB
)

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			// client closed or error; triggers unregister
			return
		}
		// No-op: if you want client → server messages, handle them here.
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ---- Utilities -------------------------------------------------------------

func getStreamNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	return out
}

func parseStreams(raw string) map[string]bool {
	if strings.TrimSpace(raw) == "" {
		return map[string]bool{}
	}
	res := map[string]bool{}
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			res[s] = true
		}
	}
	return res
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
