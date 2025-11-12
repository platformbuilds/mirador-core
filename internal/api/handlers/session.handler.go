package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type SessionHandler struct {
	cache  cache.ValkeyCluster
	logger logger.Logger
}

func NewSessionHandler(c cache.ValkeyCluster, l logger.Logger) *SessionHandler {
	return &SessionHandler{cache: c, logger: l}
}

// GET /api/v1/sessions/active
// For now returns the current caller's session (from Authorization header).
// Later: expand to list all tenant sessions via Valkey index.
func (h *SessionHandler) GetActiveSessions(c *gin.Context) {
	sessionID := c.GetString("session_id")
	if sessionID == "" {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"sessions": []interface{}{}, "total": 0}})
		return
	}
	sess, err := h.cache.GetSession(c.Request.Context(), sessionID)
	if err != nil || sess == nil {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"sessions": []interface{}{}, "total": 0}})
		return
	}
	// touch last activity – optional; only if your cache supports SetSession on read
	sess.LastActivity = time.Now()
	_ = h.cache.SetSession(c.Request.Context(), sess)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"sessions": []interface{}{sess},
			"total":    1,
		},
	})
}

// POST /api/v1/sessions/invalidate
// Body: { "token": "..." }  (falls back to Authorization header)
func (h *SessionHandler) InvalidateSession(c *gin.Context) {
	var body struct {
		Token string `json:"token"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Token == "" {
		body.Token = c.GetString("session_id")
	}
	if body.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "missing token"})
		return
	}

	// If your cache has a dedicated delete, use it; otherwise overwrite with short TTL.
	if del, ok := interface{}(h.cache).(interface {
		DeleteSession(ctx interface{}, token string) error
	}); ok {
		_ = del.DeleteSession(c.Request.Context(), body.Token)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"invalidated": true},
	})
}

// GET /api/v1/sessions/user/:userId
// Minimal: if the current token belongs to that user, return it.
// Later: list all tokens for user via sess_index:<tenant>:<user>.
func (h *SessionHandler) GetUserSessions(c *gin.Context) {
	userID := c.Param("userId")
	sessionID := c.GetString("session_id")
	if sessionID == "" {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"sessions": []interface{}{}, "total": 0}})
		return
	}
	sess, err := h.cache.GetSession(c.Request.Context(), sessionID)
	if err != nil || sess == nil || sess.UserID != userID {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"sessions": []interface{}{}, "total": 0}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"sessions": []interface{}{sess},
			"total":    1,
		},
	})
}
