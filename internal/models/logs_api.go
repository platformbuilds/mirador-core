package models

// Generic LogSQL query
func (r *LogsQLQueryRequest) GetExtra() map[string]string { // used by service
	if r.Extra == nil {
		return map[string]string{}
	}
	return r.Extra
}

// Histogram
type LogsHistogramRequest struct {
	Query    string `form:"query" json:"query"`
	Start    int64  `form:"start" json:"start"`
	End      int64  `form:"end" json:"end"`
	Step     int64  `form:"step" json:"step"`         // bucket width in ms
	Sampling int    `form:"sampling" json:"sampling"` // 0 or 1 = no sampling; N = keep 1/N rows
	TenantID string `form:"tenantId" json:"tenantId"`
	Limit    int    `form:"limit" json:"limit"` // optional server guard
}

type HistogramBucket struct {
	TS    int64 `json:"ts"`    // bucket start
	Count int   `json:"count"` // rows in bucket
}

type LogsHistogramResponse struct {
	Buckets []HistogramBucket `json:"buckets"`
	Stats   map[string]any    `json:"stats,omitempty"`
	Sampled bool              `json:"sampled"`
}

// Facets
type LogsFacetsRequest struct {
	Query    string   `form:"query" json:"query"`
	Start    int64    `form:"start" json:"start"`
	End      int64    `form:"end" json:"end"`
	Fields   []string `form:"fields" json:"fields"` // e.g. service,level,host
	Limit    int      `form:"limit" json:"limit"`   // top-N per field (default 20)
	Sampling int      `form:"sampling" json:"sampling"`
	TenantID string   `form:"tenantId" json:"tenantId"`
}

type FacetBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type Facet struct {
	Field   string        `json:"field"`
	Buckets []FacetBucket `json:"buckets"`
}

type LogsFacetsResponse struct {
	Facets  []Facet        `json:"facets"`
	Stats   map[string]any `json:"stats,omitempty"`
	Sampled bool           `json:"sampled"`
}

// Search (paged)
type LogsSearchRequest struct {
	Query     string      `json:"query"`
	Start     int64       `json:"start"`
	End       int64       `json:"end"`
	Limit     int         `json:"limit"` // rows per page
	PageAfter *PageCursor `json:"page_after,omitempty"`
	TenantID  string      `json:"tenantId"`
}

type PageCursor struct {
	TS     int64 `json:"ts"`     // last row timestamp (ms)
	Offset int   `json:"offset"` // tie-breaker if same ms
}

type LogsSearchResponse struct {
	Rows          []map[string]any `json:"rows"`
	Fields        []string         `json:"fields,omitempty"`
	NextPageAfter *PageCursor      `json:"next_page_after,omitempty"`
	Stats         map[string]any   `json:"stats,omitempty"`
}
