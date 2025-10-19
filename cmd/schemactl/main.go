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
	flag.StringVar(&mode, "mode", "dump", "mode: dump|restore")
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
	default:
		fmt.Fprintln(os.Stderr, "unknown mode")
		os.Exit(1)
	}
}
