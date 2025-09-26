package services

import (
    "bytes"
    "compress/gzip"
    "context"
    "encoding/csv"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "sort"
    "strconv"
    "strings"
    "time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/utils"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"sync"
)

type VictoriaLogsService struct {
    name      string
    endpoints []string
    timeout   time.Duration
    client    *http.Client
    logger    logger.Logger
    current   int
    mu        sync.Mutex

    username string
    password string

    // Optional child services for multi-source aggregation
    children []*VictoriaLogsService
}

func NewVictoriaLogsService(cfg config.VictoriaLogsConfig, logger logger.Logger) *VictoriaLogsService {
    return &VictoriaLogsService{
        name:      cfg.Name,
        endpoints: cfg.Endpoints,
        timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
        client: &http.Client{
            Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
        },
        logger:   logger,
        username: cfg.Username,
        password: cfg.Password,
    }
}

// SetChildren configures downstream services used for aggregation
func (s *VictoriaLogsService) SetChildren(children []*VictoriaLogsService) {
    s.mu.Lock()
    s.children = children
    s.mu.Unlock()
    if len(children) > 0 {
        s.logger.Info("VictoriaLogs multi-source aggregation enabled", "sources", len(children)+boolToInt(len(s.endpoints) > 0))
    }
}

// -------------------------------------------------------------------
// ExecuteQuery delegates to ExecuteQueryStream and buffers rows in memory.
// -------------------------------------------------------------------
func (s *VictoriaLogsService) ExecuteQuery(
    ctx context.Context,
    req *models.LogsQLQueryRequest,
) (*models.LogsQLQueryResult, error) {

    // Aggregation: fan-out and merge results when children present
    if len(s.children) > 0 {
        type out struct{ res *models.LogsQLQueryResult; err error }
        services := make([]*VictoriaLogsService, 0, len(s.children)+1)
        if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() { services = append(services, s) }
        services = append(services, s.children...)
        ch := make(chan out, len(services))
        for _, svc := range services {
            go func(svc *VictoriaLogsService) { r, e := svc.ExecuteQuery(ctx, req); ch <- out{r, e} }(svc)
        }
        fieldsSet := map[string]struct{}{}
        var merged []map[string]any
        totalCount := 0
        totalBytes := int64(0)
        sources := make([]map[string]any, 0, len(services))
        success := 0
        for i := 0; i < len(services); i++ {
            o := <-ch
            if o.err != nil || o.res == nil { continue }
            if len(o.res.Logs) > 0 { merged = append(merged, o.res.Logs...) }
            for _, f := range o.res.Fields { fieldsSet[f] = struct{}{} }
            if o.res.Stats != nil {
                if v, ok := toInt(o.res.Stats["count"]); ok { totalCount += v }
                if b, ok := toInt64(o.res.Stats["bytes_read"]); ok { totalBytes += b }
                src := map[string]any{"source": svcName(o.res.Stats), "stats": o.res.Stats}
                sources = append(sources, src)
            }
            success++
        }
        if success == 0 { return nil, fmt.Errorf("all logs sources failed") }
        fields := make([]string, 0, len(fieldsSet))
        for k := range fieldsSet { fields = append(fields, k) }
        sort.Strings(fields)
        return &models.LogsQLQueryResult{
            Logs:   merged,
            Fields: fields,
            Stats: map[string]any{
                "count":       totalCount,
                "bytes_read":  totalBytes,
                "aggregated":  true,
                "sources":     sources,
                "streaming":   false,
            },
        }, nil
    }

	var rows []map[string]any
	onRow := func(m map[string]any) error {
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
    if len(rows) == 0 {
        res.Logs = make([]map[string]any, 0)
    } else {
        res.Logs = rows
    }
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

    if len(s.children) > 0 {
        // Fan out streaming queries. Serialize onRow to keep caller safety.
        mu := &sync.Mutex{}
        safeOnRow := func(m map[string]any) error { mu.Lock(); defer mu.Unlock(); return onRow(m) }
        services := make([]*VictoriaLogsService, 0, len(s.children)+1)
        if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() { services = append(services, s) }
        services = append(services, s.children...)
        type out struct{ res *models.LogsQLQueryResult; err error }
        ch := make(chan out, len(services))
        for _, svc := range services {
            go func(svc *VictoriaLogsService) { r, e := svc.ExecuteQueryStream(ctx, req, safeOnRow); ch <- out{r, e} }(svc)
        }
        fields := map[string]struct{}{}
        totalCount := 0
        totalBytes := int64(0)
        sources := make([]map[string]any, 0, len(services))
        success := 0
        for i := 0; i < len(services); i++ {
            o := <-ch
            if o.err != nil || o.res == nil { continue }
            for _, f := range o.res.Fields { fields[f] = struct{}{} }
            if o.res.Stats != nil {
                if v, ok := toInt(o.res.Stats["count"]); ok { totalCount += v }
                if b, ok := toInt64(o.res.Stats["bytes_read"]); ok { totalBytes += b }
                sources = append(sources, map[string]any{"source": svcName(o.res.Stats), "stats": o.res.Stats})
            }
            success++
        }
        if success == 0 { return nil, fmt.Errorf("all logs sources failed") }
        // finalize
        fieldList := make([]string, 0, len(fields))
        for k := range fields { fieldList = append(fieldList, k) }
        sort.Strings(fieldList)
        return &models.LogsQLQueryResult{
            Logs:   nil,
            Fields: fieldList,
            Stats: map[string]any{
                "count":      totalCount,
                "bytes_read": totalBytes,
                "aggregated": true,
                "sources":    sources,
                "streaming":  true,
            },
        }, nil
    }

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

    // Use the documented VictoriaLogs endpoint for querying logs
    // https://docs.victoriametrics.com/victorialogs/querying/#http-api
    // The correct path is /select/logsql/query (no /export, no /api/v1)
    queryPath := strings.TrimRight(base, "/") + "/select/logsql/query"
    u, err := url.Parse(queryPath)
    if err != nil {
        return nil, fmt.Errorf("invalid endpoint: %w", err)
    }

    q := url.Values{}
    // Prefer JSON output to simplify parsing across VL versions
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
	u.RawQuery = q.Encode()

    httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
    if err != nil {
        return nil, fmt.Errorf("build request: %w", err)
    }
    httpReq.Header.Set("Accept", "*/*")
    httpReq.Header.Set("Accept-Encoding", "gzip")
    if s.username != "" { httpReq.SetBasicAuth(s.username, s.password) }

	if t := strings.TrimSpace(req.TenantID); t != "" {
        // VictoriaLogs expects numeric AccountID when multitenancy is enabled.
        if utils.IsUint32String(t) {
            httpReq.Header.Set("AccountID", t)
        }
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
        if msg != "" { return nil, fmt.Errorf("victoria logs %d: %s", resp.StatusCode, msg) }
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

    // Read the full payload to handle both NDJSON and JSON-wrapped formats.
    payload, err := io.ReadAll(reader)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    fieldsUnion := make(map[string]struct{}, 64)
    var (
        bytesRead int64 = int64(len(payload))
        count     int
    )

    consumeRow := func(row map[string]any) error {
        for k := range row { fieldsUnion[k] = struct{}{} }
        if err := onRow(row); err != nil { return err }
        count++
        return nil
    }

    data := bytes.TrimSpace(payload)
    if len(data) == 0 {
        // Some older builds may ignore format param; retry without it
        qp := u.Query()
        qp.Del("format")
        u2 := *u
        u2.RawQuery = qp.Encode()
        httpReq2, _ := http.NewRequestWithContext(ctx, http.MethodGet, u2.String(), nil)
        httpReq2.Header = httpReq.Header.Clone()
        _ = resp.Body.Close()
        resp, err = s.client.Do(httpReq2)
        if err != nil { return nil, fmt.Errorf("request retry: %w", err) }
        defer resp.Body.Close()
        if resp.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("victoria logs %d: %s", resp.StatusCode, readErrBody(resp.Body))
        }
        if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
            gr, gzErr := gzip.NewReader(resp.Body)
            if gzErr != nil { return nil, fmt.Errorf("gzip reader: %w", gzErr) }
            defer gr.Close()
            reader = gr
            compressed = true
        } else {
            reader = resp.Body
        }
        payload, err = io.ReadAll(reader)
        if err != nil { return nil, fmt.Errorf("read response: %w", err) }
    }

    if len(data) > 0 {
        // Case 1: JSON wrapper {"status":"success","data":[{...}, ...]}
        var wrapped struct {
            Status string        `json:"status"`
            Data   interface{}   `json:"data"`
            Fields []string      `json:"fields"`
        }
        if json.Unmarshal(data, &wrapped) == nil && wrapped.Data != nil {
            switch v := wrapped.Data.(type) {
            case []interface{}:
                if len(wrapped.Fields) > 0 {
                    // rows format: fields + data as arrays
                    for _, it := range v {
                        arr, ok := it.([]interface{})
                        if !ok { continue }
                        row := make(map[string]any, len(wrapped.Fields))
                        for i := 0; i < len(wrapped.Fields) && i < len(arr); i++ {
                            row[wrapped.Fields[i]] = arr[i]
                        }
                        if err := consumeRow(row); err != nil { return nil, err }
                        if req.Limit > 0 && count >= req.Limit { break }
                    }
                } else {
                    // array of JSON objects
                    for _, it := range v {
                        if m, ok := it.(map[string]interface{}); ok {
                            if err := consumeRow(m); err != nil { return nil, err }
                            if req.Limit > 0 && count >= req.Limit { break }
                        }
                    }
                }
            case []map[string]interface{}:
                for _, m := range v {
                    if err := consumeRow(m); err != nil { return nil, err }
                    if req.Limit > 0 && count >= req.Limit { break }
                }
            default:
                // Not an array; fall through to NDJSON attempt
                wrapped.Data = nil
            }
        }
        if count == 0 { // Case 2: NDJSON or concatenated JSON objects
            dec := json.NewDecoder(bytes.NewReader(data))
            for {
                var row map[string]any
                if err := dec.Decode(&row); err != nil {
                    if errors.Is(err, io.EOF) { break }
                    // If decoding fails immediately, exit with error text to help debugging
                    return nil, fmt.Errorf("decode logs json: %w", err)
                }
                if len(row) == 0 { continue }
                if err := consumeRow(row); err != nil { return nil, err }
                if req.Limit > 0 && count >= req.Limit { break }
            }
        }
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

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")
    if s.username != "" { req.SetBasicAuth(s.username, s.password) }
	if utils.IsUint32String(tenantID) {
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

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    if s.username != "" { req.SetBasicAuth(s.username, s.password) }
	if utils.IsUint32String(tenantID) {
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
			f := &models.SystemFracture{}
			if v, ok := prediction["id"].(string); ok {
				f.ID = v
			}
			if v, ok := prediction["component"].(string); ok {
				f.Component = v
			}
			fractures = append(fractures, f)
		}
	}
	return fractures, nil
}

func (s *VictoriaLogsService) selectEndpoint() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    if len(s.endpoints) == 0 {
        return ""
    }
    ep := s.endpoints[s.current%len(s.endpoints)]
    s.current++
    return ep
}

// ReplaceEndpoints swaps endpoints list (used by discovery)
func (s *VictoriaLogsService) ReplaceEndpoints(eps []string) {
    s.mu.Lock()
    s.endpoints = append([]string(nil), eps...)
    s.current = 0
    s.mu.Unlock()
    s.logger.Info("VictoriaLogs endpoints updated", "source", s.name, "count", len(eps))
}

func (s *VictoriaLogsService) HealthCheck(ctx context.Context) error {
    if len(s.children) > 0 {
        if err := s.healthCheckSelf(ctx); err == nil { return nil }
        for _, c := range s.children { if c.HealthCheck(ctx) == nil { return nil } }
        return fmt.Errorf("all logs sources unhealthy")
    }
    return s.healthCheckSelf(ctx)
}

func (s *VictoriaLogsService) healthCheckSelf(ctx context.Context) error {
    endpoint := s.selectEndpoint()
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/health", nil)
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

// GetStreams queries VictoriaLogs for available log streams.
func (s *VictoriaLogsService) GetStreams(ctx context.Context, tenantID string, limit int) ([]map[string]string, error) {
    if len(s.children) > 0 {
        services := make([]*VictoriaLogsService, 0, len(s.children)+1)
        if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() { services = append(services, s) }
        services = append(services, s.children...)
        ch := make(chan struct{ out []map[string]string; err error }, len(services))
        for _, svc := range services {
            go func(svc *VictoriaLogsService) { o, e := svc.GetStreams(ctx, tenantID, limit); ch <- struct{ out []map[string]string; err error }{o, e} }(svc)
        }
        seen := map[string]struct{}{}
        var merged []map[string]string
        for i := 0; i < len(services); i++ {
            r := <-ch
            if r.err != nil { continue }
            for _, m := range r.out {
                if v := m["label"]; v != "" {
                    if _, ok := seen[v]; ok { continue }
                    seen[v] = struct{}{}
                    merged = append(merged, map[string]string{"label": v})
                    if limit > 0 && len(merged) >= limit { return merged, nil }
                }
            }
        }
        return merged, nil
    }
    // VictoriaLogs does not expose a generic "/labels" endpoint. Derive useful
    // stream labels from available field names and common conventions.
    fields, err := s.GetFields(ctx, tenantID)
    if err != nil {
        // Fall back to a conservative default list
        fields = []string{"service", "level", "host"}
    }

    // Preferred candidates typically used for stream-like grouping.
    candidates := []string{"service", "app", "application", "component", "level", "host", "namespace", "pod", "container"}

    contains := func(list []string, v string) bool {
        for _, x := range list { if x == v { return true } }
        return false
    }

    pick := make([]map[string]string, 0, len(fields))
    for _, c := range candidates {
        if contains(fields, c) {
            pick = append(pick, map[string]string{"label": c})
            if limit > 0 && len(pick) >= limit { return pick, nil }
        }
    }
    // If nothing matched, just return the first N field names as labels
    if len(pick) == 0 {
        for _, f := range fields {
            pick = append(pick, map[string]string{"label": f})
            if limit > 0 && len(pick) >= limit { break }
        }
    }
    return pick, nil
}

// GetFields retrieves available log fields.
func (s *VictoriaLogsService) GetFields(ctx context.Context, tenantID string) ([]string, error) {
    if len(s.children) > 0 {
        services := make([]*VictoriaLogsService, 0, len(s.children)+1)
        if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() { services = append(services, s) }
        services = append(services, s.children...)
        ch := make(chan struct{ out []string; err error }, len(services))
        for _, svc := range services { go func(svc *VictoriaLogsService){ o, e := svc.GetFields(ctx, tenantID); ch <- struct{out []string; err error}{o, e} }(svc) }
        set := map[string]struct{}{}
        for i := 0; i < len(services); i++ {
            r := <-ch
            if r.err != nil { continue }
            for _, f := range r.out { set[f] = struct{}{} }
        }
        fields := make([]string, 0, len(set))
        for k := range set { fields = append(fields, k) }
        sort.Strings(fields)
        return fields, nil
    }
    // Hardcode a query equivalent to:
    // { "query": "*", "start": now-10m, "end": now, "limit": 500 }
    nowMs := time.Now().UnixMilli()
    req := &models.LogsQLQueryRequest{
        Query:    "*",
        Start:    nowMs - 10*60*1000,
        End:      nowMs,
        Limit:    500,
        TenantID: tenantID,
    }
    res, err := s.ExecuteQuery(ctx, req)
    if err != nil {
        return nil, err
    }
    if res == nil || len(res.Fields) == 0 {
        return []string{}, nil
    }
    return res.Fields, nil
}

// ExportLogs returns a binary export (used by handler for download).
func (s *VictoriaLogsService) ExportLogs(ctx context.Context, request *models.LogsExportRequest) (*models.LogsExportResult, error) {
	if request == nil {
		return nil, fmt.Errorf("nil export request")
	}
	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return nil, fmt.Errorf("no victoria logs endpoint configured")
	}

	params := url.Values{}
	if strings.TrimSpace(request.Query) != "" {
		params.Set("query", request.Query)
	}
    if request.Start > 0 {
        params.Set("start", strconv.FormatInt(normalizeToMillis(request.Start), 10))
    }
    if request.End > 0 {
        params.Set("end", strconv.FormatInt(normalizeToMillis(request.End), 10))
    }
	if request.Limit > 0 {
		params.Set("limit", strconv.Itoa(request.Limit))
	}
    format := strings.ToLower(strings.TrimSpace(request.Format))
    if format == "" {
        // Default to CSV for downloads
        format = "csv"
    }
	params.Set("format", format)

    // Use the documented query endpoint and pass the desired format.
    // There is no /export for VictoriaLogs HTTP API.
    // https://docs.victoriametrics.com/victorialogs/querying/#http-api
    queryURL := fmt.Sprintf("%s/select/logsql/query?%s", endpoint, params.Encode())
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
    if err != nil {
        return nil, err
    }
    if utils.IsUint32String(request.TenantID) {
        req.Header.Set("AccountID", request.TenantID)
    }
    // Hint desired response type to VictoriaLogs.
    if format == "csv" {
        req.Header.Set("Accept", "text/csv")
    } else {
        req.Header.Set("Accept", "application/json, */*")
    }

    resp, err := s.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("VictoriaLogs export failed with status %d: %s", resp.StatusCode, readErrBody(resp.Body))
    }

    // Read body (handle optional gzip)
    var r io.Reader = resp.Body
    if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
        gr, gzErr := gzip.NewReader(resp.Body)
        if gzErr == nil {
            defer gr.Close()
            r = gr
        }
    }
    data, err := io.ReadAll(r)
    if err != nil {
        return nil, fmt.Errorf("read export: %w", err)
    }

    ct := strings.ToLower(resp.Header.Get("Content-Type"))
    if format == "csv" && !strings.Contains(ct, "csv") {
        // Convert JSON/NDJSON payload to CSV
        csvData, convErr := toCSV(data)
        if convErr == nil {
            data = csvData
        } else {
            // If conversion failed, keep original data but annotate warning
            s.logger.Warn("CSV conversion failed; returning original payload", "error", convErr)
        }
    }

	filename := fmt.Sprintf("logs-%d.%s", time.Now().Unix(), format)
	return &models.LogsExportResult{
		Filename: filename,
		Format:   format,
		Size:     len(data),
		Data:     data,
	}, nil
}

// toCSV converts a VictoriaLogs JSON/NDJSON query response into CSV bytes.
// It supports these shapes:
// 1) {"fields":[...],"data":[[...], ...]}
// 2) [{...}, {...}] (array of objects)
// 3) NDJSON / concatenated JSON objects
func toCSV(payload []byte) ([]byte, error) {
    b := bytes.TrimSpace(payload)
    if len(b) == 0 { return []byte{}, nil }

    // 1) Wrapped rows format
    var wr struct {
        Status string          `json:"status"`
        Fields []string        `json:"fields"`
        Data   []json.RawMessage `json:"data"`
    }
    if json.Unmarshal(b, &wr) == nil && len(wr.Fields) > 0 && len(wr.Data) > 0 {
        // Data may be [][]any; decode lazily per row
        buf := &bytes.Buffer{}
        w := csv.NewWriter(buf)
        _ = w.Write(wr.Fields)
        for _, raw := range wr.Data {
            var arr []any
            if json.Unmarshal(raw, &arr) != nil { continue }
            rec := make([]string, len(wr.Fields))
            for i := 0; i < len(wr.Fields) && i < len(arr); i++ {
                rec[i] = toScalarString(arr[i])
            }
            _ = w.Write(rec)
        }
        w.Flush()
        return buf.Bytes(), nil
    }

    // 2) Array of objects
    var objs1 []map[string]any
    if json.Unmarshal(b, &objs1) == nil && len(objs1) > 0 {
        return csvFromObjects(objs1, nil)
    }

    // 3) NDJSON / concatenated JSON objects
    dec := json.NewDecoder(bytes.NewReader(b))
    var rows []map[string]any
    for {
        var m map[string]any
        if err := dec.Decode(&m); err != nil {
            if errors.Is(err, io.EOF) { break }
            // If failing early, abort conversion
            if len(rows) == 0 { return nil, err }
            break
        }
        if len(m) > 0 { rows = append(rows, m) }
    }
    if len(rows) > 0 {
        return csvFromObjects(rows, nil)
    }
    // Unknown shape: return original payload
    return payload, fmt.Errorf("unrecognized payload shape for CSV conversion")
}

func csvFromObjects(rows []map[string]any, prefer []string) ([]byte, error) {
    fields := make([]string, 0)
    seen := map[string]struct{}{}
    if len(prefer) > 0 {
        for _, f := range prefer { seen[f] = struct{}{} }
        fields = append(fields, prefer...)
    }
    // collect all keys
    for _, r := range rows {
        for k := range r {
            if _, ok := seen[k]; !ok { seen[k] = struct{}{}; fields = append(fields, k) }
        }
    }
    // stable order: if no preferred fields given, sort
    if len(prefer) == 0 { sort.Strings(fields) }

    buf := &bytes.Buffer{}
    w := csv.NewWriter(buf)
    if err := w.Write(fields); err != nil { return nil, err }
    for _, r := range rows {
        rec := make([]string, len(fields))
        for i, f := range fields { rec[i] = toScalarString(r[f]) }
        if err := w.Write(rec); err != nil { return nil, err }
    }
    w.Flush()
    if err := w.Error(); err != nil { return nil, err }
    return buf.Bytes(), nil
}

func toScalarString(v any) string {
    switch x := v.(type) {
    case nil:
        return ""
    case string:
        return x
    case float64, float32, int, int32, int64, uint, uint32, uint64, bool:
        return fmt.Sprint(x)
    default:
        b, _ := json.Marshal(x)
        return string(b)
    }
}

// isUint32 returns true if s parses as base-10 uint32
// (moved) tenant ID numeric check lives in utils.IsUint32String

// helpers for aggregation
func toInt(v any) (int, bool) {
    switch x := v.(type) {
    case int: return x, true
    case int64: return int(x), true
    case float64: return int(x), true
    case float32: return int(x), true
    case json.Number:
        if i, err := x.Int64(); err == nil { return int(i), true }
    }
    return 0, false
}
func toInt64(v any) (int64, bool) {
    switch x := v.(type) {
    case int64: return x, true
    case int: return int64(x), true
    case float64: return int64(x), true
    case float32: return int64(x), true
    case json.Number:
        if i, err := x.Int64(); err == nil { return i, true }
    }
    return 0, false
}
func svcName(stats map[string]any) string {
    if stats == nil { return "" }
    if v, ok := stats["source"].(string); ok { return v }
    if v, ok := stats["endpoint"].(string); ok { return v }
    return ""
}
