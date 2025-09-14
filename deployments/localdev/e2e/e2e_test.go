package e2e

import (
    "bytes"
    "encoding/json"
    "net/http"
    "os"
    "testing"
)

func baseURL() string {
    if v := os.Getenv("E2E_BASE_URL"); v != "" { return v }
    return "http://localhost:8080"
}

func TestHealthAndOpenAPI(t *testing.T) {
    b := baseURL()
    for _, path := range []string{"/health", "/ready", "/api/openapi.json"} {
        resp, err := http.Get(b + path)
        if err != nil { t.Fatalf("GET %s: %v", path, err) }
        if resp.StatusCode != 200 { t.Fatalf("%s status=%d", path, resp.StatusCode) }
        resp.Body.Close()
    }
}

func TestMetricsQL_Minimal(t *testing.T) {
    b := baseURL()
    // labels (GET)
    resp, err := http.Get(b + "/api/v1/labels")
    if err != nil { t.Fatalf("labels: %v", err) }
    if resp.StatusCode != 200 { t.Fatalf("labels status=%d", resp.StatusCode) }
    resp.Body.Close()
    // query (POST) with simple expression
    payload := map[string]any{"query":"up"}
    body, _ := json.Marshal(payload)
    r, err := http.Post(b+"/api/v1/query", "application/json", bytes.NewReader(body))
    if err != nil { t.Fatalf("query: %v", err) }
    if r.StatusCode != 200 { t.Fatalf("query status=%d", r.StatusCode) }
    r.Body.Close()
}

func TestLogs_Minimal(t *testing.T) {
    b := baseURL()
    // streams (GET)
    resp, err := http.Get(b + "/api/v1/logs/streams")
    if err != nil { t.Fatalf("logs/streams: %v", err) }
    if resp.StatusCode != 200 { t.Fatalf("logs/streams status=%d", resp.StatusCode) }
    resp.Body.Close()

    // histogram (GET) defaults
    resp, err = http.Get(b + "/api/v1/logs/histogram")
    if err != nil { t.Fatalf("logs/histogram: %v", err) }
    if resp.StatusCode != 200 { t.Fatalf("logs/histogram status=%d", resp.StatusCode) }
    resp.Body.Close()

    // search (POST) with minimal payload
    q := map[string]any{"query":"_stream:*","limit":10}
    body, _ := json.Marshal(q)
    r, err := http.Post(b+"/api/v1/logs/search", "application/json", bytes.NewReader(body))
    if err != nil { t.Fatalf("logs/search: %v", err) }
    if r.StatusCode != 200 { t.Fatalf("logs/search status=%d", r.StatusCode) }
    r.Body.Close()
}

