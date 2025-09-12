package weaviate

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
)

// httpTransport implements Transport using the lightweight HTTP client.
type httpTransport struct{
    c *Client
}

func NewHTTPTransport(c *Client) Transport { return &httpTransport{c: c} }

func (t *httpTransport) Ready(ctx context.Context) error { return t.c.Ready(ctx) }

func (t *httpTransport) EnsureClasses(ctx context.Context, classDefs []map[string]any) error {
    // GET /v1/schema then POST each missing class
    var cur struct{ Classes []struct{ Class string } `json:"classes"` }
    if err := t.doJSON(ctx, http.MethodGet, "/v1/schema", nil, &cur); err != nil { return err }
    have := map[string]struct{}{}
    for _, c := range cur.Classes { have[c.Class] = struct{}{} }
    for _, def := range classDefs {
        if name, _ := def["class"].(string); name != "" {
            if _, ok := have[name]; ok { continue }
        }
        // Weaviate expects POST /v1/schema/classes to create a single class
        if err := t.doJSON(ctx, http.MethodPost, "/v1/schema/classes", def, nil); err != nil { return err }
    }
    return nil
}

func (t *httpTransport) PutObject(ctx context.Context, class, id string, props map[string]any) error {
    payload := map[string]any{"class": class, "id": id, "properties": props}
    // First try PUT (replace). Some Weaviate versions return 500 when object doesn't exist.
    if err := t.doJSON(ctx, http.MethodPut, "/v1/objects/"+id, payload, nil); err != nil {
        // Fallback: if object not found for PUT, try POST (create)
        msg := err.Error()
        if strings.Contains(msg, "no object with id") || strings.Contains(msg, "404") {
            return t.doJSON(ctx, http.MethodPost, "/v1/objects", payload, nil)
        }
        // Some versions expect POST for create; if PUT failed with 422, try POST as well
        if strings.Contains(msg, "422") {
            if e2 := t.doJSON(ctx, http.MethodPost, "/v1/objects", payload, nil); e2 == nil {
                return nil
            }
        }
        return err
    }
    return nil
}

func (t *httpTransport) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
    body := map[string]any{"query": query}
    if len(variables) > 0 { body["variables"] = variables }
    return t.doJSON(ctx, http.MethodPost, "/v1/graphql", body, out)
}

func (t *httpTransport) doJSON(ctx context.Context, method, path string, body any, out any) error {
    var buf *bytes.Reader
    if body != nil {
        b, _ := json.Marshal(body)
        buf = bytes.NewReader(b)
    } else {
        buf = bytes.NewReader(nil)
    }
    req, _ := http.NewRequestWithContext(ctx, method, t.c.BaseURL+path, buf)
    req.Header.Set("Content-Type", "application/json")
    if t.c.APIKey != "" { req.Header.Set("Authorization", "Bearer "+t.c.APIKey) }
    resp, err := t.c.HTTP.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        msg := strings.TrimSpace(string(b))
        if msg == "" { msg = "<empty body>" }
        return fmt.Errorf("weaviate %s %s failed: %s: %s", method, path, resp.Status, msg)
    }
    if out != nil { return json.NewDecoder(resp.Body).Decode(out) }
    return nil
}
