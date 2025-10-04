package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils"
	lq "github.com/platformbuilds/mirador-core/internal/utils/lucene"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// LogsHandler provides D3-friendly APIs (histogram, facets, search, tail)
type LogsHandler struct {
	logs *services.VictoriaLogsService
	log  logger.Logger

	// server limits / defaults
	maxRangeMS int64 // e.g., 24h in ms
	defStepMS  int64 // e.g., 60_000
	defTopN    int
}

func NewLogsHandler(svc *services.VictoriaLogsService, log logger.Logger) *LogsHandler {
	return &LogsHandler{
		logs:       svc,
		log:        log,
		maxRangeMS: 24 * 60 * 60 * 1000, // 24h
		defStepMS:  60_000,              // 1m buckets
		defTopN:    20,
	}
}

// GET /api/v1/logs/histogram
func (h *LogsHandler) Histogram(c *gin.Context) {
	var req models.LogsHistogramRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad query params"})
		return
	}
	// Lucene → LogsQL translation (does not change response shape)
	qlang := strings.ToLower(strings.TrimSpace(c.Query("query_language")))
	if strings.TrimSpace(req.Query) != "" && (qlang == "lucene" || lq.IsLikelyLucene(req.Query)) {
		validator := utils.NewQueryValidator()
		if err := validator.ValidateLucene(req.Query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if translated, ok := lq.Translate(req.Query, lq.TargetLogsQL); ok {
			req.Query = translated
			c.Header("X-Query-Translated-From", "lucene")
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to translate Lucene query"})
			return
		}
	}
	if req.Step <= 0 {
		req.Step = h.defStepMS
	}
	if req.End == 0 {
		req.End = time.Now().UnixMilli()
	}
	if req.Start == 0 {
		req.Start = req.End - h.maxRangeMS
	}
	if req.End-req.Start > h.maxRangeMS {
		req.Start = req.End - h.maxRangeMS
	}
	if req.Limit <= 0 {
		req.Limit = 0 // not used here, but honored by service if set
	}

	bucketCount := int((req.End - req.Start) / req.Step)
	if bucketCount <= 0 || bucketCount > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid step or range"})
		return
	}
	buckets := make([]int, bucketCount)

	rowsSeen := int64(0)
	sampled := req.Sampling > 1
	sampleN := req.Sampling
	if sampleN <= 1 {
		sampleN = 1
	}

	start := req.Start
	end := req.End
	if strings.Contains(req.Query, "_time:") {
		start, end = 0, 0
	}
	_, err := h.logs.ExecuteQueryStream(c.Request.Context(), &models.LogsQLQueryRequest{
		Query:    req.Query,
		Start:    start,
		End:      end,
		TenantID: req.TenantID,
	}, func(row map[string]any) error {
		n := atomic.AddInt64(&rowsSeen, 1)
		if sampleN > 1 && (n%int64(sampleN)) != 0 {
			return nil
		}
		// Expect _time (ms) or similar; try common keys
		ts := extractTS(row)
		if ts < req.Start || ts >= req.End {
			return nil
		}
		idx := int((ts - req.Start) / req.Step)
		if idx >= 0 && idx < len(buckets) {
			buckets[idx]++
		}
		return nil
	})
	if err != nil {
		h.log.Error("histogram query failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "histogram failed"})
		return
	}

	out := make([]models.HistogramBucket, 0, len(buckets))
	for i, v := range buckets {
		out = append(out, models.HistogramBucket{
			TS:    req.Start + int64(i)*req.Step,
			Count: v,
		})
	}
	c.JSON(http.StatusOK, models.LogsHistogramResponse{
		Buckets: out,
		Stats:   map[string]any{"buckets": len(buckets), "sampleN": sampleN},
		Sampled: sampled,
	})
}

// GET /api/v1/logs/facets?fields=service,level
func (h *LogsHandler) Facets(c *gin.Context) {
	var req models.LogsFacetsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad query params"})
		return
	}
	// Lucene → LogsQL translation (does not change response shape)
	qlang := strings.ToLower(strings.TrimSpace(c.Query("query_language")))
	if strings.TrimSpace(req.Query) != "" && (qlang == "lucene" || lq.IsLikelyLucene(req.Query)) {
		validator := utils.NewQueryValidator()
		if err := validator.ValidateLucene(req.Query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if translated, ok := lq.Translate(req.Query, lq.TargetLogsQL); ok {
			req.Query = translated
			c.Header("X-Query-Translated-From", "lucene")
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to translate Lucene query"})
			return
		}
	}
	if req.End == 0 {
		req.End = time.Now().UnixMilli()
	}
	if req.Start == 0 {
		req.Start = req.End - h.maxRangeMS
	}
	if req.End-req.Start > h.maxRangeMS {
		req.Start = req.End - h.maxRangeMS
	}
	topN := req.Limit
	if topN <= 0 || topN > 200 {
		topN = h.defTopN
	}
	sampled := req.Sampling > 1
	sampleN := req.Sampling
	if sampleN <= 1 {
		sampleN = 1
	}

	type counter map[string]int
	counts := map[string]counter{}
	for _, f := range req.Fields {
		if f == "" {
			continue
		}
		counts[f] = counter{}
	}

	rowsSeen := int64(0)
	start := req.Start
	end := req.End
	if strings.Contains(req.Query, "_time:") {
		start, end = 0, 0
	}
	_, err := h.logs.ExecuteQueryStream(c.Request.Context(), &models.LogsQLQueryRequest{
		Query:    req.Query,
		Start:    start,
		End:      end,
		TenantID: req.TenantID,
	}, func(row map[string]any) error {
		n := atomic.AddInt64(&rowsSeen, 1)
		if sampleN > 1 && (n%int64(sampleN)) != 0 {
			return nil
		}
		for field := range counts {
			if v, ok := row[field]; ok && v != nil {
				key := toString(v)
				counts[field][key]++
			}
		}
		return nil
	})
	if err != nil {
		h.log.Error("facets query failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "facets failed"})
		return
	}

	resp := models.LogsFacetsResponse{
		Facets:  make([]models.Facet, 0, len(counts)),
		Stats:   map[string]any{"fields": len(counts), "sampleN": sampleN},
		Sampled: sampled,
	}
	for field, m := range counts {
		// turn map into slice and sort desc
		bs := make([]models.FacetBucket, 0, len(m))
		for k, v := range m {
			bs = append(bs, models.FacetBucket{Key: k, Count: v})
		}
		sort.Slice(bs, func(i, j int) bool { return bs[i].Count > bs[j].Count })
		if len(bs) > topN {
			bs = bs[:topN]
		}
		resp.Facets = append(resp.Facets, models.Facet{
			Field:   field,
			Buckets: bs,
		})
	}
	c.JSON(http.StatusOK, resp)
}

// POST /api/v1/logs/search
func (h *LogsHandler) Search(c *gin.Context) {
	var req models.LogsSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	// Translate Lucene -> LogsQL if requested or detected
	if strings.EqualFold(req.QueryLanguage, "lucene") || lq.IsLikelyLucene(req.Query) {
		validator := utils.NewQueryValidator()
		if err := validator.ValidateLucene(req.Query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if translated, ok := lq.Translate(req.Query, lq.TargetLogsQL); ok {
			req.Query = translated
			c.Header("X-Query-Translated-From", "lucene")
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to translate Lucene query"})
			return
		}
	}
	if req.Limit <= 0 || req.Limit > 10_000 {
		req.Limit = 1000
	}
	if req.End == 0 {
		req.End = time.Now().UnixMilli()
	}
	if req.Start == 0 || req.End-req.Start > h.maxRangeMS {
		req.Start = req.End - h.maxRangeMS
	}

	var (
		rows      = make([]map[string]any, 0, req.Limit)
		fieldsSet = map[string]struct{}{}
		count     = 0
		afterTS   int64
		afterOff  int
	)

	// Apply cursor by skipping until cursor is passed
	skipUntilCursor := func(ts int64, off *int) bool {
		if req.PageAfter == nil {
			return false
		}
		// if same millisecond, skip first PageAfter.Offset rows
		if ts == req.PageAfter.TS {
			if *off < req.PageAfter.Offset {
				*off++
				return true
			}
		}
		return ts < req.PageAfter.TS
	}

	offsetInMS := 0
	start := req.Start
	end := req.End
	if strings.Contains(req.Query, "_time:") {
		start, end = 0, 0
	}
	res, err := h.logs.ExecuteQueryStream(c.Request.Context(), &models.LogsQLQueryRequest{
		Query:    req.Query,
		Start:    start,
		End:      end,
		TenantID: req.TenantID,
	}, func(row map[string]any) error {
		ts := extractTS(row)
		if skipUntilCursor(ts, &offsetInMS) {
			return nil
		}
		for k := range row {
			fieldsSet[k] = struct{}{}
		}
		rows = append(rows, row)
		count++
		afterTS = ts
		afterOff = offsetInMS + 1
		if count >= req.Limit {
			return nil
		}
		return nil
	})
	if err != nil {
		h.log.Error("search query failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}

	fields := make([]string, 0, len(fieldsSet))
	for k := range fieldsSet {
		fields = append(fields, k)
	}

	var next *models.PageCursor
	if len(rows) == req.Limit {
		next = &models.PageCursor{TS: afterTS, Offset: afterOff}
	}

	c.JSON(http.StatusOK, models.LogsSearchResponse{
		Rows:          rows,
		Fields:        fields,
		NextPageAfter: next,
		Stats:         map[string]any{"count": len(rows), "streaming_stats": res.Stats},
	})
}

// helpers
func extractTS(row map[string]any) int64 {
	// try common keys in VictoriaLogs: _time (ms), ts, timestamp
	if v, ok := row["_time"]; ok {
		if ms := toInt64(v); ms > 0 {
			return ms
		}
	}
	if v, ok := row["ts"]; ok {
		if ms := toInt64(v); ms > 0 {
			return ms
		}
	}
	if v, ok := row["timestamp"]; ok {
		if ms := toInt64(v); ms > 0 {
			return ms
		}
	}
	return 0
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case float64:
		return fmt.Sprintf("%.0f", x)
	case float32:
		return fmt.Sprintf("%.0f", x)
	case int, int64, int32, uint64, uint32, uint:
		return fmt.Sprintf("%v", x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		i, _ := x.Int64()
		return i
	case string:
		// try parse
		if i, err := json.Number(x).Int64(); err == nil {
			return i
		}
	}
	return 0
}
