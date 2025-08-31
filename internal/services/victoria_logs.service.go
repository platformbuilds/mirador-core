package services

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type VictoriaLogsService struct {
	endpoints []string
	timeout   time.Duration
	client    *http.Client
	logger    logger.Logger
	current   int
}

func NewVictoriaLogsService(cfg config.VictoriaLogsConfig, logger logger.Logger) *VictoriaLogsService {
	return &VictoriaLogsService{
		endpoints: cfg.Endpoints,
		timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
		},
		logger: logger,
	}
}

// -------------------------------------------------------------------
// ExecuteQuery delegates to ExecuteQueryStream and buffers rows in memory.
// Existing handlers depending on []map[string]any keep working unchanged.
// -------------------------------------------------------------------
func (s *VictoriaLogsService) ExecuteQuery(
	ctx context.Context,
	req *models.LogsQLQueryRequest,
) (*models.LogsQLQueryResult, error) {

	var rows []map[string]any
	onRow := func(m map[string]any) error {
		// shallow copy to avoid scanner buffer reuse issues
		cp := make(map[string]any, len(m))
		for k, v := range m {
			cp[k] = v
		}
		rows = append(rows, cp)
		return nil
	}

	res, err := s.ExecuteQueryStream(ctx, req, onRow)
	if err != nil {
		return nil, err
	}
	res.Logs = rows
	if res.Stats != nil {
		res.Stats["streaming"] = false
	}
	return res, nil
}

// -------------------------------------------------------------------
// ExecuteQueryStream is the single source of truth for VictoriaLogs queries.
// -------------------------------------------------------------------
func (s *VictoriaLogsService) ExecuteQueryStream(
	ctx context.Context,
	req *models.LogsQLQueryRequest,
	onRow func(row map[string]any) error,
) (*models.LogsQLQueryResult, error) {

	if req == nil {
		return nil, errors.New("nil request")
	}
	if onRow == nil {
		return nil, errors.New("onRow callback is required")
	}

	startWall := time.Now()
	base := s.selectEndpoint()
	if base == "" {
		return nil, errors.New("no victoria logs endpoint configured")
	}

	u, err := url.Parse(strings.TrimRight(base, "/") + "/select/logsql/api/v1/export")
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	q := url.Values{}
	q.Set("format", "json")
	if strings.TrimSpace(req.Query) != "" {
		q.Set("query", req.Query)
	}
	if req.Start > 0 {
		q.Set("start", strconv.FormatInt(normalizeToMillis(req.Start), 10))
	}
	if req.End > 0 {
		q.Set("end", strconv.FormatInt(normalizeToMillis(req.End), 10))
	}
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}

	// Optional: pass extra flags if request supports them
	type flags interface{ GetExtra() map[string]string }
	if f, ok := any(req).(flags); ok {
		for k, v := range f.GetExtra() {
			if k != "" && v != "" {
				q.Set(k, v)
			}
		}
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Accept-Encoding", "gzip")

	if t := strings.TrimSpace(req.TenantID); t != "" {
		httpReq.Header.Set("X-Scope-OrgID", t)
		httpReq.Header.Set("AccountID", t)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readErrBody(resp.Body)
		if msg != "" {
			return nil, fmt.Errorf("victoria logs %d: %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("victoria logs returned status %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	compressed := false
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gr, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return nil, fmt.Errorf("gzip reader: %w", gzErr)
		}
		defer gr.Close()
		reader = gr
		compressed = true
	}

	sc := bufio.NewScanner(reader)
	const maxLine = 16 * 1024 * 1024
	sc.Buffer(make([]byte, 0, 256*1024), maxLine)

	fieldsUnion := make(map[string]struct{}, 64)
	var (
		bytesRead int64
		count     int
	)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		b := sc.Bytes()
		bytesRead += int64(len(b))
		line := bytes.TrimSpace(b)
		if len(line) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, fmt.Errorf("decode json line: %w", err)
		}
		for k := range row {
			fieldsUnion[k] = struct{}{}
		}
		if err := onRow(row); err != nil {
			return nil, err
		}
		count++
		if req.Limit > 0 && count >= req.Limit {
			break
		}
	}
	if err := sc.Err(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("stream read error: %w", err)
	}

	fields := make([]string, 0, len(fieldsUnion))
	for k := range fieldsUnion {
		fields = append(fields, k)
	}

	took := time.Since(startWall)
	return &models.LogsQLQueryResult{
		Logs:   nil,
		Fields: fields,
		Stats: map[string]any{
			"count":      count,
			"bytes_read": bytesRead,
			"took_ms":    took.Milliseconds(),
			"endpoint":   base,
			"compressed": compressed,
			"streaming":  true,
		},
	}, nil
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------
func normalizeToMillis(v int64) int64 {
	switch {
	case v <= 0:
		return 0
	case v < 1_000_000_000:
		return v * 1000
	case v < 10_000_000_000:
		return v * 1000
	case v < 1_000_000_000_000:
		return v * 1000
	case v < 10_000_000_000_000:
		return v
	default:
		return v / 1_000_000
	}
}

func readErrBody(r io.Reader) string {
	const max = 64 * 1024
	b, _ := io.ReadAll(io.LimitReader(r, max))
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(b, &m) == nil {
		if msg, ok := m["error"].(string); ok && msg != "" {
			return msg
		}
		if msg, ok := m["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return s
}

// -------------------------------------------------------------------
// Existing methods kept intact
// -------------------------------------------------------------------
func (s *VictoriaLogsService) StoreJSONEvent(ctx context.Context, event map[string]interface{}, tenantID string) error {
	logEntry := map[string]interface{}{
		"_time": event["_time"],
		"_msg":  event["_msg"],
		"data":  event,
	}
	jsonData, err := json.Marshal([]map[string]interface{}{logEntry})
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}
	endpoint := s.selectEndpoint()
	url := fmt.Sprintf("%s/insert/jsonline", endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if tenantID != "" {
		req.Header.Set("AccountID", tenantID)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VictoriaLogs returned status %d", resp.StatusCode)
	}
	s.logger.Info("Event stored in VictoriaLogs", "type", event["type"], "tenant", tenantID)
	return nil
}

func (s *VictoriaLogsService) QueryPredictionEvents(ctx context.Context, query, tenantID string) ([]*models.SystemFracture, error) {
	endpoint := s.selectEndpoint()
	url := fmt.Sprintf("%s/select/logsql/query", endpoint)
	reqBody := map[string]interface{}{"query": query, "limit": 1000}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tenantID != "" {
		req.Header.Set("AccountID", tenantID)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var queryResponse models.LogsQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResponse); err != nil {
		return nil, err
	}
	var fractures []*models.SystemFracture
	for _, entry := range queryResponse.Data {
		if prediction, ok := entry["prediction"].(map[string]interface{}); ok {
			fracture := &models.SystemFracture{
				ID:        prediction["id"].(string),
				Component: prediction["component"].(string),
			}
			fractures = append(fractures, fracture)
		}
	}
	return fractures, nil
}

func (s *VictoriaLogsService) selectEndpoint() string {
	return s.endpoints[time.Now().Unix()%int64(len(s.endpoints))]
}

func (s *VictoriaLogsService) HealthCheck(ctx context.Context) error {
	endpoint := s.selectEndpoint()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VictoriaLogs health check failed: status %d", resp.StatusCode)
	}
	return nil
}

// GET /api/v1/logs/fields - unchanged
func (h *LogsQLHandler) GetFields(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	fields, err := h.logsService.GetFields(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get log fields", "tenant", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "Failed to retrieve log fields"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"fields": fields, "total": len(fields)}})
}

// POST /api/v1/logs/export - unchanged
func (h *LogsQLHandler) ExportLogs(c *gin.Context) {
	var request models.LogsExportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Invalid export request format"})
		return
	}
	request.TenantID = c.GetString("tenant_id")
	if err := h.validator.ValidateLogsQL(request.Query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": fmt.Sprintf("Invalid LogsQL query: %s", err.Error())})
		return
	}
	result, err := h.logsService.ExportLogs(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("Failed to export logs", "tenant", request.TenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "Failed to export logs"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
	c.Header("Content-Type", getContentType(result.Format))
	c.Header("Content-Length", fmt.Sprintf("%d", result.Size))
	c.Data(http.StatusOK, getContentType(result.Format), result.Data)
}

func getContentType(format string) string {
	switch format {
	case "json":
		return "application/json"
	case "csv":
		return "text/csv"
	case "txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// GetStreams queries VictoriaLogs for available log streams (distinct label sets).
func (s *VictoriaLogsService) GetStreams(ctx context.Context, tenantID string, limit int) ([]map[string]string, error) {
	endpoint := s.selectEndpoint()
	u := fmt.Sprintf("%s/select/logsql/labels", endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if tenantID != "" {
		req.Header.Set("X-Scope-OrgID", tenantID)
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readErrBody(resp.Body)
		if msg != "" {
			return nil, fmt.Errorf("victoria logs %d: %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("victoria logs returned status %d", resp.StatusCode)
	}

	var labels []string
	if err := json.NewDecoder(resp.Body).Decode(&labels); err != nil {
		return nil, fmt.Errorf("decode labels: %w", err)
	}

	// For now, just wrap each label name into a map to look like a "stream".
	// Later you could expand this to query label values for each.
	streams := make([]map[string]string, 0, len(labels))
	for _, lbl := range labels {
		streams = append(streams, map[string]string{"label": lbl})
		if limit > 0 && len(streams) >= limit {
			break
		}
	}

	return streams, nil
}
