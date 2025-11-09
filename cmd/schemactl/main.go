package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type client struct {
	base   string
	apiKey string
	http   *http.Client
}

func newClient() *client {
	host := getenv("WEAVIATE_HOST", "localhost")
	port := getenv("WEAVIATE_PORT", "8080")
	scheme := getenv("WEAVIATE_SCHEME", "http")
	key := os.Getenv("WEAVIATE_API_KEY")
	return &client{base: fmt.Sprintf("%s://%s:%s", scheme, host, port), apiKey: key, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s -> %s: %s", method, path, resp.Status, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// dumpedObject is a portable representation used for restore.
type dumpedObject struct {
	Class      string         `json:"class"`
	ID         string         `json:"id"`
	Properties map[string]any `json:"properties"`
}

func dump(ctx context.Context, c *client, outPath string) error {
	// New primary and version classes
	classes := []string{
		"Metric", "Label", "LogField", "Service", "Operation",
		"MetricVersion", "LabelVersion", "LogFieldVersion", "ServiceVersion", "OperationVersion",
	}
	out := []dumpedObject{}
	for _, cl := range classes {
		// 1) list IDs via GraphQL (no need to know property names here)
		q := fmt.Sprintf("query{ Get { %s(limit: 10000){ _additional { id } } } }", cl)
		var resp struct {
			Data struct {
				Get map[string][]struct {
					Additional struct {
						ID string `json:"id"`
					} `json:"_additional"`
				}
			}
		}
		if err := c.do(ctx, http.MethodPost, "/v1/graphql", map[string]any{"query": q}, &resp); err != nil {
			return err
		}
		// pull the only array present
		var rows []struct {
			Additional struct {
				ID string `json:"id"`
			} `json:"_additional"`
		}
		for _, v := range resp.Data.Get {
			rows = v
			break
		}
		// 2) fetch each object by ID via REST
		for _, r := range rows {
			var obj struct {
				Class      string         `json:"class"`
				ID         string         `json:"id"`
				Properties map[string]any `json:"properties"`
			}
			if err := c.do(ctx, http.MethodGet, "/v1/objects/"+r.Additional.ID, nil, &obj); err != nil {
				return err
			}
			out = append(out, dumpedObject{Class: obj.Class, ID: obj.ID, Properties: obj.Properties})
		}
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return os.WriteFile(outPath, b, 0644)
}

func seed(ctx context.Context, c *client, tenantID string) error {
	// Seed default dashboard
	if err := seedDefaultDashboard(ctx, c, tenantID); err != nil {
		return fmt.Errorf("failed to seed default dashboard: %w", err)
	}

	// Seed sample KPIs
	if err := seedSampleKPIs(ctx, c, tenantID); err != nil {
		return fmt.Errorf("failed to seed sample KPIs: %w", err)
	}

	fmt.Printf("Successfully seeded data for tenant %s\n", tenantID)
	return nil
}

func seedDefaultDashboard(ctx context.Context, c *client, tenantID string) error {
	dashboard := map[string]interface{}{
		"class": "Dashboard",
		"id":    "default",
		"properties": map[string]interface{}{
			"id":          "default",
			"name":        "Default Dashboard",
			"ownerUserId": "system",
			"visibility":  "org",
			"isDefault":   true,
			"tenantId":    tenantID,
			"createdAt":   time.Now().Format(time.RFC3339),
			"updatedAt":   time.Now().Format(time.RFC3339),
		},
	}

	// Check if dashboard already exists
	var existing struct {
		Properties map[string]interface{} `json:"properties"`
	}
	err := c.do(ctx, http.MethodGet, "/v1/objects/"+dashboard["id"].(string), nil, &existing)
	if err == nil {
		fmt.Printf("Default dashboard already exists for tenant %s\n", tenantID)
		return nil
	}

	// Create dashboard
	if err := c.do(ctx, http.MethodPost, "/v1/objects", dashboard, nil); err != nil {
		return fmt.Errorf("failed to create dashboard: %w", err)
	}

	fmt.Printf("Created default dashboard for tenant %s\n", tenantID)
	return nil
}

func seedSampleKPIs(ctx context.Context, c *client, tenantID string) error {
	sampleKPIs := []map[string]interface{}{
		{
			"class": "KPIDefinition",
			"id":    "http_request_duration",
			"properties": map[string]interface{}{
				"id":     "http_request_duration",
				"kind":   "tech",
				"name":   "HTTP Request Duration",
				"unit":   "seconds",
				"format": "duration",
				"query": map[string]interface{}{
					"metric": "http_request_duration_seconds",
					"labels": map[string]interface{}{
						"method": "{{method}}",
						"status": "{{status}}",
					},
				},
				"thresholds": []map[string]interface{}{
					{
						"operator": "gt",
						"value":    1.0,
						"severity": "warning",
						"message":  "Request duration is high",
					},
					{
						"operator": "gt",
						"value":    5.0,
						"severity": "critical",
						"message":  "Request duration is critically high",
					},
				},
				"tags":       []string{"http", "performance", "latency"},
				"definition": "Average HTTP request duration across all endpoints",
				"sentiment":  "NEGATIVE",
				"sparkline": map[string]interface{}{
					"type": "line",
					"query": map[string]interface{}{
						"range": "1h",
					},
				},
				"ownerUserId": "system",
				"visibility":  "org",
				"tenantId":    tenantID,
				"createdAt":   time.Now().Format(time.RFC3339),
				"updatedAt":   time.Now().Format(time.RFC3339),
			},
		},
		{
			"class": "KPIDefinition",
			"id":    "error_rate",
			"properties": map[string]interface{}{
				"id":     "error_rate",
				"kind":   "tech",
				"name":   "Error Rate",
				"unit":   "percent",
				"format": "percentage",
				"query": map[string]interface{}{
					"metric": "http_requests_total",
					"labels": map[string]interface{}{
						"status": ">=400",
					},
					"aggregation": "rate",
				},
				"thresholds": []map[string]interface{}{
					{
						"operator": "gt",
						"value":    5.0,
						"severity": "warning",
						"message":  "Error rate is elevated",
					},
					{
						"operator": "gt",
						"value":    10.0,
						"severity": "critical",
						"message":  "Error rate is critically high",
					},
				},
				"tags":       []string{"errors", "reliability", "http"},
				"definition": "Percentage of HTTP requests that result in errors (4xx/5xx)",
				"sentiment":  "NEGATIVE",
				"sparkline": map[string]interface{}{
					"type": "area",
					"query": map[string]interface{}{
						"range": "1h",
					},
				},
				"ownerUserId": "system",
				"visibility":  "org",
				"tenantId":    tenantID,
				"createdAt":   time.Now().Format(time.RFC3339),
				"updatedAt":   time.Now().Format(time.RFC3339),
			},
		},
	}

	for _, kpi := range sampleKPIs {
		// Check if KPI already exists
		var existing struct {
			Properties map[string]interface{} `json:"properties"`
		}
		err := c.do(ctx, http.MethodGet, "/v1/objects/"+kpi["id"].(string), nil, &existing)
		if err == nil {
			fmt.Printf("KPI %s already exists for tenant %s\n", kpi["id"], tenantID)
			continue
		}

		// Create KPI
		if err := c.do(ctx, http.MethodPost, "/v1/objects", kpi, nil); err != nil {
			return fmt.Errorf("failed to create KPI %s: %w", kpi["id"], err)
		}

		fmt.Printf("Created KPI %s for tenant %s\n", kpi["id"], tenantID)
	}

	return nil
}

func restore(ctx context.Context, c *client, inPath string) error {
	b, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	var items []dumpedObject
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	for _, it := range items {
		payload := map[string]any{"class": it.Class, "id": it.ID, "properties": it.Properties}
		if err := c.do(ctx, http.MethodPut, "/v1/objects/"+it.ID, payload, nil); err != nil {
			return fmt.Errorf("restore %s/%s: %w", it.Class, it.ID, err)
		}
	}
	return nil
}

func main() {
	var out string
	var in string
	var mode string
	var tenantID string
	flag.StringVar(&mode, "mode", "dump", "mode: dump|restore|seed")
	flag.StringVar(&out, "out", "schema_dump.json", "output file for dump")
	flag.StringVar(&in, "in", "schema_dump.json", "input file for restore")
	flag.StringVar(&tenantID, "tenant", "default", "tenant ID for seeding")
	flag.Parse()
	c := newClient()
	ctx := context.Background()
	switch mode {
	case "dump":
		if err := dump(ctx, c, out); err != nil {
			fmt.Fprintln(os.Stderr, "dump failed:", err)
			os.Exit(1)
		}
		fmt.Println("dumped to", out)
	case "restore":
		if err := restore(ctx, c, in); err != nil {
			fmt.Fprintln(os.Stderr, "restore failed:", err)
			os.Exit(1)
		}
		fmt.Println("restored from", in)
	case "seed":
		if err := seed(ctx, c, tenantID); err != nil {
			fmt.Fprintln(os.Stderr, "seed failed:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown mode")
		os.Exit(1)
	}
}
