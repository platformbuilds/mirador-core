package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
) // KPIRepo extends SchemaStore with KPI-specific operations
// KPIRepo interface is defined in the repo package

// KPIHandler provides API endpoints for KPI definitions, layouts, dashboards, and user preferences.
// This handler implements the separate KPI APIs as defined in the API contract.
type KPIHandler struct {
	repo   repo.KPIRepo
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewKPIHandler creates a new KPI handler
func NewKPIHandler(r repo.SchemaStore, cache cache.ValkeyCluster, l logger.Logger) *KPIHandler {
	kpiRepo, ok := r.(repo.KPIRepo)
	if !ok {
		l.Error("SchemaStore does not implement KPIRepo interface - KPI functionality will not be available")
		return nil
	}
	return &KPIHandler{
		repo:   kpiRepo,
		cache:  cache,
		logger: l,
	}
} // ------------------- KPI Definitions API -------------------

// GetKPIDefinitions retrieves all KPI definitions with optional filtering
func (h *KPIHandler) GetKPIDefinitions(c *gin.Context) {
	var req models.KPIListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	if req.TenantID == "" {
		req.TenantID = c.GetString("tenant_id")
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	kpis, total, err := h.listKPIs(req.TenantID, req.Tags, req.Limit, req.Offset)
	if err != nil {
		h.logger.Error("KPI list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list KPIs"})
		return
	}

	nextOffset := req.Offset + len(kpis)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.KPIListResponse{
		KPIDefinitions: kpis,
		Total:          total,
		NextOffset:     nextOffset,
	})
}

// CreateOrUpdateKPIDefinition creates or updates a KPI definition
func (h *KPIHandler) CreateOrUpdateKPIDefinition(c *gin.Context) {
	var req models.KPIDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.KPIDefinition == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kpi definition is required"})
		return
	}

	kpi := req.KPIDefinition
	if kpi.TenantID == "" {
		kpi.TenantID = c.GetString("tenant_id")
	}

	// Validate sentiment field
	if kpi.Sentiment != "" && kpi.Sentiment != "NEGATIVE" && kpi.Sentiment != "POSITIVE" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sentiment must be either 'NEGATIVE' or 'POSITIVE'"})
		return
	}

	if kpi.ID == "" {
		kpi.ID = uuid.New().String()
	}

	kpi.UpdatedAt = time.Now()
	if kpi.CreatedAt.IsZero() {
		kpi.CreatedAt = kpi.UpdatedAt
	}

	err := h.upsertKPI(kpi)
	if err != nil {
		h.logger.Error("KPI upsert failed", "error", err, "id", kpi.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert KPI"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": kpi.ID})
}

// DeleteKPIDefinition deletes a KPI definition by ID
func (h *KPIHandler) DeleteKPIDefinition(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "KPI id is required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	err := h.deleteKPI(tenantID, id)
	if err != nil {
		h.logger.Error("KPI delete failed", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete KPI"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ------------------- KPI Layouts API -------------------

// GetKPILayouts retrieves layouts for a specific dashboard
func (h *KPIHandler) GetKPILayouts(c *gin.Context) {
	dashboardID := c.Query("dashboard")
	if dashboardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboard parameter is required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	layouts, err := h.getKPILayoutsForDashboard(tenantID, dashboardID)
	if err != nil {
		h.logger.Error("KPI layouts get failed", "error", err, "dashboard", dashboardID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get KPI layouts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"layouts": layouts})
}

// BatchUpdateKPILayouts updates multiple KPI layouts for a dashboard
func (h *KPIHandler) BatchUpdateKPILayouts(c *gin.Context) {
	var req struct {
		DashboardID string                 `json:"dashboardId"`
		Layouts     map[string]interface{} `json:"layouts"` // map[kpiId] -> layout
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.DashboardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboardId is required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	err := h.batchUpsertKPILayouts(tenantID, req.DashboardID, req.Layouts)
	if err != nil {
		h.logger.Error("KPI layouts batch update failed", "error", err, "dashboard", req.DashboardID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update KPI layouts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ------------------- Dashboard Management API -------------------

// GetDashboards retrieves all dashboards with optional filtering
func (h *KPIHandler) GetDashboards(c *gin.Context) {
	var req models.DashboardListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	if req.TenantID == "" {
		req.TenantID = c.GetString("tenant_id")
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	dashboards, total, err := h.listDashboards(req.TenantID, req.Limit, req.Offset)
	if err != nil {
		h.logger.Error("dashboard list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list dashboards"})
		return
	}

	nextOffset := req.Offset + len(dashboards)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.DashboardListResponse{
		Dashboards: dashboards,
		Total:      total,
		NextOffset: nextOffset,
	})
} // CreateDashboard creates a new dashboard
func (h *KPIHandler) CreateDashboard(c *gin.Context) {
	var req models.DashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.Dashboard == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboard is required"})
		return
	}

	dashboard := req.Dashboard
	if dashboard.TenantID == "" {
		dashboard.TenantID = c.GetString("tenant_id")
	}

	if dashboard.ID == "" {
		dashboard.ID = uuid.New().String()
	}

	dashboard.CreatedAt = time.Now()
	dashboard.UpdatedAt = dashboard.CreatedAt

	err := h.upsertDashboard(dashboard)
	if err != nil {
		h.logger.Error("dashboard create failed", "error", err, "id", dashboard.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create dashboard"})
		return
	}

	c.JSON(http.StatusCreated, models.DashboardResponse{Dashboard: dashboard})
}

// UpdateDashboard updates an existing dashboard
func (h *KPIHandler) UpdateDashboard(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboard id is required"})
		return
	}

	var req models.DashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.Dashboard == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboard is required"})
		return
	}

	// Get existing dashboard
	tenantID := c.GetString("tenant_id")
	existing, err := h.getDashboard(tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dashboard not found"})
		return
	}

	// Update fields
	existing.Name = req.Dashboard.Name
	existing.Visibility = req.Dashboard.Visibility
	existing.UpdatedAt = time.Now()

	err = h.upsertDashboard(existing)
	if err != nil {
		h.logger.Error("dashboard update failed", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update dashboard"})
		return
	}

	c.JSON(http.StatusOK, models.DashboardResponse{Dashboard: existing})
}

// DeleteDashboard deletes a dashboard by ID
func (h *KPIHandler) DeleteDashboard(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboard id is required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	// Check if it's the default dashboard
	if id == "default" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete default dashboard"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	err := h.deleteDashboard(tenantID, id)
	if err != nil {
		h.logger.Error("dashboard delete failed", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete dashboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ------------------- Implementation methods (extracted from unified handler) -------------------

func (h *KPIHandler) upsertKPI(kpi *models.KPIDefinition) error {
	return h.repo.UpsertKPI(kpi)
}

func (h *KPIHandler) getKPI(tenantID, id string) (*models.KPIDefinition, error) {
	return h.repo.GetKPI(tenantID, id)
}

func (h *KPIHandler) listKPIs(tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {
	return h.repo.ListKPIs(tenantID, tags, limit, offset)
}

func (h *KPIHandler) deleteKPI(tenantID, id string) error {
	return h.repo.DeleteKPI(tenantID, id)
}

func (h *KPIHandler) getKPILayoutsForDashboard(tenantID, dashboardID string) (map[string]interface{}, error) {
	return h.repo.GetKPILayoutsForDashboard(tenantID, dashboardID)
}

func (h *KPIHandler) batchUpsertKPILayouts(tenantID, dashboardID string, layouts map[string]interface{}) error {
	return h.repo.BatchUpsertKPILayouts(tenantID, dashboardID, layouts)
}

func (h *KPIHandler) upsertDashboard(dashboard *models.Dashboard) error {
	return h.repo.UpsertDashboard(dashboard)
}

func (h *KPIHandler) getDashboard(tenantID, id string) (*models.Dashboard, error) {
	return h.repo.GetDashboard(tenantID, id)
}

func (h *KPIHandler) listDashboards(tenantID string, limit, offset int) ([]*models.Dashboard, int, error) {
	return h.repo.ListDashboards(tenantID, limit, offset)
}

func (h *KPIHandler) deleteDashboard(tenantID, id string) error {
	return h.repo.DeleteDashboard(tenantID, id)
}
