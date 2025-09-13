package repo

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	storageweaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
)

// WeaviateRepo implements SchemaStore using Weaviate objects + GraphQL queries.
type WeaviateRepo struct {
	t       storageweaviate.Transport
	mu      sync.Mutex
	ensured bool
}

func stringArray(name string) map[string]any {
	return map[string]any{"name": name, "dataType": []string{"text[]"}}
}

func NewWeaviateRepo(c *storageweaviate.Client) *WeaviateRepo {
	return &WeaviateRepo{t: storageweaviate.NewHTTPTransport(c)}
}

func NewWeaviateRepoFromTransport(t storageweaviate.Transport) *WeaviateRepo {
	return &WeaviateRepo{t: t}
}

/* ------------------------------- primitives ------------------------------ */

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

// doJSON removed; handled by transport

// ensureClass creates a single class, checking if it exists first
func (r *WeaviateRepo) ensureClass(ctx context.Context, className string, classDef map[string]any) error {
	// Check if class already exists by attempting to get its schema
	exists, err := r.classExists(ctx, className)
	if err != nil {
		return fmt.Errorf("failed to check if class %s exists: %w", className, err)
	}

	if exists {
		// Class already exists, skip creation
		return nil
	}

	// Create the class
	if err := r.t.EnsureClasses(ctx, []map[string]any{classDef}); err != nil {
		return fmt.Errorf("failed to create class %s: %w", className, err)
	}

	return nil
}

// classExists checks if a class exists using REST API instead of GraphQL
func (r *WeaviateRepo) classExists(ctx context.Context, className string) (bool, error) {
	var schema struct {
		Classes []struct {
			Class string `json:"class"`
		} `json:"classes"`
	}

	// Use the Transport interface method instead of type assertion
	if err := r.t.GetSchema(ctx, &schema); err != nil {
		// If we can't get the schema, assume class doesn't exist
		// This handles the case where Weaviate is running but has no schema
		if strings.Contains(err.Error(), "422") || strings.Contains(err.Error(), "no schema") {
			return false, nil
		}
		return false, err
	}

	// Check if our class exists in the schema
	for _, class := range schema.Classes {
		if class.Class == className {
			return true, nil
		}
	}

	return false, nil
}

// Helper builders for schema classes
func class(name string, props map[string]any) map[string]any {
	return map[string]any{
		"class":           name,
		"vectorizer":      "none",
		"vectorIndexType": "hnsw",
		"moduleConfig":    map[string]any{},
		"properties":      props["properties"],
	}
}
func props(items ...map[string]any) map[string]any { return map[string]any{"properties": items} }
func text(name string) map[string]any {
	return map[string]any{"name": name, "dataType": []string{"text"}}
}
func intp(name string) map[string]any {
	return map[string]any{"name": name, "dataType": []string{"int"}}
}
func boolp(name string) map[string]any {
	return map[string]any{"name": name, "dataType": []string{"boolean"}}
}
func date(name string) map[string]any {
	return map[string]any{"name": name, "dataType": []string{"date"}}
}
func refp(name, to string) map[string]any {
	return map[string]any{"name": name, "dataType": []string{to}}
}

// object returns a property definition for a free-form object.
// Weaviate requires nestedProperties for object types; using an empty list allows empty objects like {}
// and lets you evolve the nested keys later (or keep the object unindexed).
func object(name string) map[string]any {
	// Weaviate requires at least one nestedProperty for object/object[] types.
	// Provide a permissive placeholder property "note" of type text to satisfy schema constraints
	// while allowing flexible payloads/tags/examples storage.
	return map[string]any{
		"name":             name,
		"dataType":         []string{"object"},
		"nestedProperties": []any{map[string]any{"name": "note", "dataType": []string{"text"}}},
	}
}

// Label version payload with all required fields
func labelVersionPayload() map[string]any {
	return map[string]any{
		"name":     "payload",
		"dataType": []string{"object"},
		"nestedProperties": []any{
			map[string]any{"name": "tenantId", "dataType": []string{"text"}},
			map[string]any{"name": "metric", "dataType": []string{"text"}},
			map[string]any{"name": "name", "dataType": []string{"text"}},
			map[string]any{"name": "type", "dataType": []string{"text"}},
			map[string]any{"name": "required", "dataType": []string{"boolean"}},
			map[string]any{
				"name":     "allowedValues",
				"dataType": []string{"object"},
				"nestedProperties": []any{
					map[string]any{"name": "note", "dataType": []string{"text"}},
				},
			},
			map[string]any{"name": "definition", "dataType": []string{"text"}},
			map[string]any{"name": "updatedAt", "dataType": []string{"date"}},
		},
	}
}

func (r *WeaviateRepo) gql(ctx context.Context, query string, variables map[string]any, out any) error {
	return r.t.GraphQL(ctx, query, variables, out)
}

func (r *WeaviateRepo) putObject(ctx context.Context, class, id string, props map[string]any) error {
	// Ensure schema exists once per process (cheap no-op if already present)
	r.ensureOnce(ctx)
	if err := r.t.PutObject(ctx, class, id, props); err != nil {
		// If class is missing, try to (re)ensure schema once, then retry
		msg := err.Error()
		if strings.Contains(msg, "class \"") && strings.Contains(msg, "not found") {
			_ = r.EnsureSchema(ctx)
			return r.t.PutObject(ctx, class, id, props)
		}
		return err
	}
	return nil
}

// ensureOnce runs EnsureSchema only once per repo lifetime.
func (r *WeaviateRepo) ensureOnce(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ensured {
		return
	}

	fmt.Println("DEBUG: ensureOnce called, running EnsureSchema")
	if err := r.EnsureSchema(ctx); err != nil {
		fmt.Printf("DEBUG: EnsureSchema failed in ensureOnce: %v\n", err)
		return
	}

	r.ensured = true
	fmt.Println("DEBUG: ensureOnce completed successfully")
}

/* -------------------------------- metrics -------------------------------- */

func (r *WeaviateRepo) UpsertMetric(ctx context.Context, m MetricDef, author string) error {
	// compute next version for this metric
	next, _ := r.maxVersion(ctx, "MetricVersion", map[string]string{"tenantId": m.TenantID, "name": m.Metric})
	nowRFC := time.Now().UTC().Format(time.RFC3339Nano)
	props := map[string]any{
		"tenantId":   m.TenantID,
		"name":       m.Metric,
		"definition": m.Description,
		"owner":      m.Owner,
		"tags":       m.Tags,
		"unit":       "",
		"source":     "",
		"version":    next,
		"updatedAt":  nowRFC,
	}
	id := makeID("Metric", m.TenantID, m.Metric)
	if err := r.putObject(ctx, "Metric", id, props); err != nil {
		return err
	}
	payload := map[string]any{"tenantId": m.TenantID, "name": m.Metric, "definition": m.Description, "owner": m.Owner, "tags": m.Tags, "unit": "", "source": "", "updatedAt": time.Now()}
	vid := makeID("MetricVersion", m.TenantID, m.Metric, fmt.Sprintf("%d", next))
	vprops := map[string]any{"tenantId": m.TenantID, "name": m.Metric, "version": next, "payload": payload, "author": author, "createdAt": nowRFC}
	return r.putObject(ctx, "MetricVersion", vid, vprops)
}

func (r *WeaviateRepo) GetMetric(ctx context.Context, tenantID, metric string) (*MetricDef, error) {
	q := `query($tenant:String!,$name:String!){ Get { Metric(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["name"],operator:Equal,valueString:$name}]}, limit:1){ name definition owner tags updatedAt }}}`
	var resp struct {
		Data struct {
			Get struct{ Metric []map[string]any }
		}
	}
	if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "name": metric}, &resp); err != nil {
		return nil, err
	}
	arr := resp.Data.Get.Metric
	if len(arr) == 0 {
		return nil, fmt.Errorf("not found")
	}
	it := arr[0]
	var tags map[string]any
	if m, ok := it["tags"].(map[string]any); ok {
		tags = m
	}
	var updated time.Time
	if s, ok := it["updatedAt"].(string); ok {
		updated, _ = time.Parse(time.RFC3339Nano, s)
	}
	return &MetricDef{TenantID: tenantID, Metric: metric, Description: it["definition"].(string), Owner: it["owner"].(string), Tags: tags, UpdatedAt: updated}, nil
}

/* -------------------------------- labels --------------------------------- */

func (r *WeaviateRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*MetricLabelDef, error) {
	if len(labels) == 0 {
		return map[string]*MetricLabelDef{}, nil
	}
	// Build OR operands for labels
	ops := make([]map[string]any, 0, len(labels))
	for _, l := range labels {
		ops = append(ops, map[string]any{"path": []string{"name"}, "operator": "Equal", "valueString": l})
	}
	// GraphQL JSON variables simplify assembling the where clause
	q := `query($tenant:String!,$metric:String!,$ops:[WhereFilter!]!){ Get { Label(where:{operator:And,operands:[{path:[\"tenantId\"],operator:Equal,valueString:$tenant},{path:[\"metric\"],operator:Equal,valueString:$metric},{operator:Or,operands:$ops}]}){ name type required allowedValues definition } } }`
	vars := map[string]any{"tenant": tenantID, "metric": metric, "ops": ops}
	var resp struct {
		Data struct {
			Get struct{ Label []map[string]any }
		}
	}
	if err := r.gql(ctx, q, vars, &resp); err != nil {
		return nil, err
	}
	out := map[string]*MetricLabelDef{}
	for _, it := range resp.Data.Get.Label {
		var allowed map[string]any
		if m, ok := it["allowedValues"].(map[string]any); ok {
			allowed = m
		}
		out[it["name"].(string)] = &MetricLabelDef{TenantID: tenantID, Metric: metric, Label: it["name"].(string), Type: it["type"].(string), Required: it["required"].(bool), AllowedVals: allowed, Description: it["definition"].(string)}
	}
	return out, nil
}

func (r *WeaviateRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	next, _ := r.maxVersion(ctx, "LabelVersion", map[string]string{"tenantId": tenantID, "metric": metric, "name": label})
	nowRFC := time.Now().UTC().Format(time.RFC3339Nano)
	props := map[string]any{
		"tenantId":      tenantID,
		"metric":        metric,
		"name":          label,
		"type":          typ,
		"required":      required,
		"allowedValues": allowed,
		"definition":    description,
		"version":       next,
		"updatedAt":     nowRFC,
	}
	id := makeID("Label", tenantID, metric, label)
	if err := r.putObject(ctx, "Label", id, props); err != nil {
		return err
	}
	payload := map[string]any{"tenantId": tenantID, "metric": metric, "name": label, "type": typ, "required": required, "allowedValues": allowed, "definition": description, "updatedAt": time.Now()}
	vid := makeID("LabelVersion", tenantID, metric, label, fmt.Sprintf("%d", next))
	vprops := map[string]any{"tenantId": tenantID, "metric": metric, "name": label, "version": next, "payload": payload, "author": "", "createdAt": nowRFC}
	return r.putObject(ctx, "LabelVersion", vid, vprops)
}

/* --------------------------------- logs ---------------------------------- */

func (r *WeaviateRepo) UpsertLogField(ctx context.Context, f LogFieldDef, author string) error {
	next, _ := r.maxVersion(ctx, "LogFieldVersion", map[string]string{"tenantId": f.TenantID, "name": f.Field})
	nowRFC := time.Now().UTC().Format(time.RFC3339Nano)
	props := map[string]any{"tenantId": f.TenantID, "name": f.Field, "type": f.Type, "definition": f.Description, "tags": f.Tags, "examples": f.Examples, "version": next, "updatedAt": nowRFC}
	id := makeID("LogField", f.TenantID, f.Field)
	if err := r.putObject(ctx, "LogField", id, props); err != nil {
		return err
	}
	vid := makeID("LogFieldVersion", f.TenantID, f.Field, fmt.Sprintf("%d", next))
	vprops := map[string]any{"tenantId": f.TenantID, "name": f.Field, "version": next, "payload": map[string]any{"tenantId": f.TenantID, "name": f.Field, "type": f.Type, "definition": f.Description, "tags": f.Tags, "examples": f.Examples, "updatedAt": time.Now()}, "author": author, "createdAt": nowRFC}
	return r.putObject(ctx, "LogFieldVersion", vid, vprops)
}

func (r *WeaviateRepo) GetLogField(ctx context.Context, tenantID, field string) (*LogFieldDef, error) {
	q := `query($tenant:String!,$name:String!){ Get { LogField(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["name"],operator:Equal,valueString:$name}]}, limit:1){ name type definition tags examples updatedAt } } }`
	var resp struct {
		Data struct {
			Get struct{ LogField []map[string]any }
		}
	}
	if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "name": field}, &resp); err != nil {
		return nil, err
	}
	arr := resp.Data.Get.LogField
	if len(arr) == 0 {
		return nil, fmt.Errorf("not found")
	}
	it := arr[0]
	var tags, ex map[string]any
	if m, ok := it["tags"].(map[string]any); ok {
		tags = m
	}
	if m, ok := it["examples"].(map[string]any); ok {
		ex = m
	}
	var updated time.Time
	if s, ok := it["updatedAt"].(string); ok {
		updated, _ = time.Parse(time.RFC3339Nano, s)
	}
	return &LogFieldDef{TenantID: tenantID, Field: it["name"].(string), Type: it["type"].(string), Description: it["definition"].(string), Tags: tags, Examples: ex, UpdatedAt: updated}, nil
}

/* -------------------------------- traces --------------------------------- */

func (r *WeaviateRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, purpose, owner string, tags map[string]any, author string) error {
	next, _ := r.maxVersion(ctx, "ServiceVersion", map[string]string{"tenantId": tenantID, "name": service})
	nowRFC := time.Now().UTC().Format(time.RFC3339Nano)
	props := map[string]any{"tenantId": tenantID, "name": service, "definition": purpose, "purpose": purpose, "owner": owner, "tags": tags, "version": next, "updatedAt": nowRFC}
	id := makeID("Service", tenantID, service)
	if err := r.putObject(ctx, "Service", id, props); err != nil {
		return err
	}
	vid := makeID("ServiceVersion", tenantID, service, fmt.Sprintf("%d", next))
	vprops := map[string]any{"tenantId": tenantID, "name": service, "version": next, "payload": map[string]any{"tenantId": tenantID, "name": service, "definition": purpose, "purpose": purpose, "owner": owner, "tags": tags, "updatedAt": time.Now()}, "author": author, "createdAt": nowRFC}
	return r.putObject(ctx, "ServiceVersion", vid, vprops)
}

func (r *WeaviateRepo) GetTraceService(ctx context.Context, tenantID, service string) (*TraceServiceDef, error) {
	q := `query($tenant:String!,$name:String!){ Get { Service(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["name"],operator:Equal,valueString:$name}]}, limit:1){ purpose owner tags updatedAt } } }`
	var resp struct {
		Data struct {
			Get struct{ Service []map[string]any }
		}
	}
	if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "name": service}, &resp); err != nil {
		return nil, err
	}
	arr := resp.Data.Get.Service
	if len(arr) == 0 {
		return nil, fmt.Errorf("not found")
	}
	it := arr[0]
	var tags map[string]any
	if m, ok := it["tags"].(map[string]any); ok {
		tags = m
	}
	var updated time.Time
	if s, ok := it["updatedAt"].(string); ok {
		updated, _ = time.Parse(time.RFC3339Nano, s)
	}
	return &TraceServiceDef{TenantID: tenantID, Service: service, Purpose: it["purpose"].(string), Owner: it["owner"].(string), Tags: tags, UpdatedAt: updated}, nil
}

func (r *WeaviateRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, purpose, owner string, tags map[string]any, author string) error {
	next, _ := r.maxVersion(ctx, "OperationVersion", map[string]string{"tenantId": tenantID, "service": service, "name": operation})
	nowRFC := time.Now().UTC().Format(time.RFC3339Nano)
	props := map[string]any{"tenantId": tenantID, "service": service, "name": operation, "definition": purpose, "purpose": purpose, "owner": owner, "tags": tags, "version": next, "updatedAt": nowRFC}
	id := makeID("Operation", tenantID, service, operation)
	if err := r.putObject(ctx, "Operation", id, props); err != nil {
		return err
	}
	vid := makeID("OperationVersion", tenantID, service, operation, fmt.Sprintf("%d", next))
	vprops := map[string]any{"tenantId": tenantID, "service": service, "name": operation, "version": next, "payload": map[string]any{"tenantId": tenantID, "service": service, "name": operation, "definition": purpose, "purpose": purpose, "owner": owner, "tags": tags, "updatedAt": time.Now()}, "author": author, "createdAt": nowRFC}
	return r.putObject(ctx, "OperationVersion", vid, vprops)
}

func (r *WeaviateRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*TraceOperationDef, error) {
	q := `query($tenant:String!,$service:String!,$name:String!){ Get { Operation(where:{operator:And,operands:[{path:["tenantId"],operator:Equal,valueString:$tenant},{path:["service"],operator:Equal,valueString:$service},{path:["name"],operator:Equal,valueString:$name}]}, limit:1){ purpose owner tags updatedAt } } }`
	var resp struct {
		Data struct {
			Get struct{ Operation []map[string]any }
		}
	}
	if err := r.gql(ctx, q, map[string]any{"tenant": tenantID, "service": service, "name": operation}, &resp); err != nil {
		return nil, err
	}
	arr := resp.Data.Get.Operation
	if len(arr) == 0 {
		return nil, fmt.Errorf("not found")
	}
	it := arr[0]
	var tags map[string]any
	if m, ok := it["tags"].(map[string]any); ok {
		tags = m
	}
	var updated time.Time
	if s, ok := it["updatedAt"].(string); ok {
		updated, _ = time.Parse(time.RFC3339Nano, s)
	}
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
	return r.listVersions(ctx, "MetricVersion", map[string]string{"tenantId": tenantID, "name": metric})
}
func (r *WeaviateRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]VersionInfo, error) {
	return r.listVersions(ctx, "LogFieldVersion", map[string]string{"tenantId": tenantID, "name": field})
}
func (r *WeaviateRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]VersionInfo, error) {
	return r.listVersions(ctx, "ServiceVersion", map[string]string{"tenantId": tenantID, "name": service})
}
func (r *WeaviateRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]VersionInfo, error) {
	return r.listVersions(ctx, "OperationVersion", map[string]string{"tenantId": tenantID, "service": service, "name": operation})
}

func (r *WeaviateRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, VersionInfo, error) {
	return r.getVersion(ctx, "MetricVersion", map[string]string{"tenantId": tenantID, "name": metric, "version": fmt.Sprintf("%d", version)})
}
func (r *WeaviateRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, VersionInfo, error) {
	return r.getVersion(ctx, "LogFieldVersion", map[string]string{"tenantId": tenantID, "name": field, "version": fmt.Sprintf("%d", version)})
}
func (r *WeaviateRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, VersionInfo, error) {
	return r.getVersion(ctx, "ServiceVersion", map[string]string{"tenantId": tenantID, "name": service, "version": fmt.Sprintf("%d", version)})
}
func (r *WeaviateRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, VersionInfo, error) {
	return r.getVersion(ctx, "OperationVersion", map[string]string{"tenantId": tenantID, "service": service, "name": operation, "version": fmt.Sprintf("%d", version)})
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
	var resp struct {
		Data struct {
			Get map[string][]gqlVersionRow `json:"Get"`
		}
	}
	if err := r.gql(ctx, query, map[string]any{"ops": ops}, &resp); err != nil {
		return nil, err
	}
	// reflect-less access: pull the only array present
	var rows []gqlVersionRow
	for _, v := range resp.Data.Get {
		rows = v
		break
	}
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
	var resp struct {
		Data struct{ Get map[string][]gqlVersionRow }
	}
	if err := r.gql(ctx, query, map[string]any{"ops": ops}, &resp); err != nil {
		return nil, VersionInfo{}, err
	}
	var rows []gqlVersionRow
	for _, v := range resp.Data.Get {
		rows = v
		break
	}
	if len(rows) == 0 {
		return nil, VersionInfo{}, fmt.Errorf("not found")
	}
	row := rows[0]
	var payload map[string]any
	if m, ok := row.Payload.(map[string]any); ok {
		payload = m
	}
	t, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
	return payload, VersionInfo{Version: row.Version, Author: row.Author, CreatedAt: t}, nil
}

func (r *WeaviateRepo) maxVersion(ctx context.Context, class string, eq map[string]string) (int64, error) {
	ops := make([]map[string]any, 0, len(eq))
	for path, val := range eq {
		ops = append(ops, map[string]any{"path": []string{path}, "operator": "Equal", "valueString": val})
	}
	query := fmt.Sprintf("query($ops:[WhereFilter!]!){ Get { %s(where:{operator:And,operands:$ops}, sort:[{path:[\"version\"], order: desc}], limit:1){ version } } }", class)
	var resp struct {
		Data struct {
			Get map[string][]struct{ Version int64 }
		}
	}
	_ = r.gql(ctx, query, map[string]any{"ops": ops}, &resp)
	var rows []struct{ Version int64 }
	for _, v := range resp.Data.Get {
		rows = v
		break
	}
	if len(rows) == 0 {
		return 1, nil
	}
	return rows[0].Version + 1, nil
}

// New Code
// Updated Weaviate schema functions in internal/repo/schema_weaviate.go

// stringDict returns a property definition for a dictionary of strings (key-value pairs).
// This replaces the generic object() function for tags fields specifically.
func stringDict(name string) map[string]any {
	return map[string]any{
		"name":     name,
		"dataType": []string{"object"},
		"nestedProperties": []any{
			// Allow any string key with string value - Weaviate will accept dynamic keys
			map[string]any{"name": "_key", "dataType": []string{"text"}},
			map[string]any{"name": "_value", "dataType": []string{"text"}},
		},
	}
}

// Service version payload with string dictionary tags
func serviceVersionPayload() map[string]any {
	return map[string]any{
		"name":     "payload",
		"dataType": []string{"object"},
		"nestedProperties": []any{
			map[string]any{"name": "tenantId", "dataType": []string{"text"}},
			map[string]any{"name": "name", "dataType": []string{"text"}},
			map[string]any{"name": "definition", "dataType": []string{"text"}},
			map[string]any{"name": "purpose", "dataType": []string{"text"}},
			map[string]any{"name": "owner", "dataType": []string{"text"}},
			stringArray("tags"), // Use string dictionary for tags
			map[string]any{"name": "updatedAt", "dataType": []string{"date"}},
		},
	}
}

// Operation version payload with string dictionary tags
func operationVersionPayload() map[string]any {
	return map[string]any{
		"name":     "payload",
		"dataType": []string{"object"},
		"nestedProperties": []any{
			map[string]any{"name": "tenantId", "dataType": []string{"text"}},
			map[string]any{"name": "service", "dataType": []string{"text"}},
			map[string]any{"name": "name", "dataType": []string{"text"}},
			map[string]any{"name": "definition", "dataType": []string{"text"}},
			map[string]any{"name": "purpose", "dataType": []string{"text"}},
			map[string]any{"name": "owner", "dataType": []string{"text"}},
			stringArray("tags"), // Use string dictionary for tags
			map[string]any{"name": "updatedAt", "dataType": []string{"date"}},
		},
	}
}

// Metric version payload with string dictionary tags
func metricVersionPayload() map[string]any {
	return map[string]any{
		"name":     "payload",
		"dataType": []string{"object"},
		"nestedProperties": []any{
			map[string]any{"name": "tenantId", "dataType": []string{"text"}},
			map[string]any{"name": "name", "dataType": []string{"text"}},
			map[string]any{"name": "definition", "dataType": []string{"text"}},
			map[string]any{"name": "owner", "dataType": []string{"text"}},
			stringArray("tags"), // Use string dictionary for tags
			map[string]any{"name": "unit", "dataType": []string{"text"}},
			map[string]any{"name": "source", "dataType": []string{"text"}},
			map[string]any{"name": "updatedAt", "dataType": []string{"date"}},
		},
	}
}

// Log field version payload with string dictionary tags and examples
func logFieldVersionPayload() map[string]any {
	return map[string]any{
		"name":     "payload",
		"dataType": []string{"object"},
		"nestedProperties": []any{
			map[string]any{"name": "tenantId", "dataType": []string{"text"}},
			map[string]any{"name": "name", "dataType": []string{"text"}},
			map[string]any{"name": "type", "dataType": []string{"text"}},
			map[string]any{"name": "definition", "dataType": []string{"text"}},
			stringArray("tags"),    // Use string dictionary for tags
			stringDict("examples"), // Use string dictionary for examples
			map[string]any{"name": "updatedAt", "dataType": []string{"date"}},
		},
	}
}

// Update the main classes to use stringDict for tags as well
func (r *WeaviateRepo) EnsureSchema(ctx context.Context) error {
	// Define all classes that need to be created
	classDefinitions := []struct {
		name string
		def  map[string]any
	}{
		// Primary classes first (referenced by version classes)
		{"Label", class("Label", props(
			text("tenantId"), text("metric"), text("name"), text("definition"), text("type"),
			boolp("required"), object("allowedValues"), intp("version"), date("updatedAt"),
		))},
		{"Metric", class("Metric", props(
			text("tenantId"), text("name"), text("definition"), text("owner"), stringArray("tags"), // Updated to stringDict
			text("unit"), text("source"), intp("version"), date("updatedAt"),
			refp("labels", "Label"),
		))},
		{"LogField", class("LogField", props(
			text("tenantId"), text("name"), text("type"), text("definition"),
			stringArray("tags"), stringDict("examples"), intp("version"), date("updatedAt"), // Updated to stringDict
		))},
		{"Service", class("Service", props(
			text("tenantId"), text("name"), text("definition"), text("purpose"), text("owner"), stringArray("tags"), intp("version"), date("updatedAt"), // Updated to stringDict
		))},
		{"Operation", class("Operation", props(
			text("tenantId"), text("service"), text("name"), text("definition"), text("purpose"), text("owner"), stringArray("tags"), intp("version"), date("updatedAt"), // Updated to stringDict
		))},
		// Version classes with proper payload schemas
		{"MetricVersion", class("MetricVersion", props(text("tenantId"), text("name"), intp("version"), metricVersionPayload(), text("author"), date("createdAt")))},
		{"LabelVersion", class("LabelVersion", props(text("tenantId"), text("metric"), text("name"), intp("version"), labelVersionPayload(), text("author"), date("createdAt")))},
		{"LogFieldVersion", class("LogFieldVersion", props(text("tenantId"), text("name"), intp("version"), logFieldVersionPayload(), text("author"), date("createdAt")))},
		{"ServiceVersion", class("ServiceVersion", props(text("tenantId"), text("name"), intp("version"), serviceVersionPayload(), text("author"), date("createdAt")))},
		{"OperationVersion", class("OperationVersion", props(text("tenantId"), text("service"), text("name"), intp("version"), operationVersionPayload(), text("author"), date("createdAt")))},
	}

	// Create classes individually to better handle failures
	for _, classDef := range classDefinitions {
		if err := r.ensureClass(ctx, classDef.name, classDef.def); err != nil {
			return fmt.Errorf("failed to create class %s: %w", classDef.name, err)
		}
	}

	return nil
}
