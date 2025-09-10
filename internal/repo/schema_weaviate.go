package repo

import (
    "bytes"
    "context"
    "crypto/sha1"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"

    storageweaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
)

// WeaviateRepo implements SchemaStore using Weaviate objects + GraphQL queries.
type WeaviateRepo struct {
    c *storageweaviate.Client
}

func NewWeaviateRepo(c *storageweaviate.Client) *WeaviateRepo { return &WeaviateRepo{c: c} }

/* ------------------------------- primitives ------------------------------ */

func makeID(parts ...string) string {
    h := sha1.Sum([]byte(strings.Join(parts, "|")))
    return hex.EncodeToString(h[:])
}

func (r *WeaviateRepo) doJSON(ctx context.Context, method, path string, body any, out any) error {
    var buf *bytes.Reader
    if body != nil {
        b, _ := json.Marshal(body)
        buf = bytes.NewReader(b)
    } else {
        buf = bytes.NewReader(nil)
    }
    req, _ := http.NewRequestWithContext(ctx, method, r.c.BaseURL+path, buf)
    req.Header.Set("Content-Type", "application/json")
    if r.c.APIKey != "" { req.Header.Set("Authorization", "Bearer "+r.c.APIKey) }
    resp, err := r.c.HTTP.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        return fmt.Errorf("weaviate %s %s failed: %s", method, path, resp.Status)
    }
    if out != nil {
        return json.NewDecoder(resp.Body).Decode(out)
    }
    return nil
}

// EnsureSchema creates the required classes if they don't exist.
func (r *WeaviateRepo) EnsureSchema(ctx context.Context) error {
    // fetch existing
    var cur struct{ Classes []struct{ Class string } `json:"classes"` }
    if err := r.doJSON(ctx, http.MethodGet, "/v1/schema", nil, &cur); err != nil {
        // If schema endpoint is disabled, skip
        return nil
    }
    have := map[string]struct{}{}
    for _, c := range cur.Classes { have[c.Class] = struct{}{} }
    need := []map[string]any{
        class("MetricDef", props(text("tenantId"), text("metric"), text("description"), text("owner"), object("tags"), date("updatedAt"))),
        class("MetricDefVersion", props(text("tenantId"), text("metric"), intp("version"), object("payload"), text("author"), date("createdAt"))),
        class("MetricLabelDef", props(text("tenantId"), text("metric"), text("label"), text("type"), boolp("required"), object("allowedValues"), text("description"))),
        class("LogFieldDef", props(text("tenantId"), text("field"), text("type"), text("description"), object("tags"), object("examples"), date("updatedAt"))),
        class("LogFieldDefVersion", props(text("tenantId"), text("field"), intp("version"), object("payload"), text("author"), date("createdAt"))),
        class("TraceServiceDef", props(text("tenantId"), text("service"), text("purpose"), text("owner"), object("tags"), date("updatedAt"))),
        class("TraceServiceDefVersion", props(text("tenantId"), text("service"), intp("version"), object("payload"), text("author"), date("createdAt"))),
        class("TraceOperationDef", props(text("tenantId"), text("service"), text("operation"), text("purpose"), text("owner"), object("tags"), date("updatedAt"))),
        class("TraceOperationDefVersion", props(text("tenantId"), text("service"), text("operation"), intp("version"), object("payload"), text("author"), date("createdAt"))),
    }
    for _, def := range need {
        cname := def["class"].(string)
        if _, ok := have[cname]; ok { continue }
        if err := r.doJSON(ctx, http.MethodPost, "/v1/schema", def, nil); err != nil {
            // Continue creating others; return first error at end
            return err
        }
    }
    return nil
}

// Helper builders for schema classes
func class(name string, props map[string]any) map[string]any {
    return map[string]any{
        "class":            name,
        "vectorizer":       "none",
        "vectorIndexType":  "hnsw",
        "moduleConfig":     map[string]any{},
        "properties":       props["properties"],
    }
}
func props(items ...map[string]any) map[string]any { return map[string]any{"properties": items} }
func text(name string) map[string]any { return map[string]any{"name": name, "dataType": []string{"text"}} }
func intp(name string) map[string]any { return map[string]any{"name": name, "dataType": []string{"int"}} }
func boolp(name string) map[string]any { return map[string]any{"name": name, "dataType": []string{"boolean"}} }
func date(name string) map[string]any { return map[string]any{"name": name, "dataType": []string{"date"}} }
func object(name string) map[string]any { return map[string]any{"name": name, "dataType": []string{"object"}} }

func (r *WeaviateRepo) gql(ctx context.Context, query string, variables map[string]any, out any) error {
    payload := map[string]any{"query": query}
    if len(variables) > 0 { payload["variables"] = variables }
    return r.doJSON(ctx, http.MethodPost, "/v1/graphql", payload, out)
}

func (r *WeaviateRepo) putObject(ctx context.Context, class, id string, props map[string]any) error {
    payload := map[string]any{
        "class":      class,
        "id":         id,
        "properties": props,
    }
    return r.doJSON(ctx, http.MethodPut, "/v1/objects/"+id, payload, nil)
}

/* -------------------------------- metrics -------------------------------- */

func (r *WeaviateRepo) UpsertMetric(ctx context.Context, m MetricDef, author string) error {
    props := map[string]any{
        "tenantId":    m.TenantID,
        "metric":      m.Metric,
        "description": m.Description,
        "owner":       m.Owner,
        "tags":        m.Tags,
        "updatedAt":   time.Now().UTC().Format(time.RFC3339Nano),
    }
    id := makeID("MetricDef", m.TenantID, m.Metric)
    if err := r.putObject(ctx, "MetricDef", id, props); err != nil { return err }
    // version row: compute next version
    next, _ := r.maxVersion(ctx, "MetricDefVersion", map[string]string{"tenantId": m.TenantID, "metric": m.Metric})
    payload := map[string]any{"tenantId": m.TenantID, "metric": m.Metric, "description": m.Description, "owner": m.Owner, "tags": m.Tags, "updatedAt": time.Now()}
    vid := makeID("MetricDefVersion", m.TenantID, m.Metric, fmt.Sprintf("%d", next))
    vprops := map[string]any{"tenantId": m.TenantID, "metric": m.Metric, "version": next, "payload": payload, "author": author, "createdAt": time.Now().UTC().Format(time.RFC3339Nano)}
    return r.putObject(ctx, "MetricDefVersion", vid, vprops)
}

func (r *WeaviateRepo) GetMetric(ctx context.Context, tenantID, metric string) (*MetricDef, error) {
    q := `query($tenant:String!,$metric:String!){ Get { MetricDef(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["metric"],operator:Equal,valueString:$metric}]}, limit:1){ tenantId metric description owner tags updatedAt }}}`
    var resp struct{ Data struct{ Get struct{ MetricDef []map[string]any } } }
    if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "metric": metric}, &resp); err != nil { return nil, err }
    arr := resp.Data.Get.MetricDef
    if len(arr) == 0 { return nil, fmt.Errorf("not found") }
    it := arr[0]
    var tags map[string]any
    if m, ok := it["tags"].(map[string]any); ok { tags = m }
    var updated time.Time
    if s, ok := it["updatedAt"].(string); ok { updated, _ = time.Parse(time.RFC3339Nano, s) }
    return &MetricDef{TenantID: tenantID, Metric: metric, Description: it["description"].(string), Owner: it["owner"].(string), Tags: tags, UpdatedAt: updated}, nil
}

/* -------------------------------- labels --------------------------------- */

func (r *WeaviateRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*MetricLabelDef, error) {
    if len(labels) == 0 { return map[string]*MetricLabelDef{}, nil }
    // Build OR operands for labels
    ops := make([]map[string]any, 0, len(labels))
    for _, l := range labels {
        ops = append(ops, map[string]any{"path": []string{"label"}, "operator": "Equal", "valueString": l})
    }
    // GraphQL JSON variables simplify assembling the where clause
    q := `query($tenant:String!,$metric:String!,$ops:[WhereFilter!]!){ Get { MetricLabelDef(where:{operator:And,operands:[{path:[\"tenantId\"],operator:Equal,valueString:$tenant},{path:[\"metric\"],operator:Equal,valueString:$metric},{operator:Or,operands:$ops}]}){ label type required allowedValues description } } }`
    vars := map[string]any{"tenant": tenantID, "metric": metric, "ops": ops}
    var resp struct{ Data struct{ Get struct{ MetricLabelDef []map[string]any } } }
    if err := r.gql(ctx, q, vars, &resp); err != nil { return nil, err }
    out := map[string]*MetricLabelDef{}
    for _, it := range resp.Data.Get.MetricLabelDef {
        var allowed map[string]any
        if m, ok := it["allowedValues"].(map[string]any); ok { allowed = m }
        out[it["label"].(string)] = &MetricLabelDef{TenantID: tenantID, Metric: metric, Label: it["label"].(string), Type: it["type"].(string), Required: it["required"].(bool), AllowedVals: allowed, Description: it["description"].(string)}
    }
    return out, nil
}

func (r *WeaviateRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
    props := map[string]any{
        "tenantId":     tenantID,
        "metric":       metric,
        "label":        label,
        "type":         typ,
        "required":     required,
        "allowedValues": allowed,
        "description":  description,
    }
    id := makeID("MetricLabelDef", tenantID, metric, label)
    return r.putObject(ctx, "MetricLabelDef", id, props)
}

/* --------------------------------- logs ---------------------------------- */

func (r *WeaviateRepo) UpsertLogField(ctx context.Context, f LogFieldDef, author string) error {
    props := map[string]any{"tenantId": f.TenantID, "field": f.Field, "type": f.Type, "description": f.Description, "tags": f.Tags, "examples": f.Examples, "updatedAt": time.Now().UTC().Format(time.RFC3339Nano)}
    id := makeID("LogFieldDef", f.TenantID, f.Field)
    if err := r.putObject(ctx, "LogFieldDef", id, props); err != nil { return err }
    next, _ := r.maxVersion(ctx, "LogFieldDefVersion", map[string]string{"tenantId": f.TenantID, "field": f.Field})
    vid := makeID("LogFieldDefVersion", f.TenantID, f.Field, fmt.Sprintf("%d", next))
    vprops := map[string]any{"tenantId": f.TenantID, "field": f.Field, "version": next, "payload": map[string]any{"tenantId": f.TenantID, "field": f.Field, "type": f.Type, "description": f.Description, "tags": f.Tags, "examples": f.Examples, "updatedAt": time.Now()}, "author": author, "createdAt": time.Now().UTC().Format(time.RFC3339Nano)}
    return r.putObject(ctx, "LogFieldDefVersion", vid, vprops)
}

func (r *WeaviateRepo) GetLogField(ctx context.Context, tenantID, field string) (*LogFieldDef, error) {
    q := `query($tenant:String!,$field:String!){ Get { LogFieldDef(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["field"],operator:Equal,valueString:$field}]}, limit:1){ field type description tags examples updatedAt } } }`
    var resp struct{ Data struct{ Get struct{ LogFieldDef []map[string]any } } }
    if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "field": field}, &resp); err != nil { return nil, err }
    arr := resp.Data.Get.LogFieldDef
    if len(arr) == 0 { return nil, fmt.Errorf("not found") }
    it := arr[0]
    var tags, ex map[string]any
    if m, ok := it["tags"].(map[string]any); ok { tags = m }
    if m, ok := it["examples"].(map[string]any); ok { ex = m }
    var updated time.Time
    if s, ok := it["updatedAt"].(string); ok { updated, _ = time.Parse(time.RFC3339Nano, s) }
    return &LogFieldDef{TenantID: tenantID, Field: it["field"].(string), Type: it["type"].(string), Description: it["description"].(string), Tags: tags, Examples: ex, UpdatedAt: updated}, nil
}

/* -------------------------------- traces --------------------------------- */

func (r *WeaviateRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, purpose, owner string, tags map[string]any, author string) error {
    props := map[string]any{"tenantId": tenantID, "service": service, "purpose": purpose, "owner": owner, "tags": tags, "updatedAt": time.Now().UTC().Format(time.RFC3339Nano)}
    id := makeID("TraceServiceDef", tenantID, service)
    if err := r.putObject(ctx, "TraceServiceDef", id, props); err != nil { return err }
    next, _ := r.maxVersion(ctx, "TraceServiceDefVersion", map[string]string{"tenantId": tenantID, "service": service})
    vid := makeID("TraceServiceDefVersion", tenantID, service, fmt.Sprintf("%d", next))
    vprops := map[string]any{"tenantId": tenantID, "service": service, "version": next, "payload": map[string]any{"tenantId": tenantID, "service": service, "purpose": purpose, "owner": owner, "tags": tags, "updatedAt": time.Now()}, "author": author, "createdAt": time.Now().UTC().Format(time.RFC3339Nano)}
    return r.putObject(ctx, "TraceServiceDefVersion", vid, vprops)
}

func (r *WeaviateRepo) GetTraceService(ctx context.Context, tenantID, service string) (*TraceServiceDef, error) {
    q := `query($tenant:String!,$service:String!){ Get { TraceServiceDef(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["service"],operator:Equal,valueString:$service}]}, limit:1){ purpose owner tags updatedAt } } }`
    var resp struct{ Data struct{ Get struct{ TraceServiceDef []map[string]any } } }
    if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "service": service}, &resp); err != nil { return nil, err }
    arr := resp.Data.Get.TraceServiceDef
    if len(arr) == 0 { return nil, fmt.Errorf("not found") }
    it := arr[0]
    var tags map[string]any
    if m, ok := it["tags"].(map[string]any); ok { tags = m }
    var updated time.Time
    if s, ok := it["updatedAt"].(string); ok { updated, _ = time.Parse(time.RFC3339Nano, s) }
    return &TraceServiceDef{TenantID: tenantID, Service: service, Purpose: it["purpose"].(string), Owner: it["owner"].(string), Tags: tags, UpdatedAt: updated}, nil
}

func (r *WeaviateRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, purpose, owner string, tags map[string]any, author string) error {
    props := map[string]any{"tenantId": tenantID, "service": service, "operation": operation, "purpose": purpose, "owner": owner, "tags": tags, "updatedAt": time.Now().UTC().Format(time.RFC3339Nano)}
    id := makeID("TraceOperationDef", tenantID, service, operation)
    if err := r.putObject(ctx, "TraceOperationDef", id, props); err != nil { return err }
    next, _ := r.maxVersion(ctx, "TraceOperationDefVersion", map[string]string{"tenantId": tenantID, "service": service, "operation": operation})
    vid := makeID("TraceOperationDefVersion", tenantID, service, operation, fmt.Sprintf("%d", next))
    vprops := map[string]any{"tenantId": tenantID, "service": service, "operation": operation, "version": next, "payload": map[string]any{"tenantId": tenantID, "service": service, "operation": operation, "purpose": purpose, "owner": owner, "tags": tags, "updatedAt": time.Now()}, "author": author, "createdAt": time.Now().UTC().Format(time.RFC3339Nano)}
    return r.putObject(ctx, "TraceOperationDefVersion", vid, vprops)
}

func (r *WeaviateRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*TraceOperationDef, error) {
    q := `query($tenant:String!,$service:String!,$op:String!){ Get { TraceOperationDef(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["service"],operator:Equal,valueString:$service},{path:["operation"],operator:Equal,valueString:$op}]}, limit:1){ purpose owner tags updatedAt } } }`
    var resp struct{ Data struct{ Get struct{ TraceOperationDef []map[string]any } } }
    if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "service": service, "op": operation}, &resp); err != nil { return nil, err }
    arr := resp.Data.Get.TraceOperationDef
    if len(arr) == 0 { return nil, fmt.Errorf("not found") }
    it := arr[0]
    var tags map[string]any
    if m, ok := it["tags"].(map[string]any); ok { tags = m }
    var updated time.Time
    if s, ok := it["updatedAt"].(string); ok { updated, _ = time.Parse(time.RFC3339Nano, s) }
    return &TraceOperationDef{TenantID: tenantID, Service: service, Operation: operation, Purpose: it["purpose"].(string), Owner: it["owner"].(string), Tags: tags, UpdatedAt: updated}, nil
}

/* ------------------------------ versions I/O ----------------------------- */

type gqlVersionRow struct {
    Version   int64  `json:"version"`
    Author    string `json:"author"`
    CreatedAt string `json:"createdAt"`
    Payload   any    `json:"payload"`
}

func (r *WeaviateRepo) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]VersionInfo, error) {
    return r.listVersions(ctx, "MetricDefVersion", map[string]string{"tenantId": tenantID, "metric": metric})
}
func (r *WeaviateRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]VersionInfo, error) {
    return r.listVersions(ctx, "LogFieldDefVersion", map[string]string{"tenantId": tenantID, "field": field})
}
func (r *WeaviateRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]VersionInfo, error) {
    return r.listVersions(ctx, "TraceServiceDefVersion", map[string]string{"tenantId": tenantID, "service": service})
}
func (r *WeaviateRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]VersionInfo, error) {
    return r.listVersions(ctx, "TraceOperationDefVersion", map[string]string{"tenantId": tenantID, "service": service, "operation": operation})
}

func (r *WeaviateRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, VersionInfo, error) {
    return r.getVersion(ctx, "MetricDefVersion", map[string]string{"tenantId": tenantID, "metric": metric, "version": fmt.Sprintf("%d", version)})
}
func (r *WeaviateRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, VersionInfo, error) {
    return r.getVersion(ctx, "LogFieldDefVersion", map[string]string{"tenantId": tenantID, "field": field, "version": fmt.Sprintf("%d", version)})
}
func (r *WeaviateRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, VersionInfo, error) {
    return r.getVersion(ctx, "TraceServiceDefVersion", map[string]string{"tenantId": tenantID, "service": service, "version": fmt.Sprintf("%d", version)})
}
func (r *WeaviateRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, VersionInfo, error) {
    return r.getVersion(ctx, "TraceOperationDefVersion", map[string]string{"tenantId": tenantID, "service": service, "operation": operation, "version": fmt.Sprintf("%d", version)})
}

func (r *WeaviateRepo) listVersions(ctx context.Context, class string, eq map[string]string) ([]VersionInfo, error) {
    // Build where operands from eq map
    ops := make([]map[string]any, 0, len(eq))
    for path, val := range eq {
        if path == "version" {
            // string value; weaviate handles numeric compare with valueInt
            if n, err := strconv.ParseInt(val, 10, 64); err == nil {
                ops = append(ops, map[string]any{"path": []string{path}, "operator": "Equal", "valueInt": n})
            } else {
                ops = append(ops, map[string]any{"path": []string{path}, "operator": "Equal", "valueString": val})
            }
        } else {
            ops = append(ops, map[string]any{"path": []string{path}, "operator": "Equal", "valueString": val})
        }
    }
    query := fmt.Sprintf("query($ops:[WhereFilter!]!){ Get { %s(where:{operator:And,operands:$ops}, sort:[{path:[\"version\"], order: desc}], limit:1000){ version author createdAt } } }", class)
    var resp struct{ Data struct{ Get map[string][]gqlVersionRow `json:"Get"` } }
    if err := r.gql(ctx, query, map[string]any{"ops": ops}, &resp); err != nil { return nil, err }
    // reflect-less access: pull the only array present
    var rows []gqlVersionRow
    for _, v := range resp.Data.Get { rows = v; break }
    out := make([]VersionInfo, 0, len(rows))
    for _, row := range rows {
        t, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
        out = append(out, VersionInfo{Version: row.Version, Author: row.Author, CreatedAt: t})
    }
    return out, nil
}

func (r *WeaviateRepo) getVersion(ctx context.Context, class string, eq map[string]string) (map[string]any, VersionInfo, error) {
    ops := make([]map[string]any, 0, len(eq))
    for path, val := range eq {
        if path == "version" {
            // val provided as string; convert to int64 when possible
            ival := any(val)
            if n, err := strconv.ParseInt(val, 10, 64); err == nil {
                ival = n
            }
            ops = append(ops, map[string]any{"path": []string{path}, "operator": "Equal", "valueInt": ival})
        } else {
            ops = append(ops, map[string]any{"path": []string{path}, "operator": "Equal", "valueString": val})
        }
    }
    query := fmt.Sprintf("query($ops:[WhereFilter!]!){ Get { %s(where:{operator:And,operands:$ops}, limit:1){ version author createdAt payload } } }", class)
    var resp struct{ Data struct{ Get map[string][]gqlVersionRow } }
    if err := r.gql(ctx, query, map[string]any{"ops": ops}, &resp); err != nil { return nil, VersionInfo{}, err }
    var rows []gqlVersionRow
    for _, v := range resp.Data.Get { rows = v; break }
    if len(rows) == 0 { return nil, VersionInfo{}, fmt.Errorf("not found") }
    row := rows[0]
    var payload map[string]any
    if m, ok := row.Payload.(map[string]any); ok { payload = m }
    t, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
    return payload, VersionInfo{Version: row.Version, Author: row.Author, CreatedAt: t}, nil
}

func (r *WeaviateRepo) maxVersion(ctx context.Context, class string, eq map[string]string) (int64, error) {
    ops := make([]map[string]any, 0, len(eq))
    for path, val := range eq {
        ops = append(ops, map[string]any{"path": []string{path}, "operator": "Equal", "valueString": val})
    }
    query := fmt.Sprintf("query($ops:[WhereOperator!]!){ Get { %s(where:{operator:And,operands:$ops}, sort:[{path:[\"version\"], order: desc}], limit:1){ version } } }", class)
    var resp struct{ Data struct{ Get map[string][]struct{ Version int64 } } }
    _ = r.gql(ctx, query, map[string]any{"ops": ops}, &resp)
    var rows []struct{ Version int64 }
    for _, v := range resp.Data.Get { rows = v; break }
    if len(rows) == 0 { return 1, nil }
    return rows[0].Version + 1, nil
}
