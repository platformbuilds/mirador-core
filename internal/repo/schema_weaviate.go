package repo

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
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
	start := time.Now()
	err = r.t.EnsureClasses(ctx, []map[string]any{classDef})
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("ensure_class", className, duration, false)
		return fmt.Errorf("failed to create class %s: %w", className, err)
	}

	monitoring.RecordWeaviateOperation("ensure_class", className, duration, true)
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
	start := time.Now()
	err := r.t.GetSchema(ctx, &schema)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_schema", "schema", duration, false)
		// If we can't get the schema, assume class doesn't exist
		// This handles the case where Weaviate is running but has no schema
		if strings.Contains(err.Error(), "422") || strings.Contains(err.Error(), "no schema") {
			return false, nil
		}
		return false, err
	}

	monitoring.RecordWeaviateOperation("get_schema", "schema", duration, true)

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
			map[string]any{"name": "category", "dataType": []string{"text"}},
			map[string]any{"name": "sentiment", "dataType": []string{"text"}},
			map[string]any{"name": "updatedAt", "dataType": []string{"date"}},
		},
	}
}

func (r *WeaviateRepo) gql(ctx context.Context, query string, variables map[string]any, out any) error {
	start := time.Now()
	err := r.t.GraphQL(ctx, query, variables, out)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("graphql", "query", duration, false)
		return err
	}

	monitoring.RecordWeaviateOperation("graphql", "query", duration, true)
	return nil
}

func (r *WeaviateRepo) putObject(ctx context.Context, class, id string, props map[string]any) error {
	// Ensure schema exists once per process (cheap no-op if already present)
	r.ensureOnce(ctx)
	start := time.Now()
	if err := r.t.PutObject(ctx, class, id, props); err != nil {
		// If class is missing, try to (re)ensure schema once, then retry
		msg := err.Error()
		if strings.Contains(msg, "class \"") && strings.Contains(msg, "not found") {
			_ = r.EnsureSchema(ctx)
			err = r.t.PutObject(ctx, class, id, props)
		}
		if err != nil {
			duration := time.Since(start)
			monitoring.RecordWeaviateOperation("put_object", class, duration, false)
			return err
		}
	}
	duration := time.Since(start)
	monitoring.RecordWeaviateOperation("put_object", class, duration, true)
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
		"category":   m.Category,
		"sentiment":  m.Sentiment,
		"unit":       "",
		"source":     "",
		"version":    next,
		"updatedAt":  nowRFC,
	}
	id := makeID("Metric", m.TenantID, m.Metric)
	if err := r.putObject(ctx, "Metric", id, props); err != nil {
		return err
	}
	payload := map[string]any{"tenantId": m.TenantID, "name": m.Metric, "definition": m.Description, "owner": m.Owner, "tags": m.Tags, "category": m.Category, "sentiment": m.Sentiment, "unit": "", "source": "", "updatedAt": time.Now()}
	vid := makeID("MetricVersion", m.TenantID, m.Metric, fmt.Sprintf("%d", next))
	vprops := map[string]any{"tenantId": m.TenantID, "name": m.Metric, "version": next, "payload": payload, "author": author, "createdAt": nowRFC}
	return r.putObject(ctx, "MetricVersion", vid, vprops)
}

// replace the entire GetMetric method with this version

func (r *WeaviateRepo) GetMetric(ctx context.Context, tenantID, metric string) (*MetricDef, error) {
	// Inline values (same approach as GetTraceService/GetTraceOperation)
	q := fmt.Sprintf(`{
	  Get {
	    Metric(
	      where: {
	        operator: And,
	        operands: [
	          { path: ["tenantId"], operator: Equal, valueString: "%s" },
	          { path: ["name"],     operator: Equal, valueString: "%s" }
	        ]
	      },
	      limit: 1
	    ) {
	      name
	      definition
	      owner
	      tags
	      category
	      sentiment
	      updatedAt
	    }
	  }
	}`, tenantID, metric)

	var resp struct {
		Data struct {
			Get struct {
				Metric []map[string]any `json:"Metric"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, fmt.Errorf("weaviate query failed for metric '%s' tenant '%s': %w", metric, tenantID, err)
	}

	arr := resp.Data.Get.Metric
	if len(arr) == 0 {
		return nil, fmt.Errorf("metric '%s' not found in Weaviate for tenant '%s'", metric, tenantID)
	}

	it := arr[0]

	// tags: []interface{} -> []string (same as service/operation codepaths)
	var tags []string
	if raw, ok := it["tags"].([]interface{}); ok {
		tags = make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	// updatedAt: RFC3339 -> time.Time
	var updated time.Time
	if s, ok := it["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			updated = t
		} else if t2, err2 := time.Parse(time.RFC3339, s); err2 == nil {
			updated = t2
		}
	}

	// fields map 1:1 with UpsertMetric props: name->Metric, definition->Description
	desc, _ := it["definition"].(string)
	owner, _ := it["owner"].(string)
	category, _ := it["category"].(string)
	sentiment, _ := it["sentiment"].(string)

	return &MetricDef{
		TenantID:    tenantID,
		Metric:      metric,
		Description: desc,
		Owner:       owner,
		Tags:        tags,
		Category:    category,
		Sentiment:   sentiment,
		UpdatedAt:   updated,
	}, nil
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

	props := map[string]any{
		"tenantId":   f.TenantID,
		"name":       f.Field,
		"type":       f.Type,
		"definition": f.Description,
		"tags":       f.Tags,
		"category":   f.Category,
		"sentiment":  f.Sentiment,
		"version":    next,
		"updatedAt":  nowRFC,
	}
	id := makeID("LogField", f.TenantID, f.Field)
	if err := r.putObject(ctx, "LogField", id, props); err != nil {
		return err
	}
	vid := makeID("LogFieldVersion", f.TenantID, f.Field, fmt.Sprintf("%d", next))
	vprops := map[string]any{
		"tenantId": f.TenantID,
		"name":     f.Field,
		"version":  next,
		"payload": map[string]any{
			"tenantId":   f.TenantID,
			"name":       f.Field,
			"type":       f.Type,
			"definition": f.Description,
			"tags":       f.Tags,
			"category":   f.Category,
			"sentiment":  f.Sentiment,
			"updatedAt":  nowRFC,
		},
		"author":    author,
		"createdAt": nowRFC,
	}
	return r.putObject(ctx, "LogFieldVersion", vid, vprops)
}

// GetLogField reads a single log field by (tenantID, fieldName) from Weaviate.
// It mirrors the inline GraphQL pattern used for GetMetric / GetTraceService / GetTraceOperation.
// It also tolerates repos that stored the identifier under "name" or "field".
func (r *WeaviateRepo) GetLogField(ctx context.Context, tenantID, fieldName string) (*LogFieldDef, error) {
	// Primary: EXACT GraphQL requested (tenantId + name, valueString)
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\\`, `\\\\`)
		s = strings.ReplaceAll(s, `"`, `\\\"`)
		return s
	}

	q := fmt.Sprintf(`{
      Get {
        LogField(
          where: {
            operator: And,
            operands: [
              { path: ["tenantId"], operator: Equal, valueString: "%s" },
              { path: ["name"],     operator: Equal, valueString: "%s" }
            ]
          },
          limit: 1
        ) {
          tenantId
          name
          type
          definition
          tags
          category
          sentiment
          updatedAt
          _additional { id }
        }
      }
    }`, esc(tenantID), esc(fieldName))

	var resp struct {
		Data struct {
			Get struct {
				LogField []map[string]any `json:"LogField"`
			} `json:"Get"`
		} `json:"data"`
	}
	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, fmt.Errorf("weaviate query failed for log field '%s' tenant '%s': %w", fieldName, tenantID, err)
	}

	arr := resp.Data.Get.LogField
	if len(arr) == 0 {
		// Fallback: by deterministic ID (UUIDv5 of LogField|tenant|field)
		fid := makeID("LogField", tenantID, fieldName)
		qid := fmt.Sprintf(`{
          Get {
            LogField(
              where: { path: ["id"], operator: Equal, valueString: "%s" },
              limit: 1
            ) {
              tenantId
              name
              type
              definition
              tags
              updatedAt
              _additional { id }
            }
          }
        }`, esc(fid))
		var rid struct {
			Data struct {
				Get struct {
					LogField []map[string]any `json:"LogField"`
				} `json:"Get"`
			} `json:"data"`
		}
		if err := r.gql(ctx, qid, nil, &rid); err == nil {
			arr = rid.Data.Get.LogField
		}
		if len(arr) == 0 {
			return nil, fmt.Errorf("log field '%s' not found for tenant '%s'", fieldName, tenantID)
		}
	}

	it := arr[0]

	// We store the field identifier under "name"
	gotField, _ := it["name"].(string)

	// tags: normalize from []interface{} or []string
	var tags []string
	if raw, ok := it["tags"].([]interface{}); ok {
		tags = make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok && s != "" {
				tags = append(tags, s)
			}
		}
	} else if raw2, ok := it["tags"].([]string); ok {
		tags = raw2
	}

	// type/description (support legacy "description")
	typ, _ := it["type"].(string)
	desc, _ := it["definition"].(string)
	if desc == "" {
		desc, _ = it["description"].(string)
	}

	// updatedAt: RFC3339 -> time.Time (best-effort)
	var updated time.Time
	if s, ok := it["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			updated = t
		}
	}

	return &LogFieldDef{
		TenantID:    tenantID,
		Field:       gotField,
		Type:        typ,
		Description: desc,
		Tags:        tags,
		Category:    it["category"].(string),
		Sentiment:   it["sentiment"].(string),
		UpdatedAt:   updated,
	}, nil
}

/* -------------------------------- traces --------------------------------- */

func (r *WeaviateRepo) UpsertTraceServiceWithAuthor(
	ctx context.Context,
	tenantID, service, purpose, owner, category, sentiment string,
	tags []string,
	author string,
) error {
	next, _ := r.maxVersion(ctx, "ServiceVersion", map[string]string{"tenantId": tenantID, "name": service})
	now := time.Now().UTC()
	nowRFC := now.Format(time.RFC3339Nano)

	// Convert []string -> []interface{} for Weaviate text[] field
	weaviateTags := make([]interface{}, len(tags))
	for i, v := range tags {
		weaviateTags[i] = v
	}

	props := map[string]any{
		"tenantId":   tenantID,
		"name":       service,
		"definition": purpose,
		"purpose":    purpose,
		"owner":      owner,
		"tags":       weaviateTags,
		"category":   category,
		"sentiment":  sentiment,
		"version":    next,
		"updatedAt":  nowRFC, // keep timestamps consistent as strings
	}

	id := makeID("Service", tenantID, service)
	if err := r.putObject(ctx, "Service", id, props); err != nil {
		return err
	}

	vid := makeID("ServiceVersion", tenantID, service, fmt.Sprintf("%d", next))
	vprops := map[string]any{
		"tenantId": tenantID,
		"name":     service,
		"version":  next,
		"payload": map[string]any{
			"tenantId":   tenantID,
			"name":       service,
			"definition": purpose,
			"purpose":    purpose,
			"owner":      owner,
			"tags":       weaviateTags,
			"category":   category,
			"sentiment":  sentiment,
			"updatedAt":  nowRFC, // match type
		},
		"author":    author,
		"createdAt": nowRFC,
	}
	return r.putObject(ctx, "ServiceVersion", vid, vprops)
}

func (r *WeaviateRepo) GetTraceService(ctx context.Context, tenantID, service string) (*TraceServiceDef, error) {
	// Use inline values instead of variables to avoid GraphQL type issues
	q := fmt.Sprintf(`{ Get { Service(where: {operator: And, operands: [{path: ["tenantId"], operator: Equal, valueString: "%s"}, {path: ["name"], operator: Equal, valueString: "%s"}]}, limit: 1) { purpose owner tags category sentiment updatedAt } } }`, tenantID, service)

	var resp struct {
		Data struct {
			Get struct{ Service []map[string]any }
		}
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, fmt.Errorf("weaviate query failed for service '%s' tenant '%s': %w", service, tenantID, err)
	}

	arr := resp.Data.Get.Service
	if len(arr) == 0 {
		return nil, fmt.Errorf("service '%s' not found in Weaviate for tenant '%s'", service, tenantID)
	}

	it := arr[0]

	// Convert tags from Weaviate format to []string for TraceServiceDef
	var tags []string
	if tagArray, ok := it["tags"].([]interface{}); ok {
		// Convert []interface{} to []string
		tags = make([]string, len(tagArray))
		for i, v := range tagArray {
			if str, ok := v.(string); ok {
				tags[i] = str
			}
		}
	} else if tagArray, ok := it["tags"].([]string); ok {
		// Direct []string assignment
		tags = tagArray
	}
	// If tags is neither format, it remains nil/empty []string

	var updated time.Time
	if s, ok := it["updatedAt"].(string); ok {
		updated, _ = time.Parse(time.RFC3339Nano, s)
	}

	return &TraceServiceDef{
		TenantID:       tenantID,
		Service:        service,
		ServicePurpose: it["purpose"].(string),
		Owner:          it["owner"].(string),
		Tags:           tags, // Now correctly []string
		Category:       it["category"].(string),
		Sentiment:      it["sentiment"].(string),
		UpdatedAt:      updated,
	}, nil
}

func (r *WeaviateRepo) DebugListServices(ctx context.Context, tenantID string) ([]map[string]any, error) {
	q := `query($tenant:String!){ Get { Service(where:{path:["tenantId"],operator:Equal,valueString:$tenant}){ name purpose owner tags updatedAt } } }`
	var resp struct {
		Data struct {
			Get struct{ Service []map[string]any }
		}
	}
	if err := r.gql(ctx, q, map[string]any{"tenant": tenantID}, &resp); err != nil {
		return nil, err
	}
	return resp.Data.Get.Service, nil
}

func (r *WeaviateRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, purpose, owner, category, sentiment string, tags []string, author string) error {
	next, _ := r.maxVersion(ctx, "OperationVersion", map[string]string{"tenantId": tenantID, "service": service, "name": operation})
	nowRFC := time.Now().UTC().Format(time.RFC3339Nano)
	props := map[string]any{"tenantId": tenantID, "service": service, "name": operation, "definition": purpose, "purpose": purpose, "owner": owner, "tags": tags, "category": category, "sentiment": sentiment, "version": next, "updatedAt": nowRFC}
	id := makeID("Operation", tenantID, service, operation)
	if err := r.putObject(ctx, "Operation", id, props); err != nil {
		return err
	}
	vid := makeID("OperationVersion", tenantID, service, operation, fmt.Sprintf("%d", next))
	vprops := map[string]any{"tenantId": tenantID, "service": service, "name": operation, "version": next, "payload": map[string]any{"tenantId": tenantID, "service": service, "name": operation, "definition": purpose, "purpose": purpose, "owner": owner, "tags": tags, "category": category, "sentiment": sentiment, "updatedAt": time.Now()}, "author": author, "createdAt": nowRFC}
	return r.putObject(ctx, "OperationVersion", vid, vprops)
}

func (r *WeaviateRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*TraceOperationDef, error) {
	// Minimal GraphQL string escaping
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}

	q := fmt.Sprintf(`{
	  Get {
	    Operation(
	      where: {
	        operator: And,
	        operands: [
	          { path: ["tenantId"], operator: Equal, valueString: "%s" },
	          { path: ["service"],  operator: Equal, valueString: "%s" },
	          { path: ["name"],     operator: Equal, valueString: "%s" }
	        ]
	      },
	      limit: 1
	    ) {
	      purpose
	      owner
	      tags
	      category
	      sentiment
	      updatedAt
	      _additional { id }
	    }
	  }
	}`, esc(tenantID), esc(service), esc(operation))

	var resp struct {
		Data struct {
			Get struct {
				Operation []map[string]any `json:"Operation"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, fmt.Errorf("weaviate query failed for op %q (service %q tenant %q): %w", operation, service, tenantID, err)
	}

	arr := resp.Data.Get.Operation
	if len(arr) == 0 {
		return nil, fmt.Errorf("operation %q (service %q) not found for tenant %q", operation, service, tenantID)
	}
	it := arr[0]

	// nil-safe string getter
	getStr := func(m map[string]any, key string) string {
		if v, ok := m[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	// tags: accept []any or []string
	var tags []string
	if raw, ok := it["tags"]; ok && raw != nil {
		switch a := raw.(type) {
		case []any:
			for _, v := range a {
				if s, ok := v.(string); ok {
					tags = append(tags, s)
				}
			}
		case []string:
			tags = append(tags, a...)
		}
	}

	// updatedAt: RFC3339Nano then RFC3339
	var updated time.Time
	if s := getStr(it, "updatedAt"); s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			updated = t
		} else if t2, err2 := time.Parse(time.RFC3339, s); err2 == nil {
			updated = t2
		}
	}

	return &TraceOperationDef{
		TenantID:       tenantID,
		Service:        service,
		Operation:      operation,
		ServicePurpose: getStr(it, "purpose"),
		Owner:          getStr(it, "owner"),
		Tags:           tags,
		Category:       getStr(it, "category"),
		Sentiment:      getStr(it, "sentiment"),
		UpdatedAt:      updated,
	}, nil
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
			map[string]any{"name": "category", "dataType": []string{"text"}},
			map[string]any{"name": "sentiment", "dataType": []string{"text"}},
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
			map[string]any{"name": "category", "dataType": []string{"text"}},
			map[string]any{"name": "sentiment", "dataType": []string{"text"}},
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
			map[string]any{"name": "category", "dataType": []string{"text"}},
			map[string]any{"name": "sentiment", "dataType": []string{"text"}},
			map[string]any{"name": "unit", "dataType": []string{"text"}},
			map[string]any{"name": "source", "dataType": []string{"text"}},
			map[string]any{"name": "updatedAt", "dataType": []string{"date"}},
		},
	}
}

// Log field version payload with string dictionary tags
func logFieldVersionPayload() map[string]any {
	return map[string]any{
		"name":     "payload",
		"dataType": []string{"object"},
		"nestedProperties": []any{
			map[string]any{"name": "tenantId", "dataType": []string{"text"}},
			map[string]any{"name": "name", "dataType": []string{"text"}},
			map[string]any{"name": "type", "dataType": []string{"text"}},
			map[string]any{"name": "definition", "dataType": []string{"text"}},
			stringArray("tags"),
			map[string]any{"name": "category", "dataType": []string{"text"}},
			map[string]any{"name": "sentiment", "dataType": []string{"text"}},
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
			boolp("required"), object("allowedValues"), text("category"), text("sentiment"), intp("version"), date("updatedAt"),
		))},
		{"Metric", class("Metric", props(
			text("tenantId"), text("name"), text("definition"), text("owner"), stringArray("tags"), text("category"), text("sentiment"),
			text("unit"), text("source"), intp("version"), date("updatedAt"),
			refp("labels", "Label"),
		))},
		{"LogField", class("LogField", props(
			text("tenantId"), text("name"), text("type"), text("definition"),
			stringArray("tags"), text("category"), text("sentiment"), intp("version"), date("updatedAt"),
		))},
		{"Service", class("Service", props(
			text("tenantId"), text("name"), text("definition"), text("purpose"), text("owner"), stringArray("tags"), text("category"), text("sentiment"), intp("version"), date("updatedAt"), // Updated to stringDict
		))},
		{"Operation", class("Operation", props(
			text("tenantId"), text("service"), text("name"), text("definition"), text("purpose"), text("owner"), stringArray("tags"), text("category"), text("sentiment"), intp("version"), date("updatedAt"), // Updated to stringDict
		))},
		// New API classes for mirador-core v8.0.0
		{"KPIDefinition", class("KPIDefinition", props(
			text("kind"), text("name"), text("unit"), text("format"),
			object("query"), object("thresholds"), stringArray("tags"), object("sparkline"),
			text("ownerUserId"), text("visibility"), date("createdAt"), date("updatedAt"),
		))},
		{"Dashboard", class("Dashboard", props(
			text("name"), text("ownerUserId"), text("visibility"), boolp("isDefault"),
			date("createdAt"), date("updatedAt"),
		))},
		{"KPILayout", class("KPILayout", props(
			refp("kpiDefinition", "KPIDefinition"), refp("dashboard", "Dashboard"),
			intp("x"), intp("y"), intp("w"), intp("h"), date("createdAt"), date("updatedAt"),
		))},
		{"UserPreferences", class("UserPreferences", props(
			refp("currentDashboard", "Dashboard"), text("theme"), boolp("sidebarCollapsed"),
			text("defaultDashboard"), text("timezone"), boolp("keyboardHintSeen"), text("miradorCoreEndpoint"),
			object("preferences"), date("createdAt"), date("updatedAt"),
		))},
		// Version classes with proper payload schemas
		{"MetricVersion", class("MetricVersion", props(text("tenantId"), text("name"), intp("version"), metricVersionPayload(), text("author"), date("createdAt")))},
		{"LabelVersion", class("LabelVersion", props(text("tenantId"), text("name"), intp("version"), labelVersionPayload(), text("author"), date("createdAt")))},
		{"LogFieldVersion", class("LogFieldVersion", props(text("tenantId"), text("name"), intp("version"), logFieldVersionPayload(), text("author"), date("createdAt")))},
		{"ServiceVersion", class("ServiceVersion", props(text("tenantId"), text("name"), intp("version"), serviceVersionPayload(), text("author"), date("createdAt")))},
		{"OperationVersion", class("OperationVersion", props(text("tenantId"), text("service"), text("name"), intp("version"), operationVersionPayload(), text("author"), date("createdAt")))},
		// RBAC classes
		{"RBACTenant", class("RBACTenant", props(
			text("name"), text("displayName"), text("description"),
			object("deployments"), text("status"), text("adminEmail"), text("adminName"),
			map[string]any{
				"name":     "quotas",
				"dataType": []string{"object"},
				"nestedProperties": []any{
					map[string]any{"name": "maxUsers", "dataType": []string{"int"}},
					map[string]any{"name": "maxDashboards", "dataType": []string{"int"}},
					map[string]any{"name": "maxKpis", "dataType": []string{"int"}},
					map[string]any{"name": "storageLimitGb", "dataType": []string{"int"}},
					map[string]any{"name": "apiRateLimit", "dataType": []string{"int"}},
				},
			}, stringArray("features"), object("metadata"), stringArray("tags"), boolp("isSystem"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACUser", class("RBACUser", props(
			text("email"), text("username"), text("fullName"), text("globalRole"), text("passwordHash"),
			boolp("mfaEnabled"), text("mfaSecret"), text("status"), boolp("emailVerified"), text("avatar"),
			text("phone"), text("timezone"), text("language"), date("lastLoginAt"), intp("loginCount"),
			intp("failedLoginCount"), date("lockedUntil"), object("metadata"), stringArray("tags"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACTenantUser", class("RBACTenantUser", props(
			text("tenantId"), text("userId"), text("tenantRole"), text("status"), text("invitedBy"),
			date("invitedAt"), date("acceptedAt"), stringArray("additionalPermissions"), object("metadata"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACMiradorAuth", class("RBACMiradorAuth", props(
			text("userId"), text("username"), text("email"), text("passwordHash"), text("salt"),
			text("totpSecret"), boolp("totpEnabled"), stringArray("backupCodes"), text("tenantId"),
			stringArray("roles"), stringArray("groups"), boolp("isActive"), date("passwordChangedAt"),
			date("passwordExpiresAt"), date("lastLoginAt"), intp("failedLoginCount"), date("lockedUntil"),
			boolp("requirePasswordChange"), object("metadata"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACAuthConfig", class("RBACAuthConfig", props(
			text("tenantId"), text("defaultBackend"), stringArray("enabledBackends"), object("backendConfigs"),
			object("passwordPolicy"), boolp("require2fa"), text("totpIssuer"), intp("sessionTimeoutMinutes"),
			intp("maxConcurrentSessions"), boolp("allowRememberMe"), intp("rememberMeDays"), object("metadata"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACRole", class("RBACRole", props(
			text("tenantId"), text("name"), text("description"), stringArray("permissions"),
			boolp("isSystem"), stringArray("parentRoles"), object("metadata"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACPermission", class("RBACPermission", props(
			text("tenantId"), text("resource"), text("action"), text("scope"), text("description"),
			text("resourcePattern"), object("conditions"), boolp("isSystem"), object("metadata"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACGroup", class("RBACGroup", props(
			text("tenantId"), text("name"), text("description"), stringArray("members"), stringArray("roles"),
			stringArray("parentGroups"), boolp("isSystem"), intp("maxMembers"), boolp("memberSyncEnabled"),
			text("externalId"), object("metadata"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACRoleBinding", class("RBACRoleBinding", props(
			text("subjectType"), text("subjectId"), text("roleId"), text("scope"), text("resourceId"),
			date("expiresAt"), date("notBefore"), intp("precedence"), object("conditions"),
			text("justification"), text("approvedBy"), date("approvedAt"), object("metadata"),
			date("createdAt"), date("updatedAt"), text("createdBy"), text("updatedBy"),
		))},
		{"RBACAuditLog", class("RBACAuditLog", props(
			text("tenantId"), date("timestamp"), text("subjectId"), text("subjectType"), text("action"),
			text("resource"), text("resourceId"), text("result"), object("details"), text("severity"),
			text("source"), text("correlationId"), text("retentionClass"),
		))},
	}

	// Create classes individually to better handle failures
	for _, classDef := range classDefinitions {
		if err := r.ensureClass(ctx, classDef.name, classDef.def); err != nil {
			return fmt.Errorf("failed to create class %s: %w", classDef.name, err)
		}
	}

	return nil
}

// UpsertLabel creates/updates an independent label definition (not metric-scoped)
func (r *WeaviateRepo) UpsertLabel(ctx context.Context, tenantID, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	next, _ := r.maxVersion(ctx, "LabelVersion", map[string]string{"tenantId": tenantID, "name": name})
	nowRFC := time.Now().UTC().Format(time.RFC3339Nano)
	obj := map[string]any{
		"tenantId":      tenantID,
		"name":          name,
		"type":          typ,
		"required":      required,
		"allowedValues": allowed,
		"definition":    description,
		"category":      category,
		"sentiment":     sentiment,
		"version":       next,
		"updatedAt":     nowRFC,
	}
	id := makeID("Label", tenantID, name)
	if err := r.putObject(ctx, "Label", id, obj); err != nil {
		return err
	}
	vid := makeID("LabelVersion", tenantID, name, fmt.Sprintf("%d", next))
	vprops := map[string]any{
		"tenantId": tenantID,
		"name":     name,
		"version":  next,
		"payload": map[string]any{
			"tenantId":      tenantID,
			"name":          name,
			"type":          typ,
			"required":      required,
			"allowedValues": allowed,
			"definition":    description,
			"category":      category,
			"sentiment":     sentiment,
			"updatedAt":     nowRFC,
		},
		"author":    author,
		"createdAt": nowRFC,
	}
	return r.putObject(ctx, "LabelVersion", vid, vprops)
}

func (r *WeaviateRepo) GetLabel(ctx context.Context, tenantID, name string) (*LabelDef, error) {
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\\`, `\\\\`)
		s = strings.ReplaceAll(s, `"`, `\\\"`)
		return s
	}
	q := fmt.Sprintf(`{
      Get { Label(
        where: { operator: And, operands: [
          { path: ["tenantId"], operator: Equal, valueString: "%s" },
          { path: ["name"], operator: Equal, valueString: "%s" }
        ]}, limit: 1) {
          type required allowedValues definition category sentiment updatedAt
        }
      }
    }`, esc(tenantID), esc(name))
	var resp struct {
		Data struct {
			Get struct {
				Label []map[string]any `json:"Label"`
			} `json:"Get"`
		} `json:"data"`
	}
	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data.Get.Label) == 0 {
		return nil, fmt.Errorf("not found")
	}
	it := resp.Data.Get.Label[0]
	allowed := map[string]any{}
	if m, ok := it["allowedValues"].(map[string]any); ok {
		allowed = m
	}
	upd := time.Now()
	if s, ok := it["updatedAt"].(string); ok {
		if t, e := time.Parse(time.RFC3339, s); e == nil {
			upd = t
		}
	}
	tstr, _ := it["type"].(string)
	req, _ := it["required"].(bool)
	def, _ := it["definition"].(string)
	cat, _ := it["category"].(string)
	sent, _ := it["sentiment"].(string)
	return &LabelDef{TenantID: tenantID, Name: name, Type: tstr, Required: req, AllowedVals: allowed, Description: def, Category: cat, Sentiment: sent, UpdatedAt: upd}, nil
}

func (r *WeaviateRepo) ListLabelVersions(ctx context.Context, tenantID, name string) ([]VersionInfo, error) {
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\\`, `\\\\`)
		s = strings.ReplaceAll(s, `"`, `\\\"`)
		return s
	}
	q := fmt.Sprintf(`{ Get { LabelVersion(
      where: { operator: And, operands: [
        { path: ["tenantId"], operator: Equal, valueString: "%s" },
        { path: ["name"], operator: Equal, valueString: "%s" }
      ]}, limit: 1000, sort: [{path:["version"], order: desc}])
      { version author createdAt }
    } }`, esc(tenantID), esc(name))
	var resp struct {
		Data struct {
			Get struct {
				LabelVersion []map[string]any `json:"LabelVersion"`
			} `json:"Get"`
		} `json:"data"`
	}
	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]VersionInfo, 0, len(resp.Data.Get.LabelVersion))
	for _, it := range resp.Data.Get.LabelVersion {
		var t time.Time
		if s, ok := it["createdAt"].(string); ok {
			t, _ = time.Parse(time.RFC3339, s)
		}
		v := int64(0)
		switch x := it["version"].(type) {
		case float64:
			v = int64(x)
		}
		auth, _ := it["author"].(string)
		out = append(out, VersionInfo{Version: v, Author: auth, CreatedAt: t})
	}
	return out, nil
}

func (r *WeaviateRepo) GetLabelVersion(ctx context.Context, tenantID, name string, version int64) (map[string]any, VersionInfo, error) {
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\\`, `\\\\`)
		s = strings.ReplaceAll(s, `"`, `\\\"`)
		return s
	}
	q := fmt.Sprintf(`{ Get { LabelVersion(
      where: { operator: And, operands: [
        { path: ["tenantId"], operator: Equal, valueString: "%s" },
        { path: ["name"], operator: Equal, valueString: "%s" },
        { path: ["version"], operator: Equal, valueInt: %d }
      ]}, limit: 1) { version author createdAt payload } } }`, esc(tenantID), esc(name), version)
	var resp struct {
		Data struct {
			Get struct {
				LabelVersion []map[string]any `json:"LabelVersion"`
			} `json:"Get"`
		} `json:"data"`
	}
	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, VersionInfo{}, err
	}
	if len(resp.Data.Get.LabelVersion) == 0 {
		return nil, VersionInfo{}, fmt.Errorf("not found")
	}
	it := resp.Data.Get.LabelVersion[0]
	payload, _ := it["payload"].(map[string]any)
	var t time.Time
	if s, ok := it["createdAt"].(string); ok {
		t, _ = time.Parse(time.RFC3339, s)
	}
	v := version
	if vv, ok := it["version"].(float64); ok {
		v = int64(vv)
	}
	auth, _ := it["author"].(string)
	return payload, VersionInfo{Version: v, Author: auth, CreatedAt: t}, nil
}

func (r *WeaviateRepo) DeleteLabel(ctx context.Context, tenantID, name string) error {
	id := makeID("Label", tenantID, name)
	start := time.Now()
	err := r.t.DeleteObject(ctx, id)
	monitoring.RecordWeaviateOperation("DeleteObject", "Label", time.Since(start), err == nil)
	return err
}

func (r *WeaviateRepo) DeleteMetric(ctx context.Context, tenantID, metric string) error {
	id := makeID("Metric", tenantID, metric)
	start := time.Now()
	err := r.t.DeleteObject(ctx, id)
	monitoring.RecordWeaviateOperation("DeleteObject", "Metric", time.Since(start), err == nil)
	return err
}

func (r *WeaviateRepo) DeleteLogField(ctx context.Context, tenantID, field string) error {
	id := makeID("LogField", tenantID, field)
	start := time.Now()
	err := r.t.DeleteObject(ctx, id)
	monitoring.RecordWeaviateOperation("DeleteObject", "LogField", time.Since(start), err == nil)
	return err
}

func (r *WeaviateRepo) DeleteTraceService(ctx context.Context, tenantID, service string) error {
	id := makeID("Service", tenantID, service)
	start := time.Now()
	err := r.t.DeleteObject(ctx, id)
	monitoring.RecordWeaviateOperation("DeleteObject", "Service", time.Since(start), err == nil)
	return err
}

func (r *WeaviateRepo) DeleteTraceOperation(ctx context.Context, tenantID, service, operation string) error {
	id := makeID("Operation", tenantID, service, operation)
	start := time.Now()
	err := r.t.DeleteObject(ctx, id)
	monitoring.RecordWeaviateOperation("DeleteObject", "Operation", time.Since(start), err == nil)
	return err
}

// Unified KPI-based schema operations (migrating traditional schema types to KPIs)

// UpsertSchemaAsKPI stores any schema definition as a KPI with type-specific extensions
func (r *WeaviateRepo) UpsertSchemaAsKPI(ctx context.Context, schemaDef *models.SchemaDefinition, author string) error {
	// Convert SchemaDefinition to KPIDefinition format
	kpiDef := &models.KPIDefinition{
		ID:          schemaDef.ID,
		Kind:        "schema", // All schema definitions are of kind "schema"
		Name:        schemaDef.Name,
		Tags:        schemaDef.Tags,
		OwnerUserID: schemaDef.Author,
		Visibility:  "private", // Schema definitions are typically private
		TenantID:    schemaDef.TenantID,
		CreatedAt:   schemaDef.CreatedAt,
		UpdatedAt:   schemaDef.UpdatedAt,
	}

	// Store type-specific data in the query field as JSON
	typeSpecificData := map[string]interface{}{
		"type":       string(schemaDef.Type),
		"category":   schemaDef.Category,
		"sentiment":  schemaDef.Sentiment,
		"extensions": schemaDef.Extensions,
	}
	kpiDef.Query = typeSpecificData

	// For now, delegate to the existing schema-specific methods based on type
	// This maintains backward compatibility while migrating to KPI-based storage
	switch schemaDef.Type {
	case models.SchemaTypeLabel:
		if schemaDef.Extensions.Label == nil {
			return fmt.Errorf("label extension required")
		}
		ext := schemaDef.Extensions.Label
		return r.UpsertLabel(ctx, schemaDef.TenantID, schemaDef.Name, ext.Type, ext.Required, ext.AllowedVals, ext.Description, schemaDef.Category, schemaDef.Sentiment, author)

	case models.SchemaTypeMetric:
		if schemaDef.Extensions.Metric == nil {
			return fmt.Errorf("metric extension required")
		}
		ext := schemaDef.Extensions.Metric
		metricDef := MetricDef{
			TenantID:    schemaDef.TenantID,
			Metric:      schemaDef.Name,
			Description: ext.Description,
			Owner:       ext.Owner,
			Tags:        schemaDef.Tags,
			Category:    schemaDef.Category,
			Sentiment:   schemaDef.Sentiment,
		}
		return r.UpsertMetric(ctx, metricDef, author)

	case models.SchemaTypeLogField:
		if schemaDef.Extensions.LogField == nil {
			return fmt.Errorf("log field extension required")
		}
		ext := schemaDef.Extensions.LogField
		logFieldDef := LogFieldDef{
			TenantID:    schemaDef.TenantID,
			Field:       schemaDef.Name,
			Type:        ext.FieldType,
			Description: ext.Description,
			Tags:        schemaDef.Tags,
			Category:    schemaDef.Category,
			Sentiment:   schemaDef.Sentiment,
		}
		return r.UpsertLogField(ctx, logFieldDef, author)

	case models.SchemaTypeTraceService:
		if schemaDef.Extensions.Trace == nil {
			return fmt.Errorf("trace extension required")
		}
		ext := schemaDef.Extensions.Trace
		return r.UpsertTraceServiceWithAuthor(ctx, schemaDef.TenantID, schemaDef.Name, ext.ServicePurpose, ext.Owner, schemaDef.Category, schemaDef.Sentiment, schemaDef.Tags, author)

	case models.SchemaTypeTraceOperation:
		if schemaDef.Extensions.Trace == nil {
			return fmt.Errorf("trace extension required")
		}
		ext := schemaDef.Extensions.Trace
		if ext.Service == "" {
			return fmt.Errorf("service name required for trace operations")
		}
		return r.UpsertTraceOperationWithAuthor(ctx, schemaDef.TenantID, ext.Service, schemaDef.Name, ext.ServicePurpose, ext.Owner, schemaDef.Category, schemaDef.Sentiment, schemaDef.Tags, author)

	default:
		return fmt.Errorf("unsupported schema type for KPI migration: %s", schemaDef.Type)
	}
}

// GetSchemaAsKPI retrieves any schema definition stored as a KPI
func (r *WeaviateRepo) GetSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) (*models.SchemaDefinition, error) {
	// For now, delegate to existing schema-specific methods
	// This maintains backward compatibility while migrating to KPI-based retrieval
	switch models.SchemaType(schemaType) {
	case models.SchemaTypeLabel:
		labelDef, err := r.GetLabel(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		return &models.SchemaDefinition{
			ID:        labelDef.Name,
			Name:      labelDef.Name,
			Type:      models.SchemaTypeLabel,
			TenantID:  labelDef.TenantID,
			Category:  labelDef.Category,
			Sentiment: labelDef.Sentiment,
			UpdatedAt: labelDef.UpdatedAt,
			Extensions: models.SchemaExtensions{
				Label: &models.LabelExtension{
					Type:        labelDef.Type,
					Required:    labelDef.Required,
					AllowedVals: labelDef.AllowedVals,
					Description: labelDef.Description,
				},
			},
		}, nil

	case models.SchemaTypeMetric:
		metricDef, err := r.GetMetric(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		return &models.SchemaDefinition{
			ID:        metricDef.Metric,
			Name:      metricDef.Metric,
			Type:      models.SchemaTypeMetric,
			TenantID:  metricDef.TenantID,
			Tags:      metricDef.Tags,
			Category:  metricDef.Category,
			Sentiment: metricDef.Sentiment,
			UpdatedAt: metricDef.UpdatedAt,
			Extensions: models.SchemaExtensions{
				Metric: &models.MetricExtension{
					Description: metricDef.Description,
					Owner:       metricDef.Owner,
				},
			},
		}, nil

	case models.SchemaTypeLogField:
		logFieldDef, err := r.GetLogField(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		return &models.SchemaDefinition{
			ID:        logFieldDef.Field,
			Name:      logFieldDef.Field,
			Type:      models.SchemaTypeLogField,
			TenantID:  logFieldDef.TenantID,
			Tags:      logFieldDef.Tags,
			Category:  logFieldDef.Category,
			Sentiment: logFieldDef.Sentiment,
			UpdatedAt: logFieldDef.UpdatedAt,
			Extensions: models.SchemaExtensions{
				LogField: &models.LogFieldExtension{
					FieldType:   logFieldDef.Type,
					Description: logFieldDef.Description,
				},
			},
		}, nil

	case models.SchemaTypeTraceService:
		traceServiceDef, err := r.GetTraceService(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		return &models.SchemaDefinition{
			ID:        traceServiceDef.Service,
			Name:      traceServiceDef.Service,
			Type:      models.SchemaTypeTraceService,
			TenantID:  traceServiceDef.TenantID,
			Tags:      traceServiceDef.Tags,
			Category:  traceServiceDef.Category,
			Sentiment: traceServiceDef.Sentiment,
			UpdatedAt: traceServiceDef.UpdatedAt,
			Extensions: models.SchemaExtensions{
				Trace: &models.TraceExtension{
					ServicePurpose: traceServiceDef.ServicePurpose,
					Owner:          traceServiceDef.Owner,
				},
			},
		}, nil

	case models.SchemaTypeTraceOperation:
		// For trace operations, we need the service name from query params
		// This is a limitation - we may need to store operations differently
		return nil, fmt.Errorf("trace operation retrieval requires service parameter")

	default:
		return nil, fmt.Errorf("unsupported schema type for KPI retrieval: %s", schemaType)
	}
}

// ListSchemasAsKPIs lists schema definitions of a specific type stored as KPIs
func (r *WeaviateRepo) ListSchemasAsKPIs(ctx context.Context, tenantID, schemaType string, limit, offset int) ([]*models.SchemaDefinition, int, error) {
	// For now, return empty results as listing is not implemented for most schema types
	// This would need to be implemented for each schema type
	return []*models.SchemaDefinition{}, 0, nil
}

// DeleteSchemaAsKPI deletes any schema definition stored as a KPI
func (r *WeaviateRepo) DeleteSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) error {
	// Delegate to existing schema-specific delete methods
	switch models.SchemaType(schemaType) {
	case models.SchemaTypeLabel:
		return r.DeleteLabel(ctx, tenantID, id)
	case models.SchemaTypeMetric:
		return r.DeleteMetric(ctx, tenantID, id)
	case models.SchemaTypeLogField:
		return r.DeleteLogField(ctx, tenantID, id)
	case models.SchemaTypeTraceService:
		return r.DeleteTraceService(ctx, tenantID, id)
	case models.SchemaTypeTraceOperation:
		// For trace operations, we need the service name
		// This is a limitation of the current approach
		return fmt.Errorf("trace operation deletion requires service parameter")
	default:
		return fmt.Errorf("unsupported schema type for KPI deletion: %s", schemaType)
	}
}

// Transport returns the underlying Weaviate transport
func (r *WeaviateRepo) Transport() storageweaviate.Transport {
	return r.t
}

// ------------------- KPIRepo Interface Implementation -------------------

func (r *WeaviateRepo) UpsertKPI(ctx context.Context, kpi *models.KPIDefinition) error {
	r.ensureOnce(ctx)
	now := time.Now().UTC()
	kpi.UpdatedAt = now
	if kpi.CreatedAt.IsZero() {
		kpi.CreatedAt = now
	}

	props := map[string]any{
		"kind":        kpi.Kind,
		"name":        kpi.Name,
		"unit":        kpi.Unit,
		"format":      kpi.Format,
		"query":       kpi.Query,
		"thresholds":  kpi.Thresholds,
		"tags":        kpi.Tags,
		"sparkline":   kpi.Sparkline,
		"ownerUserId": kpi.OwnerUserID,
		"visibility":  kpi.Visibility,
		"createdAt":   kpi.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":   kpi.UpdatedAt.Format(time.RFC3339Nano),
	}

	id := makeID("KPIDefinition", kpi.TenantID, kpi.ID)
	return r.putObject(ctx, "KPIDefinition", id, props)
}

func (r *WeaviateRepo) GetKPI(ctx context.Context, tenantID, id string) (*models.KPIDefinition, error) {
	r.ensureOnce(ctx)
	q := fmt.Sprintf(`{
	  Get {
	    KPIDefinition(
	      where: {
	        operator: And,
	        operands: [
	          { path: ["id"], operator: Equal, valueString: "%s" }
	        ]
	      },
	      limit: 1
	    ) {
	      id kind name unit format query thresholds tags sparkline ownerUserId visibility createdAt updatedAt
	    }
	  }
	}`, id)

	var resp struct {
		Data struct {
			Get struct {
				KPIDefinition []map[string]any `json:"KPIDefinition"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, fmt.Errorf("weaviate query failed for KPI '%s': %w", id, err)
	}

	arr := resp.Data.Get.KPIDefinition
	if len(arr) == 0 {
		return nil, fmt.Errorf("KPI '%s' not found", id)
	}

	it := arr[0]

	// Parse timestamps
	var createdAt, updatedAt time.Time
	if s, ok := it["createdAt"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339Nano, s)
	}
	if s, ok := it["updatedAt"].(string); ok {
		updatedAt, _ = time.Parse(time.RFC3339Nano, s)
	}

	// Parse tags
	var tags []string
	if raw, ok := it["tags"].([]interface{}); ok {
		tags = make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	// Parse complex fields with type assertions
	var query map[string]interface{}
	if q, ok := it["query"].(map[string]interface{}); ok {
		query = q
	}

	var thresholds []models.Threshold
	if _, _ = it["thresholds"].([]interface{}); false {
		// ...existing code...
	}

	var sparkline map[string]interface{}
	if s, ok := it["sparkline"].(map[string]interface{}); ok {
		sparkline = s
	}

	return &models.KPIDefinition{
		ID:          getString(it, "id"),
		TenantID:    tenantID,
		Kind:        getString(it, "kind"),
		Name:        getString(it, "name"),
		Unit:        getString(it, "unit"),
		Format:      getString(it, "format"),
		Query:       query,
		Thresholds:  thresholds,
		Tags:        tags,
		Sparkline:   sparkline,
		OwnerUserID: getString(it, "ownerUserId"),
		Visibility:  getString(it, "visibility"),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (r *WeaviateRepo) ListKPIs(ctx context.Context, tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {
	r.ensureOnce(ctx)
	// Build where clause
	whereClause := ""
	if len(tags) > 0 {
		// For simplicity, we'll skip tag filtering for now and implement basic listing
		// Tag filtering would require more complex GraphQL with OR conditions
	}

	q := fmt.Sprintf(`{
	  Get {
	    KPIDefinition(
	      %s
	      limit: %d
	      offset: %d
	    ) {
	      id kind name unit format query thresholds tags sparkline ownerUserId visibility createdAt updatedAt
	    }
	  }
	}`, whereClause, limit, offset)

	var resp struct {
		Data struct {
			Get struct {
				KPIDefinition []map[string]any `json:"KPIDefinition"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, 0, err
	}

	var kpis []*models.KPIDefinition
	for _, it := range resp.Data.Get.KPIDefinition {
		// Parse timestamps
		var createdAt, updatedAt time.Time
		if s, ok := it["createdAt"].(string); ok {
			createdAt, _ = time.Parse(time.RFC3339Nano, s)
		}
		if s, ok := it["updatedAt"].(string); ok {
			updatedAt, _ = time.Parse(time.RFC3339Nano, s)
		}

		// Parse tags
		var kpiTags []string
		if raw, ok := it["tags"].([]interface{}); ok {
			kpiTags = make([]string, 0, len(raw))
			for _, v := range raw {
				if s, ok := v.(string); ok {
					kpiTags = append(kpiTags, s)
				}
			}
		}

		// Parse complex fields with type assertions
		var query map[string]interface{}
		if q, ok := it["query"].(map[string]interface{}); ok {
			query = q
		}

		var thresholds []models.Threshold
		if _, _ = it["thresholds"].([]interface{}); false {
			// ...existing code...
		}

		var sparkline map[string]interface{}
		if s, ok := it["sparkline"].(map[string]interface{}); ok {
			sparkline = s
		}

		kpis = append(kpis, &models.KPIDefinition{
			ID:          getString(it, "id"),
			TenantID:    tenantID,
			Kind:        getString(it, "kind"),
			Name:        getString(it, "name"),
			Unit:        getString(it, "unit"),
			Format:      getString(it, "format"),
			Query:       query,
			Thresholds:  thresholds,
			Tags:        kpiTags,
			Sparkline:   sparkline,
			OwnerUserID: getString(it, "ownerUserId"),
			Visibility:  getString(it, "visibility"),
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	// For total count, we'd need a separate aggregation query
	// For now, return the length of results as approximate total
	total := len(kpis)
	if limit > 0 && len(kpis) == limit {
		// If we got a full page, there might be more
		total += offset
	}

	return kpis, total, nil
}

func (r *WeaviateRepo) DeleteKPI(ctx context.Context, tenantID, id string) error {
	r.ensureOnce(ctx)
	objID := makeID("KPIDefinition", tenantID, id)
	start := time.Now()
	err := r.t.DeleteObject(ctx, objID)
	monitoring.RecordWeaviateOperation("DeleteObject", "KPIDefinition", time.Since(start), err == nil)
	return err
}

func (r *WeaviateRepo) GetKPILayoutsForDashboard(ctx context.Context, tenantID, dashboardID string) (map[string]interface{}, error) {
	r.ensureOnce(ctx)
	q := fmt.Sprintf(`{
	  Get {
	    KPILayout(
	      where: {
	        path: ["dashboard"],
	        operator: Equal,
	        valueString: "%s"
	      }
	    ) {
	      id kpiDefinition { id } x y w h
	    }
	  }
	}`, makeID("Dashboard", tenantID, dashboardID))

	var resp struct {
		Data struct {
			Get struct {
				KPILayout []map[string]any `json:"KPILayout"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, err
	}

	layouts := make(map[string]interface{})
	for _, it := range resp.Data.Get.KPILayout {
		kpiID := ""
		if kpiRef, ok := it["kpiDefinition"].(map[string]any); ok {
			if id, ok := kpiRef["id"].(string); ok {
				kpiID = id
			}
		}

		if kpiID != "" {
			layouts[kpiID] = map[string]interface{}{
				"x": getInt(it, "x"),
				"y": getInt(it, "y"),
				"w": getInt(it, "w"),
				"h": getInt(it, "h"),
			}
		}
	}

	return layouts, nil
}

func (r *WeaviateRepo) BatchUpsertKPILayouts(ctx context.Context, tenantID, dashboardID string, layouts map[string]interface{}) error {
	r.ensureOnce(ctx)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	dashboardRefID := makeID("Dashboard", tenantID, dashboardID)

	for kpiID, layoutData := range layouts {
		layout, ok := layoutData.(map[string]interface{})
		if !ok {
			continue
		}

		kpiRefID := makeID("KPIDefinition", tenantID, kpiID)
		props := map[string]any{
			"kpiDefinition": map[string]any{"id": kpiRefID},
			"dashboard":     map[string]any{"id": dashboardRefID},
			"x":             getIntFromLayout(layout, "x"),
			"y":             getIntFromLayout(layout, "y"),
			"w":             getIntFromLayout(layout, "w"),
			"h":             getIntFromLayout(layout, "h"),
			"createdAt":     now,
			"updatedAt":     now,
		}

		id := makeID("KPILayout", tenantID, dashboardID, kpiID)
		if err := r.putObject(ctx, "KPILayout", id, props); err != nil {
			return err
		}
	}

	return nil
}

func (r *WeaviateRepo) UpsertDashboard(ctx context.Context, dashboard *models.Dashboard) error {
	r.ensureOnce(ctx)
	now := time.Now().UTC()
	dashboard.UpdatedAt = now
	if dashboard.CreatedAt.IsZero() {
		dashboard.CreatedAt = now
	}

	props := map[string]any{
		"name":        dashboard.Name,
		"ownerUserId": dashboard.OwnerUserID,
		"visibility":  dashboard.Visibility,
		"isDefault":   dashboard.IsDefault,
		"createdAt":   dashboard.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":   dashboard.UpdatedAt.Format(time.RFC3339Nano),
	}

	id := makeID("Dashboard", dashboard.TenantID, dashboard.ID)
	return r.putObject(ctx, "Dashboard", id, props)
}

func (r *WeaviateRepo) GetDashboard(ctx context.Context, tenantID, id string) (*models.Dashboard, error) {
	r.ensureOnce(ctx)
	q := fmt.Sprintf(`{
	  Get {
	    Dashboard(
	      where: {
	        operator: And,
	        operands: [
	          { path: ["id"], operator: Equal, valueString: "%s" }
	        ]
	      },
	      limit: 1
	    ) {
	      id name ownerUserId visibility isDefault createdAt updatedAt
	    }
	  }
	}`, id)

	var resp struct {
		Data struct {
			Get struct {
				Dashboard []map[string]any `json:"Dashboard"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, fmt.Errorf("weaviate query failed for dashboard '%s': %w", id, err)
	}

	arr := resp.Data.Get.Dashboard
	if len(arr) == 0 {
		return nil, fmt.Errorf("dashboard '%s' not found", id)
	}

	it := arr[0]

	// Parse timestamps
	var createdAt, updatedAt time.Time
	if s, ok := it["createdAt"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339Nano, s)
	}
	if s, ok := it["updatedAt"].(string); ok {
		updatedAt, _ = time.Parse(time.RFC3339Nano, s)
	}

	return &models.Dashboard{
		ID:          getString(it, "id"),
		TenantID:    tenantID,
		Name:        getString(it, "name"),
		OwnerUserID: getString(it, "ownerUserId"),
		Visibility:  getString(it, "visibility"),
		IsDefault:   getBool(it, "isDefault"),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (r *WeaviateRepo) ListDashboards(ctx context.Context, tenantID string, limit, offset int) ([]*models.Dashboard, int, error) {
	r.ensureOnce(ctx)
	q := fmt.Sprintf(`{
	  Get {
	    Dashboard(
	      limit: %d
	      offset: %d
	    ) {
	      id name ownerUserId visibility isDefault createdAt updatedAt
	    }
	  }
	}`, limit, offset)

	var resp struct {
		Data struct {
			Get struct {
				Dashboard []map[string]any `json:"Dashboard"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.gql(ctx, q, nil, &resp); err != nil {
		return nil, 0, err
	}

	var dashboards []*models.Dashboard
	for _, it := range resp.Data.Get.Dashboard {
		// Parse timestamps
		var createdAt, updatedAt time.Time
		if s, ok := it["createdAt"].(string); ok {
			createdAt, _ = time.Parse(time.RFC3339Nano, s)
		}
		if s, ok := it["updatedAt"].(string); ok {
			updatedAt, _ = time.Parse(time.RFC3339Nano, s)
		}

		dashboards = append(dashboards, &models.Dashboard{
			ID:          getString(it, "id"),
			TenantID:    tenantID,
			Name:        getString(it, "name"),
			OwnerUserID: getString(it, "ownerUserId"),
			Visibility:  getString(it, "visibility"),
			IsDefault:   getBool(it, "isDefault"),
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	// For total count, return approximate
	total := len(dashboards)
	if limit > 0 && len(dashboards) == limit {
		total += offset
	}

	return dashboards, total, nil
}

func (r *WeaviateRepo) DeleteDashboard(ctx context.Context, tenantID, id string) error {
	r.ensureOnce(ctx)
	objID := makeID("Dashboard", tenantID, id)
	start := time.Now()
	err := r.t.DeleteObject(ctx, objID)
	monitoring.RecordWeaviateOperation("DeleteObject", "Dashboard", time.Since(start), err == nil)
	return err
}

// Helper functions for type conversion
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
}

func getIntFromLayout(layout map[string]interface{}, key string) int {
	if v, ok := layout[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
}
