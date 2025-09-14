package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type SchemaHandler struct {
	repo           repo.SchemaStore
	metricsService *services.VictoriaMetricsService
	logsService    *services.VictoriaLogsService
	cache          cache.ValkeyCluster
	logger         logger.Logger
	maxUploadBytes int64
}

func NewSchemaHandler(r repo.SchemaStore, ms *services.VictoriaMetricsService, ls *services.VictoriaLogsService, cache cache.ValkeyCluster, l logger.Logger, maxUploadBytes int64) *SchemaHandler {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 5 << 20
	}
	return &SchemaHandler{repo: r, metricsService: ms, logsService: ls, cache: cache, logger: l, maxUploadBytes: maxUploadBytes}
}

type upsertMetricReq struct {
	TenantID    string   `json:"tenantId"`
	Metric      string   `json:"metric"`
	Description string   `json:"description"`
	Owner       string   `json:"owner"`
	Tags        []string `json:"tags"` // Changed from map[string]interface{} to map[string]string
	Author      string   `json:"author"`
}

// Helper function to safely parse JSON tags from CSV and convert to map[string]any
// parseTagsJSONToSlice accepts JSON array of strings (preferred) or a JSON object (BC).
// - If it's an array: ["prod","hydnar"] -> []string{"prod","hydnar"}
// - If it's an object: {"env":"prod","site":"hydnar"} -> []string{"env=prod","site=hydnar"}
func parseTagsJSONToSlice(tagsJSON string) []string {
	if tagsJSON == "" {
		return nil
	}

	// Preferred format: []string
	var arr []string
	if err := json.Unmarshal([]byte(tagsJSON), &arr); err == nil {
		// sanitize non-string elements if any leaked in
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			out = append(out, v)
		}
		return out
	}

	// Back-compat: map[string]any -> "k=v"
	var obj map[string]any
	if err := json.Unmarshal([]byte(tagsJSON), &obj); err == nil {
		out := make([]string, 0, len(obj))
		for k, v := range obj {
			if s, ok := v.(string); ok {
				out = append(out, k+"="+s)
			} else {
				out = append(out, k+"="+fmt.Sprintf("%v", v))
			}
		}
		return out
	}

	// Back-compat (very lenient): []any -> stringify
	var anyArr []any
	if err := json.Unmarshal([]byte(tagsJSON), &anyArr); err == nil {
		out := make([]string, 0, len(anyArr))
		for _, v := range anyArr {
			out = append(out, fmt.Sprintf("%v", v))
		}
		return out
	}

	return nil
}

func parseJSONToMap(s string) map[string]any {
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err == nil {
		return m
	}
	return nil
}

func (h *SchemaHandler) UpsertMetric(c *gin.Context) {
	var req upsertMetricReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Metric == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = c.GetString("tenant_id")
	}

	// Convert map[string]string to map[string]any for repository
	metric := repo.MetricDef{
		TenantID:    req.TenantID,
		Metric:      req.Metric,
		Description: req.Description,
		Owner:       req.Owner,
		Tags:        req.Tags,
	}
	if err := h.repo.UpsertMetric(c.Request.Context(), metric, req.Author); err != nil {
		h.logger.Error("metric upsert failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upsert failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *SchemaHandler) GetMetric(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	metric := c.Param("metric")
	m, err := h.repo.GetMetric(c.Request.Context(), tenantID, metric)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *SchemaHandler) ListMetricVersions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	metric := c.Param("metric")
	out, err := h.repo.ListMetricVersions(c.Request.Context(), tenantID, metric)
	if err != nil {
		h.logger.Error("list metric versions failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list versions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": out})
}

func (h *SchemaHandler) GetMetricVersion(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	metric := c.Param("metric")
	verStr := c.Param("version")
	v, err := strconv.ParseInt(verStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version"})
		return
	}
	payload, info, err := h.repo.GetMetricVersion(c.Request.Context(), tenantID, metric, v)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": info.Version, "author": info.Author, "created_at": info.CreatedAt, "payload": payload})
}

type upsertLogFieldReq struct {
	TenantID    string   `json:"tenantId"`
	Field       string   `json:"field"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples"`
	Author      string   `json:"author"`
}

func (h *SchemaHandler) UpsertLogField(c *gin.Context) {
	var req upsertLogFieldReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Field == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = c.GetString("tenant_id")
	}

	// arrays -> as-is for tags
	tags := make([]string, 0, len(req.Tags))
	for _, s := range req.Tags {
		if s != "" {
			tags = append(tags, s)
		}
	}

	// examples []string -> map[string]any {"0": "...", "1": "..."}
	examples := make(map[string]any, len(req.Examples))
	for i, v := range req.Examples {
		examples[strconv.Itoa(i)] = v
	}

	field := repo.LogFieldDef{
		TenantID:    req.TenantID,
		Field:       req.Field,
		Type:        req.Type,
		Description: req.Description,
		Tags:        tags,
		Examples:    examples,
	}
	if err := h.repo.UpsertLogField(c.Request.Context(), field, req.Author); err != nil {
		h.logger.Error("upsert log field failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upsert failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *SchemaHandler) GetLogField(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	field := c.Param("field")
	f, err := h.repo.GetLogField(c.Request.Context(), tenantID, field)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, f)
}

func (h *SchemaHandler) ListLogFieldVersions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	field := c.Param("field")
	out, err := h.repo.ListLogFieldVersions(c.Request.Context(), tenantID, field)
	if err != nil {
		h.logger.Error("list log field versions failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list versions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": out})
}

func (h *SchemaHandler) GetLogFieldVersion(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	field := c.Param("field")
	verStr := c.Param("version")
	v, err := strconv.ParseInt(verStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version"})
		return
	}
	payload, info, err := h.repo.GetLogFieldVersion(c.Request.Context(), tenantID, field, v)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": info.Version, "author": info.Author, "created_at": info.CreatedAt, "payload": payload})
}

// ---------- Traces: Services and Operations (CRUD + Versions) ----------

type upsertTraceServiceReq struct {
	TenantID string   `json:"tenantId"`
	Service  string   `json:"service"`
	Purpose  string   `json:"purpose"`
	Owner    string   `json:"owner"`
	Tags     []string `json:"tags"`
	Author   string   `json:"author"`
}

func (h *SchemaHandler) UpsertTraceService(c *gin.Context) {
	var req upsertTraceServiceReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Service == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = c.GetString("tenant_id")
	}

	if err := h.repo.UpsertTraceServiceWithAuthor(c.Request.Context(), req.TenantID, req.Service, req.Purpose, req.Owner, req.Tags, req.Author); err != nil {
		h.logger.Error("trace service upsert failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upsert failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *SchemaHandler) GetTraceService(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	service := c.Param("service")
	s, err := h.repo.GetTraceService(c.Request.Context(), tenantID, service)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *SchemaHandler) ListTraceServiceVersions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	service := c.Param("service")
	out, err := h.repo.ListTraceServiceVersions(c.Request.Context(), tenantID, service)
	if err != nil {
		h.logger.Error("list trace service versions failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list versions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": out})
}

func (h *SchemaHandler) GetTraceServiceVersion(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	service := c.Param("service")
	verStr := c.Param("version")
	v, err := strconv.ParseInt(verStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version"})
		return
	}
	payload, info, err := h.repo.GetTraceServiceVersion(c.Request.Context(), tenantID, service, v)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": info.Version, "author": info.Author, "created_at": info.CreatedAt, "payload": payload})
}

type upsertTraceOperationReq struct {
	TenantID  string   `json:"tenantId"`
	Service   string   `json:"service"`
	Operation string   `json:"operation"`
	Purpose   string   `json:"purpose"`
	Owner     string   `json:"owner"`
	Tags      []string `json:"tags"`
	Author    string   `json:"author"`
}

func (h *SchemaHandler) UpsertTraceOperation(c *gin.Context) {
	var req upsertTraceOperationReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Service == "" || req.Operation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = c.GetString("tenant_id")
	}

	if err := h.repo.UpsertTraceOperationWithAuthor(c.Request.Context(), req.TenantID, req.Service, req.Operation, req.Purpose, req.Owner, req.Tags, req.Author); err != nil {
		h.logger.Error("trace operation upsert failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upsert failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *SchemaHandler) GetTraceOperation(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	service := c.Param("service")
	operation := c.Param("operation")
	o, err := h.repo.GetTraceOperation(c.Request.Context(), tenantID, service, operation)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, o)
}

func (h *SchemaHandler) ListTraceOperationVersions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	service := c.Param("service")
	operation := c.Param("operation")
	out, err := h.repo.ListTraceOperationVersions(c.Request.Context(), tenantID, service, operation)
	if err != nil {
		h.logger.Error("list trace operation versions failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list versions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": out})
}

func (h *SchemaHandler) GetTraceOperationVersion(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	service := c.Param("service")
	operation := c.Param("operation")
	verStr := c.Param("version")
	v, err := strconv.ParseInt(verStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version"})
		return
	}
	payload, info, err := h.repo.GetTraceOperationVersion(c.Request.Context(), tenantID, service, operation, v)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": info.Version, "author": info.Author, "created_at": info.CreatedAt, "payload": payload})
}

// BulkUpsertTraceServicesCSV ingests trace service definitions via CSV with strict headers.
// Columns: tenant_id, service (required), purpose, owner, tags_json, author
func (h *SchemaHandler) BulkUpsertTraceServicesCSV(c *gin.Context) {
	if limited := h.enforceQuota(c, "traces_services", 20); limited {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)
	if err := c.Request.ParseMultipartForm(6 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form or file too large"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	allowedCT := map[string]struct{}{"text/csv": {}, "application/vnd.ms-excel": {}, "text/plain": {}}
	if ct := header.Header.Get("Content-Type"); ct != "" {
		if _, ok := allowedCT[strings.ToLower(ct)]; !ok { /* sniff below */
		}
	}
	var sniff [512]byte
	n, _ := file.Read(sniff[:])
	if det := http.DetectContentType(sniff[:n]); det != "application/octet-stream" {
		if _, ok := allowedCT[det]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported content type"})
			return
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	// Read header
	headerRow, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty csv"})
		return
	}
	// Validate UTF-8 header and max columns
	if len(headerRow) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many columns"})
		return
	}
	for _, col := range headerRow {
		if !utf8.ValidString(col) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid utf-8 in header"})
			return
		}
	}
	idx := make(map[string]int)
	for i, col := range headerRow {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	// Strict header allowlist for trace services
	allowed := map[string]struct{}{"tenant_id": {}, "service": {}, "purpose": {}, "owner": {}, "tags_json": {}, "author": {}}
	for k := range idx {
		if _, ok := allowed[k]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown column: " + k})
			return
		}
	}
	required := []string{"service"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing column: " + col})
			return
		}
	}

	tenantOverride := c.GetString("tenant_id")
	count := 0
	var rowErrs []string

	// Limit rows to prevent abuse
	const maxRows = 10000
	rows := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			rowErrs = append(rowErrs, "read error")
			continue
		}
		rows++
		if rows > maxRows {
			rowErrs = append(rowErrs, "row limit exceeded")
			break
		}
		// Validate UTF-8 and sanitize to mitigate CSV injection
		for i := range rec {
			if !utf8.ValidString(rec[i]) {
				rec[i] = sanitizeCSVCell(rec[i])
			}
			rec[i] = sanitizeCSVCell(rec[i])
		}
		get := func(name string) string {
			if j, ok := idx[name]; ok && j < len(rec) {
				return strings.TrimSpace(rec[j])
			}
			return ""
		}
		tenant := get("tenant_id")
		if tenant == "" {
			tenant = tenantOverride
		}
		service := get("service")
		if service == "" {
			rowErrs = append(rowErrs, "missing service")
			continue
		}
		purpose := get("purpose")
		owner := get("owner")
		tags := get("tags_json")
		author := get("author")

		tagsList := parseTagsJSONToSlice(tags)

		if err := h.repo.UpsertTraceServiceWithAuthor(c.Request.Context(), tenant, service, purpose, owner, tagsList, author); err != nil {
			rowErrs = append(rowErrs, "service upsert failed: "+service)
		} else {
			count++
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "services_upserted": count, "errors": rowErrs, "file": header.Filename})
}

// BulkUpsertTraceOperationsCSV ingests operation definitions; enforces service scoping.
// Columns: tenant_id, service (required), operation (required), purpose, owner, tags_json, author
func (h *SchemaHandler) BulkUpsertTraceOperationsCSV(c *gin.Context) {
	if limited := h.enforceQuota(c, "traces_operations", 20); limited {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)
	if err := c.Request.ParseMultipartForm(6 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form or file too large"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	allowedCT := map[string]struct{}{"text/csv": {}, "application/vnd.ms-excel": {}, "text/plain": {}}
	if ct := header.Header.Get("Content-Type"); ct != "" {
		if _, ok := allowedCT[strings.ToLower(ct)]; !ok { /* sniff below */
		}
	}
	var sniff [512]byte
	n, _ := file.Read(sniff[:])
	if det := http.DetectContentType(sniff[:n]); det != "application/octet-stream" {
		if _, ok := allowedCT[det]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported content type"})
			return
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}
	r := csv.NewReader(file)
	r.TrimLeadingSpace = true
	headerRow, err := r.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty csv"})
		return
	}
	if len(headerRow) > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many columns"})
		return
	}
	for _, col := range headerRow {
		if !utf8.ValidString(col) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid utf-8 in header"})
			return
		}
	}
	idx := map[string]int{}
	for i, col := range headerRow {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	allowed := map[string]struct{}{"tenant_id": {}, "service": {}, "operation": {}, "purpose": {}, "owner": {}, "tags_json": {}, "author": {}}
	for k := range idx {
		if _, ok := allowed[k]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown column: " + k})
			return
		}
	}
	if _, ok := idx["service"]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing column: service"})
		return
	}
	if _, ok := idx["operation"]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing column: operation"})
		return
	}
	tenantOverride := c.GetString("tenant_id")
	count := 0
	var errs []string
	const maxRows = 10000
	rows := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, "read error")
			continue
		}
		rows++
		if rows > maxRows {
			errs = append(errs, "row limit exceeded")
			break
		}
		for i := range rec {
			if !utf8.ValidString(rec[i]) {
				rec[i] = sanitizeCSVCell(rec[i])
			}
			rec[i] = sanitizeCSVCell(rec[i])
		}
		get := func(k string) string {
			if j, ok := idx[k]; ok && j < len(rec) {
				return strings.TrimSpace(rec[j])
			}
			return ""
		}
		tenant := get("tenant_id")
		if tenant == "" {
			tenant = tenantOverride
		}
		service := get("service")
		operation := get("operation")
		if service == "" || operation == "" {
			errs = append(errs, "missing service/operation")
			continue
		}
		// Ensure service exists to maintain sanity that operations are per service
		if _, err := h.repo.GetTraceService(c.Request.Context(), tenant, service); err != nil {
			errs = append(errs, "undefined service: "+service)
			continue
		}
		purpose := get("purpose")
		owner := get("owner")
		tags := get("tags_json")
		author := get("author")

		tagsList := parseTagsJSONToSlice(tags)

		if err := h.repo.UpsertTraceOperationWithAuthor(c.Request.Context(), tenant, service, operation, purpose, owner, tagsList, author); err != nil {
			errs = append(errs, "operation upsert failed: "+service+":"+operation)
		} else {
			count++
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "operations_upserted": count, "errors": errs, "file": header.Filename})
}

// BulkUpsertMetricsCSV ingests metric and label definitions in CSV format.
// Security: MIME check, size limit, in-memory processing only, no disk writes.
// CSV Columns:
// tenant_id, metric, description, owner, tags_json, label, label_type, label_required, label_allowed_json, label_description, author
func (h *SchemaHandler) BulkUpsertMetricsCSV(c *gin.Context) {
	// Per-tenant daily quota (default 20/day)
	if limited := h.enforceQuota(c, "metrics", 20); limited {
		return
	}
	// Limit payload size to 5MiB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)

	// Parse multipart form (limit memory usage)
	if err := c.Request.ParseMultipartForm(6 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form or file too large"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	// Accept common CSV types
	allowedCT := map[string]struct{}{"text/csv": {}, "application/vnd.ms-excel": {}, "text/plain": {}}
	// Check header content type but don't reject yet - some browsers send incorrect MIME types
	// Sniff first 512 bytes
	var sniff [512]byte
	n, _ := file.Read(sniff[:])
	detected := http.DetectContentType(sniff[:n])
	if _, ok := allowedCT[detected]; !ok && detected != "application/octet-stream" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported content type"})
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	headerRow, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty csv"})
		return
	}
	if len(headerRow) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many columns"})
		return
	}
	for _, col := range headerRow {
		if !utf8.ValidString(col) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid utf-8 in header"})
			return
		}
	}
	idx := make(map[string]int)
	for i, col := range headerRow {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	// Strict header allowlist for metrics
	allowed := map[string]struct{}{"tenant_id": {}, "metric": {}, "description": {}, "owner": {}, "tags_json": {}, "label": {}, "label_type": {}, "label_required": {}, "label_allowed_json": {}, "label_description": {}, "author": {}}
	for k := range idx {
		if _, ok := allowed[k]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown column: " + k})
			return
		}
	}
	required := []string{"metric"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing column: " + col})
			return
		}
	}

	tenantOverride := c.GetString("tenant_id")
	// Aggregate metrics to reduce version bumps
	type metricRow struct{ tenant, metric, desc, owner, tags, author string }
	metricsSeen := map[string]metricRow{}
	labelsCount := 0
	metricsCount := 0
	var rowErrs []string

	// Limit rows to prevent abuse
	const maxRows = 10000
	rows := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			rowErrs = append(rowErrs, "read error")
			continue
		}
		rows++
		if rows > maxRows {
			rowErrs = append(rowErrs, "row limit exceeded")
			break
		}
		// Validate UTF-8 and sanitize to mitigate CSV injection
		for i := range rec {
			if !utf8.ValidString(rec[i]) {
				rec[i] = sanitizeCSVCell(rec[i])
			}
			rec[i] = sanitizeCSVCell(rec[i])
		}
		get := func(name string) string {
			if j, ok := idx[name]; ok && j < len(rec) {
				return strings.TrimSpace(rec[j])
			}
			return ""
		}
		tenant := get("tenant_id")
		if tenant == "" {
			tenant = tenantOverride
		}
		metric := get("metric")
		if metric == "" {
			rowErrs = append(rowErrs, "missing metric")
			continue
		}
		desc := get("description")
		owner := get("owner")
		tags := get("tags_json")
		author := get("author")
		key := tenant + "|" + metric
		if _, ok := metricsSeen[key]; !ok {
			metricsSeen[key] = metricRow{tenant: tenant, metric: metric, desc: desc, owner: owner, tags: tags, author: author}
		}
		// label columns optional
		label := get("label")
		if label != "" {
			ltype := get("label_type")
			lreqStr := strings.ToLower(get("label_required"))
			lreq := lreqStr == "true" || lreqStr == "1" || lreqStr == "yes"
			lallowed := get("label_allowed_json")
			ldesc := get("label_description")
			var allowed map[string]any
			if lallowed != "" {
				_ = json.Unmarshal([]byte(lallowed), &allowed)
			}
			if err := h.repo.UpsertMetricLabel(c.Request.Context(), tenant, metric, label, ltype, lreq, allowed, ldesc); err != nil {
				rowErrs = append(rowErrs, "label upsert failed for "+metric+":"+label)
			} else {
				labelsCount++
			}
		}
	}

	// Upsert metrics once
	for _, mr := range metricsSeen {
		tagsSlice := parseTagsJSONToSlice(mr.tags) // <-- array preferred, object BC

		m := repo.MetricDef{
			TenantID:    mr.tenant,
			Metric:      mr.metric,
			Description: mr.desc,
			Owner:       mr.owner,
			Tags:        tagsSlice,
			UpdatedAt:   time.Now(),
		}
		if err := h.repo.UpsertMetric(c.Request.Context(), m, mr.author); err != nil {
			rowErrs = append(rowErrs, "metric upsert failed for "+mr.metric)
		} else {
			metricsCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "ok",
		"metrics_upserted": metricsCount,
		"labels_upserted":  labelsCount,
		"errors":           rowErrs,
		"file":             header.Filename,
	})
}

func (h *SchemaHandler) DebugListAllServices(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Cast to WeaviateRepo to access debug methods
	if wrepo, ok := h.repo.(*repo.WeaviateRepo); ok {
		services, err := wrepo.DebugListServices(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"tenant":    tenantID,
			"services":  services,
			"repo_type": "WeaviateRepo",
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not using WeaviateRepo"})
	}
}

// SampleCSV generates a CSV template populated with metric and label keys for user to fill.
// Query params:
//
//	metrics: CSV list of metric names (optional). If absent, emits only the header row.
func (h *SchemaHandler) SampleCSV(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename=metric_definitions_sample.csv")
	w := csv.NewWriter(c.Writer)
	header := []string{"tenant_id", "metric", "description", "owner", "tags_json", "label", "label_type", "label_required", "label_allowed_json", "label_description", "author"}
	_ = w.Write(header)

	metricsCSV := c.Query("metrics")
	if h.metricsService == nil {
		w.Flush()
		return
	}
	tenantID := c.GetString("tenant_id")
	var metricsList []string
	if metricsCSV == "" {
		// Fetch all metric names via VM label values for __name__
		names, err := h.metricsService.GetLabelValues(c.Request.Context(), &models.LabelValuesRequest{Label: "__name__", TenantID: tenantID})
		if err == nil {
			metricsList = names
		}
	} else {
		metricsList = splitCSVParam(metricsCSV)
	}
	for _, mname := range metricsList {
		// fetch labels for this metric via VM labels API with match[]
		labels, err := h.getLabelNamesForMetric(c, tenantID, mname)
		if err != nil {
			continue
		}
		if len(labels) == 0 {
			_ = w.Write([]string{"", mname, "", "", "[]", "", "", "", "{}", "", ""})
			continue
		}
		for _, lk := range labels {
			_ = w.Write([]string{"", mname, "", "", "[]", lk, "", "", "{}", "", ""})
		}
	}
	w.Flush()
}

// BulkUpsertLogFieldsCSV ingests log field definitions via CSV.
// Columns: tenant_id, field, type, description, tags_json, examples_json, author
func (h *SchemaHandler) BulkUpsertLogFieldsCSV(c *gin.Context) {
	if limited := h.enforceQuota(c, "logs", 20); limited {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)
	if err := c.Request.ParseMultipartForm(6 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form or file too large"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	allowedCT := map[string]struct{}{"text/csv": {}, "application/vnd.ms-excel": {}, "text/plain": {}}
	if _, ok := allowedCT[strings.ToLower(ct)]; !ok && ct != "" { /* sniff below */
	}
	var sniff [512]byte
	n, _ := file.Read(sniff[:])
	detected := http.DetectContentType(sniff[:n])
	if _, ok := allowedCT[detected]; !ok && detected != "application/octet-stream" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported content type"})
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	headerRow, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty csv"})
		return
	}
	if len(headerRow) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many columns"})
		return
	}
	for _, col := range headerRow {
		if !utf8.ValidString(col) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid utf-8 in header"})
			return
		}
	}
	idx := make(map[string]int)
	for i, col := range headerRow {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	allowed := map[string]struct{}{"tenant_id": {}, "field": {}, "type": {}, "description": {}, "tags_json": {}, "examples_json": {}, "author": {}}
	for k := range idx {
		if _, ok := allowed[k]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown column: " + k})
			return
		}
	}
	if _, ok := idx["field"]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing column: field"})
		return
	}

	tenantOverride := c.GetString("tenant_id")
	fieldsCount := 0
	var rowErrs []string
	const maxRows = 10000
	rows := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			rowErrs = append(rowErrs, "read error")
			continue
		}
		rows++
		if rows > maxRows {
			rowErrs = append(rowErrs, "row limit exceeded")
			break
		}
		for i := range rec {
			if !utf8.ValidString(rec[i]) {
				rec[i] = sanitizeCSVCell(rec[i])
			}
			rec[i] = sanitizeCSVCell(rec[i])
		}
		get := func(name string) string {
			if j, ok := idx[name]; ok && j < len(rec) {
				return strings.TrimSpace(rec[j])
			}
			return ""
		}
		tenant := get("tenant_id")
		if tenant == "" {
			tenant = tenantOverride
		}
		field := get("field")
		if field == "" {
			rowErrs = append(rowErrs, "missing field")
			continue
		}
		typ := get("type")
		desc := get("description")
		tags := get("tags_json")
		examples := get("examples_json")
		author := get("author")

		// Tags: prefer JSON array of strings; legacy object -> ["k=v", ...]
		tagsSlice := parseTagsJSONToSlice(tags)

		// Examples: keep as map[string]any
		exObj := parseJSONToMap(examples)

		f := repo.LogFieldDef{TenantID: tenant, Field: field, Type: typ, Description: desc, Tags: tagsSlice, Examples: exObj, UpdatedAt: time.Now()}
		if err := h.repo.UpsertLogField(c.Request.Context(), f, author); err != nil {
			rowErrs = append(rowErrs, "field upsert failed: "+field)
		} else {
			fieldsCount++
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "fields_upserted": fieldsCount, "errors": rowErrs, "file": header.Filename})
}

// SampleCSVLogFields downloads a CSV containing all discovered log field names for users to fill.
func (h *SchemaHandler) SampleCSVLogFields(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename=log_field_definitions_sample.csv")
	w := csv.NewWriter(c.Writer)
	header := []string{"tenant_id", "field", "type", "description", "tags_json", "examples_json", "author"}
	_ = w.Write(header)
	if h.logsService == nil {
		w.Flush()
		return
	}
	tenantID := c.GetString("tenant_id")
	fields, err := h.logsService.GetFields(c.Request.Context(), tenantID)
	if err == nil {
		for _, f := range fields {
			_ = w.Write([]string{"", f, "", "", "[]", "{}", ""})
		}
	}
	w.Flush()
}

// enforceQuota increments a per-tenant daily counter and returns true if over limit.
func (h *SchemaHandler) enforceQuota(c *gin.Context, kind string, limit int) bool {
	if h.cache == nil || limit <= 0 {
		return false
	}
	tenant := c.GetString("tenant_id")
	if tenant == "" {
		tenant = "default"
	}
	day := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("bulk_upload:%s:%s:%s", kind, tenant, day)
	// get existing
	var count int
	if b, err := h.cache.Get(c.Request.Context(), key); err == nil {
		_, _ = fmt.Sscanf(string(b), "%d", &count)
	}
	if count >= limit {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "bulk upload quota exceeded"})
		return true
	}
	count++
	_ = h.cache.Set(c.Request.Context(), key, fmt.Sprintf("%d", count), 24*time.Hour)
	return false
}

// getLabelNamesForMetric uses VM labels endpoint with a match[] selector for a metric.
func (h *SchemaHandler) getLabelNamesForMetric(c *gin.Context, tenantID, metric string) ([]string, error) {
	if h.metricsService == nil {
		return nil, fmt.Errorf("no metrics service")
	}
	sel := fmt.Sprintf("{__name__=\"%s\"}", metric)
	req := &models.LabelsRequest{Start: "", End: "", Match: []string{sel}, TenantID: tenantID}
	labels, err := h.metricsService.GetLabels(c.Request.Context(), req)
	return labels, err
}

// splitCSVParam splits a comma-separated list safely.
func splitCSVParam(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sanitizeCSVCell prevents CSV injection by prefixing risky values and enforcing UTF-8.
func sanitizeCSVCell(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}
