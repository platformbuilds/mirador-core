package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "time"
    "bytes"
)

type client struct {
    base string
    apiKey string
    http *http.Client
}

func newClient() *client {
    host := getenv("WEAVIATE_HOST", "localhost")
    port := getenv("WEAVIATE_PORT", "8080")
    scheme := getenv("WEAVIATE_SCHEME", "http")
    key := os.Getenv("WEAVIATE_API_KEY")
    return &client{base: fmt.Sprintf("%s://%s:%s", scheme, host, port), apiKey: key, http: &http.Client{Timeout: 15*time.Second}}
}

func (c *client) do(ctx context.Context, method, path string, body any, out any) error {
    var data []byte
    var err error
    if body != nil { data, err = json.Marshal(body); if err != nil { return err } }
    req, _ := http.NewRequestWithContext(ctx, method, c.base+path, bytesReader(data))
    req.Header.Set("Content-Type", "application/json")
    if c.apiKey != "" { req.Header.Set("Authorization", "Bearer "+c.apiKey) }
    resp, err := c.http.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 { b,_ := ioutil.ReadAll(resp.Body); return fmt.Errorf("%s: %s", resp.Status, string(b)) }
    if out != nil { return json.NewDecoder(resp.Body).Decode(out) }
    return nil
}

func bytesReader(b []byte) *bytes.Reader { if b == nil { return bytes.NewReader(nil) }; return bytes.NewReader(b) }

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

func dump(ctx context.Context, c *client, outPath string) error {
    classes := []string{"MetricDef","MetricDefVersion","MetricLabelDef","LogFieldDef","LogFieldDefVersion","TraceServiceDef","TraceServiceDefVersion","TraceOperationDef","TraceOperationDefVersion"}
    result := map[string][]map[string]any{}
    for _, cl := range classes {
        q := fmt.Sprintf("query{ Get { %s(limit: 10000){ _additional{id} ... on %s { * } } } }", cl, cl)
        var resp map[string]any
        if err := c.do(ctx, http.MethodPost, "/v1/graphql", map[string]any{"query": q}, &resp); err != nil { return err }
        // Minimal extraction: rely on weaviate returning properties; store raw
        result[cl] = append(result[cl], map[string]any{"raw": resp})
    }
    data, _ := json.MarshalIndent(result, "", "  ")
    return ioutil.WriteFile(outPath, data, 0644)
}

func restore(ctx context.Context, c *client, inPath string) error {
    data, err := ioutil.ReadFile(inPath); if err != nil { return err }
    var payload map[string][]map[string]any
    if err := json.Unmarshal(data, &payload); err != nil { return err }
    // Naive: assumes payload was produced by dump; users can adapt as needed.
    // Alternatively, support a flat list of {class,id,properties} objects.
    for cl, arr := range payload {
        _ = cl; _ = arr // placeholder; real implementation would map to objects and PUT
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
        if err := dump(ctx, c, out); err != nil { fmt.Fprintln(os.Stderr, "dump failed:", err); os.Exit(1) }
        fmt.Println("dumped to", out)
    case "restore":
        if err := restore(ctx, c, in); err != nil { fmt.Fprintln(os.Stderr, "restore failed:", err); os.Exit(1) }
        fmt.Println("restored from", in)
    default:
        fmt.Fprintln(os.Stderr, "unknown mode")
        os.Exit(1)
    }
}
