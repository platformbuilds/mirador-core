package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

func (c *client) do(ctx context.Context, method, path string, body, out any) error {
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

// UUID v5 (SHA-1) namespace for deterministic IDs
var nsMirador = mustParseUUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8") // URL namespace (stable)

func makeID(parts ...string) string {
	name := strings.Join(parts, "|")
	return uuidV5(nsMirador, name)
}

func mustParseUUID(s string) [16]byte {
	b, ok := parseUUID(s)
	if !ok {
		panic("invalid UUID namespace: " + s)
	}
	return b
}

func parseUUID(s string) ([16]byte, bool) {
	var out [16]byte
	// remove hyphens
	hex := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			continue
		}
		hex = append(hex, s[i])
	}
	if len(hex) != 32 {
		return out, false
	}
	// convert hex to bytes
	for i := 0; i < 16; i++ {
		hi := fromHex(hex[2*i])
		lo := fromHex(hex[2*i+1])
		if hi < 0 || lo < 0 {
			return out, false
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, true
}

func fromHex(b byte) int {
	switch {
	case '0' <= b && b <= '9':
		return int(b - '0')
	case 'a' <= b && b <= 'f':
		return int(b - 'a' + 10)
	case 'A' <= b && b <= 'F':
		return int(b - 'A' + 10)
	default:
		return -1
	}
}

func uuidV5(ns [16]byte, name string) string {
	// RFC 4122, version 5: SHA-1 of namespace + name
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	sum := h.Sum(nil) // 20 bytes
	var u [16]byte
	copy(u[:], sum[:16])
	// Set version (5) in high nibble of byte 6
	u[6] = (u[6] & 0x0f) | (5 << 4)
	// Set variant (RFC4122) in the two most significant bits of byte 8
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(u[0])<<24|uint32(u[1])<<16|uint32(u[2])<<8|uint32(u[3]),
		uint16(u[4])<<8|uint16(u[5]),
		uint16(u[6])<<8|uint16(u[7]),
		uint16(u[8])<<8|uint16(u[9]),
		(uint64(u[10])<<40)|(uint64(u[11])<<32)|(uint64(u[12])<<24)|(uint64(u[13])<<16)|(uint64(u[14])<<8)|uint64(u[15]),
	)
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

func seed(ctx context.Context, c *client) error {
	// Seed sample KPIs
	if err := seedSampleKPIs(ctx, c); err != nil {
		return fmt.Errorf("failed to seed sample KPIs: %w", err)
	}

	fmt.Printf("Successfully seeded data\n")
	return nil
}

func seedSampleKPIs(ctx context.Context, c *client) error {
	sampleKPIs := []map[string]interface{}{
		{
			"id":     makeID("KPIDefinition", "http_request_duration"),
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
			"visibility": "org",
			"createdAt":  time.Now().Format(time.RFC3339),
			"updatedAt":  time.Now().Format(time.RFC3339),
		},
		{
			"id":     makeID("KPIDefinition", "error_rate"),
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
			"visibility": "org",
			"createdAt":  time.Now().Format(time.RFC3339),
			"updatedAt":  time.Now().Format(time.RFC3339),
		},
	}

	for _, kpi := range sampleKPIs {
		kpiID := kpi["id"].(string)
		kpiName := kpi["name"].(string)

		// Check if KPI already exists via KPI API
		var existing struct {
			KPIDefinition map[string]interface{} `json:"kpiDefinition"`
		}
		err := c.do(ctx, http.MethodGet, "/api/v1/kpi/defs?id="+kpiID, nil, &existing)
		if err == nil && existing.KPIDefinition != nil {
			fmt.Printf("KPI %s already exists\n", kpiName)
			continue
		}

		// Create KPI via KPI API
		req := map[string]interface{}{"kpiDefinition": kpi}
		if err := c.do(ctx, http.MethodPost, "/api/v1/kpi/defs", req, nil); err != nil {
			return fmt.Errorf("failed to create KPI %s: %w", kpiName, err)
		}

		fmt.Printf("Created KPI %s\n", kpiName)
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
	flag.StringVar(&mode, "mode", "dump", "mode: dump|restore|seed")
	flag.StringVar(&out, "out", "schema_dump.json", "output file for dump")
	flag.StringVar(&in, "in", "schema_dump.json", "input file for restore")
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
		if err := seed(ctx, c); err != nil {
			fmt.Fprintln(os.Stderr, "seed failed:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown mode")
		os.Exit(1)
	}
}
