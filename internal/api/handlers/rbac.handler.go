package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type RBACHandler struct {
	cache  cache.ValkeyCluster
	logger logger.Logger
}

func NewRBACHandler(c cache.ValkeyCluster, l logger.Logger) *RBACHandler {
	return &RBACHandler{cache: c, logger: l}
}

// GET /api/v1/rbac/roles
// Minimal: return a static starter set. Later: hydrate from Valkey keys rbac:roles:<tenant>.
func (h *RBACHandler) GetRoles(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	_ = tenantID // reserved for Valkey lookups

	roles := []gin.H{
		{"name": "admin", "permissions": []string{"*"}, "description": "Full access"},
		{"name": "ops", "permissions": []string{"dash.view", "logs.query", "traces.query"}, "description": "Operational read/query"},
		{"name": "viewer", "permissions": []string{"dash.view"}, "description": "Read-only dashboards"},
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"roles": roles, "total": len(roles)}})
}

// POST /api/v1/rbac/roles
// Body: { "name": "...", "permissions": ["..."], "description": "..." }
func (h *RBACHandler) CreateRole(c *gin.Context) {
	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		Description string   `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid role payload"})
		return
	}
	// TODO: persist into Valkey rbac:role:<tenant>:<name> and add to rbac:roles:<tenant>
	// Skipping persistence for now — API shape is stable.
	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"role":      gin.H{"name": req.Name, "permissions": req.Permissions, "description": req.Description},
			"createdAt": time.Now().Format(time.RFC3339),
		},
	})
}

// PUT /api/v1/rbac/users/:userId/roles
// Body: { "roles": ["viewer","ops"] }  -> overlay stored in Valkey (later)
func (h *RBACHandler) AssignUserRoles(c *gin.Context) {
	userID := c.Param("userId")
	var req struct {
		Roles []string `json:"roles"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Roles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid roles payload"})
		return
	}
	// TODO: persist overlay to Valkey rbac:user_roles:<tenant>:<userID>
	h.logger.Info("Assigned user roles", "userId", userID, "roles", req.Roles)
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"userId": userID, "roles": req.Roles}})
}
