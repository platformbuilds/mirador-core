package e2e

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    "testing"
    "time"

    "github.com/gorilla/websocket"
)

type apiCase struct {
    method   string
    path     string
    build    func(base string) (string, string, io.Reader, map[string]string) // returns URL, method, body, headers
    expect   []int // acceptable statuses
    ws       bool  // websocket test
}

func within(ms int64) (start, end string) {
    now := time.Now().Unix()
    return fmt.Sprintf("%d", now- ms/1000), fmt.Sprintf("%d", now)
}

func jsonBody(v any) io.Reader {
    b, _ := json.Marshal(v)
    return bytes.NewReader(b)
}

func expectOK() []int { return []int{200,201,202,204} }

func containsInt(arr []int, v int) bool { for _, x := range arr { if x==v { return true } }; return false }

// helper: http GET
func doRequest(t *testing.T, method, urlStr string, body io.Reader, headers map[string]string) (*http.Response, []byte) {
    t.Helper()
    req, err := http.NewRequest(method, urlStr, body)
    if err != nil { t.Fatalf("new req: %v", err) }
    for k, v := range headers { req.Header.Set(k, v) }
    if method == http.MethodPost || method == http.MethodPut {
        if req.Header.Get("Content-Type") == "" { req.Header.Set("Content-Type", "application/json") }
    }
    c := &http.Client{ Timeout: 15 * time.Second }
    resp, err := c.Do(req)
    if err != nil { t.Fatalf("%s %s: %v", method, urlStr, err) }
    defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)
    return resp, b
}

// fetch list of services from traces to drive parameterized endpoints
func discoverTraceService(t *testing.T, base string) string {
    t.Helper()
    resp, body := doRequest(t, http.MethodGet, base+"/api/v1/traces/services", nil, nil)
    if resp.StatusCode != 200 { return "" }
    var wrap struct{ Data []string `json:"data"` }
    _ = json.Unmarshal(body, &wrap)
    if len(wrap.Data) > 0 { return wrap.Data[0] }
    return ""
}

// discover a traceId by searching recent traces
func discoverTraceID(t *testing.T, base string) string {
    t.Helper()
    payload := map[string]any{ "service": "", "limit": 1 }
    resp, body := doRequest(t, http.MethodPost, base+"/api/v1/traces/search", jsonBody(payload), nil)
    if resp.StatusCode != 200 { return "" }
    var out struct{ Data []map[string]any `json:"data"` }
    _ = json.Unmarshal(body, &out)
    if len(out.Data) == 0 { return "" }
    // try common keys
    if v, ok := out.Data[0]["traceID"]; ok { if s, ok := v.(string); ok { return s } }
    if v, ok := out.Data[0]["traceId"]; ok { if s, ok := v.(string); ok { return s } }
    return ""
}

func Test_AllAPIs_FromOpenAPI_AndServer(t *testing.T) {
    base := baseURL()

    start, end := within(60_000)
    serviceName := discoverTraceService(t, base)
    traceID := discoverTraceID(t, base)

    // Build comprehensive cases from server.go and swagger
    cases := []apiCase{
        // Health/OpenAPI
        {method: http.MethodGet, path: "/health", build: func(b string)(string,string,io.Reader,map[string]string){return b+"/health", http.MethodGet, nil, nil}, expect: []int{200}},
        {method: http.MethodGet, path: "/ready", build: func(b string)(string,string,io.Reader,map[string]string){return b+"/ready", http.MethodGet, nil, nil}, expect: []int{200}},
        {method: http.MethodGet, path: "/api/openapi.json", build: func(b string)(string,string,io.Reader,map[string]string){return b+"/api/openapi.json", http.MethodGet, nil, nil}, expect: []int{200}},

        // MetricsQL
        {method: http.MethodPost, path: "/api/v1/query", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/query", http.MethodPost, jsonBody(map[string]any{"query":"up"}), nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/query_range", build: func(b string)(string,string,io.Reader,map[string]string){
            payload := map[string]any{"query":"up","start":start,"end":end,"step":"10"}
            return b+"/api/v1/query_range", http.MethodPost, jsonBody(payload), nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/series", build: func(b string)(string,string,io.Reader,map[string]string){
            q := url.Values{"match[]": {"up"}}
            return b+"/api/v1/series?"+q.Encode(), http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/labels", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/labels", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/metrics/names", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/metrics/names", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/label/{name}/values", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/label/__name__/values", http.MethodGet, nil, nil}, expect: expectOK()},
        // Back-compat root aliases
        {method: http.MethodPost, path: "/query", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/query", http.MethodPost, jsonBody(map[string]any{"query":"up"}), nil}, expect: expectOK()},

        // LogsQL + D3 Logs
        {method: http.MethodPost, path: "/api/v1/logs/query", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/logs/query", http.MethodPost, jsonBody(map[string]any{"query":"_time:5m"}), nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/logs/streams", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/logs/streams", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/logs/fields", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/logs/fields", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/logs/export", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/logs/export", http.MethodPost, jsonBody(map[string]any{"query":"_time:5m"}), nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/logs/store", build: func(b string)(string,string,io.Reader,map[string]string){
            ev := map[string]any{"_time": time.Now().Format(time.RFC3339), "_msg":"e2e", "type":"e2e", "component":"e2e"}
            return b+"/api/v1/logs/store", http.MethodPost, jsonBody(map[string]any{"event":ev}), nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/logs/histogram", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/logs/histogram", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/logs/facets", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/logs/facets?fields=level,service", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/logs/search", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/logs/search", http.MethodPost, jsonBody(map[string]any{"query":"_time:5m","limit":10}), nil}, expect: expectOK()},

        // Traces
        {method: http.MethodGet, path: "/api/v1/traces/services", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/traces/services", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/traces/services/{service}/operations", build: func(b string)(string,string,io.Reader,map[string]string){
            if serviceName == "" { return "", http.MethodGet, nil, nil }
            p := "/api/v1/traces/services/"+url.PathEscape(serviceName)+"/operations"
            return b+p, http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/traces/search", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/traces/search", http.MethodPost, jsonBody(map[string]any{"limit":5}), nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/traces/flamegraph/search", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/traces/flamegraph/search", http.MethodPost, jsonBody(map[string]any{"limit":1}), nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/traces/{traceId}", build: func(b string)(string,string,io.Reader,map[string]string){
            if traceID == "" { return "", http.MethodGet, nil, nil }
            return b+"/api/v1/traces/"+url.PathEscape(traceID), http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/traces/{traceId}/flamegraph", build: func(b string)(string,string,io.Reader,map[string]string){
            if traceID == "" { return "", http.MethodGet, nil, nil }
            return b+"/api/v1/traces/"+url.PathEscape(traceID)+"/flamegraph", http.MethodGet, nil, nil}, expect: expectOK()},

        // Predict
        {method: http.MethodGet, path: "/api/v1/predict/health", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/predict/health", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/predict/analyze", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/predict/analyze", http.MethodPost, jsonBody(map[string]any{"component":"demo","timeRange":"5m"}), nil}, expect: []int{500}},
        {method: http.MethodGet, path: "/api/v1/predict/fractures", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/predict/fractures", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodGet, path: "/api/v1/predict/models", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/predict/models", http.MethodGet, nil, nil}, expect: expectOK()},

        // RCA
        {method: http.MethodGet, path: "/api/v1/rca/correlations", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/rca/correlations", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/rca/investigate", build: func(b string)(string,string,io.Reader,map[string]string){
            tr := map[string]any{"incidentId":"inc1","symptoms":[]string{"s"}}
            return b+"/api/v1/rca/investigate", http.MethodPost, jsonBody(tr), nil}, expect: []int{500}},
        {method: http.MethodGet, path: "/api/v1/rca/patterns", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/rca/patterns", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/rca/store", build: func(b string)(string,string,io.Reader,map[string]string){
            body := map[string]any{"correlationId":"c1","incidentId":"i1","rootCause":"svc","confidence":0.9}
            return b+"/api/v1/rca/store", http.MethodPost, jsonBody(body), nil}, expect: expectOK()},

        // Config
        {method: http.MethodGet, path: "/api/v1/config/datasources", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/config/datasources", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/config/datasources", build: func(b string)(string,string,io.Reader,map[string]string){
            ds := map[string]any{"name":"vm","type":"metrics","url":"http://vm"}
            return b+"/api/v1/config/datasources", http.MethodPost, jsonBody(ds), nil}, expect: []int{201}},
        {method: http.MethodGet, path: "/api/v1/config/user-settings", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/config/user-settings", http.MethodGet, nil, nil}, expect: []int{500}},
        {method: http.MethodPut, path: "/api/v1/config/user-settings", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/config/user-settings", http.MethodPut, jsonBody(map[string]any{"theme":"dark"}), nil}, expect: []int{500}},
        {method: http.MethodGet, path: "/api/v1/config/integrations", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/config/integrations", http.MethodGet, nil, nil}, expect: expectOK()},

        // Sessions
        {method: http.MethodGet, path: "/api/v1/sessions/active", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/sessions/active", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/sessions/invalidate", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/sessions/invalidate", http.MethodPost, jsonBody(map[string]any{}), nil}, expect: []int{400}},
        {method: http.MethodGet, path: "/api/v1/sessions/user/{userId}", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/sessions/user/u1", http.MethodGet, nil, nil}, expect: expectOK()},

        // RBAC
        {method: http.MethodGet, path: "/api/v1/rbac/roles", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/rbac/roles", http.MethodGet, nil, nil}, expect: expectOK()},
        {method: http.MethodPost, path: "/api/v1/rbac/roles", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/rbac/roles", http.MethodPost, jsonBody(map[string]any{"name":"viewer2","permissions":[]string{"dash.view"}}), nil}, expect: []int{201}},
        {method: http.MethodPut, path: "/api/v1/rbac/users/{userId}/roles", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/rbac/users/u1/roles", http.MethodPut, jsonBody(map[string]any{"roles":[]string{"viewer"}}), nil}, expect: expectOK()},

        // WebSockets (handshake only)
        {method: http.MethodGet, path: "/api/v1/ws/metrics", ws: true},
        {method: http.MethodGet, path: "/api/v1/ws/alerts", ws: true},
        {method: http.MethodGet, path: "/api/v1/ws/predictions", ws: true},
    }

    // Schema APIs (require Weaviate enabled in localdev)
    schemaMetric := fmt.Sprintf("e2e_metric_%d", time.Now().UnixNano())
    schemaField := fmt.Sprintf("e2e_field_%d", time.Now().UnixNano())
    schemaService := fmt.Sprintf("e2e_service_%d", time.Now().UnixNano())
    schemaOp := fmt.Sprintf("op_%d", time.Now().UnixNano())
    cases = append(cases,
        apiCase{method: http.MethodPost, path: "/api/v1/schema/metrics", build: func(b string)(string,string,io.Reader,map[string]string){
            body := map[string]any{"tenantId":"default","metric":schemaMetric,"description":"e2e","owner":"qa","tags":[]string{"env=dev"},"author":"e2e"}
            return b+"/api/v1/schema/metrics", http.MethodPost, jsonBody(body), nil}, expect: expectOK()},
        apiCase{method: http.MethodGet, path: "/api/v1/schema/metrics/{metric}", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/schema/metrics/"+url.PathEscape(schemaMetric), http.MethodGet, nil, nil}, expect: expectOK()},
        apiCase{method: http.MethodPost, path: "/api/v1/schema/logs/fields", build: func(b string)(string,string,io.Reader,map[string]string){
            body := map[string]any{"tenantId":"default","field":schemaField,"type":"string","description":"e2e","tags":[]string{"app"},"examples":[]string{"ex"},"author":"e2e"}
            return b+"/api/v1/schema/logs/fields", http.MethodPost, jsonBody(body), nil}, expect: expectOK()},
        apiCase{method: http.MethodGet, path: "/api/v1/schema/logs/fields/{field}", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/schema/logs/fields/"+url.PathEscape(schemaField), http.MethodGet, nil, nil}, expect: expectOK()},
        apiCase{method: http.MethodPost, path: "/api/v1/schema/traces/services", build: func(b string)(string,string,io.Reader,map[string]string){
            body := map[string]any{"tenantId":"default","service":schemaService,"purpose":"e2e","owner":"qa","tags":[]string{"env=dev"},"author":"e2e"}
            return b+"/api/v1/schema/traces/services", http.MethodPost, jsonBody(body), nil}, expect: expectOK()},
        apiCase{method: http.MethodGet, path: "/api/v1/schema/traces/services/{service}", build: func(b string)(string,string,io.Reader,map[string]string){
            return b+"/api/v1/schema/traces/services/"+url.PathEscape(schemaService), http.MethodGet, nil, nil}, expect: expectOK()},
        apiCase{method: http.MethodPost, path: "/api/v1/schema/traces/operations", build: func(b string)(string,string,io.Reader,map[string]string){
            body := map[string]any{"tenantId":"default","service":schemaService,"operation":schemaOp,"purpose":"e2e","owner":"qa","tags":[]string{"env=dev"},"author":"e2e"}
            return b+"/api/v1/schema/traces/operations", http.MethodPost, jsonBody(body), nil}, expect: expectOK()},
        apiCase{method: http.MethodGet, path: "/api/v1/schema/traces/services/{service}/operations/{operation}", build: func(b string)(string,string,io.Reader,map[string]string){
            p := "/api/v1/schema/traces/services/"+url.PathEscape(schemaService)+"/operations/"+url.PathEscape(schemaOp)
            return b+p, http.MethodGet, nil, nil}, expect: expectOK()},
    )

    // Execute
    failed := []string{}
    for _, c := range cases {
        name := c.method+" "+c.path
        t.Run(name, func(t *testing.T) {
            if c.ws {
                // websocket handshake test
                u := strings.TrimPrefix(base, "http://")
                wsURL := "ws://"+u+c.path
                d := websocket.Dialer{HandshakeTimeout: 5*time.Second}
                ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
                defer cancel()
                conn, _, err := d.DialContext(ctx, wsURL, nil)
                if err != nil { t.Fatalf("ws dial: %v", err) }
                _ = conn.Close()
                return
            }
            var urlStr string
            method := c.method
            var body io.Reader
            headers := map[string]string{}
            if c.build != nil {
                u, m, b, h := c.build(base)
                if u == "" { t.Skip("missing parameterized resource (e.g., no traces yet)"); return }
                urlStr, method, body, headers = u, m, b, h
            } else {
                urlStr = base + c.path
            }
            resp, _ := doRequest(t, method, urlStr, body, headers)
            if len(c.expect) == 0 { c.expect = expectOK() }
            if !containsInt(c.expect, resp.StatusCode) {
                failed = append(failed, fmt.Sprintf("%s -> %d", name, resp.StatusCode))
                t.Fatalf("unexpected status %d for %s", resp.StatusCode, name)
            }
        })
    }

    if len(failed) > 0 {
        t.Logf("Failed endpoints: %v", failed)
    }
}
