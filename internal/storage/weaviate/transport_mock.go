package weaviate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MockTransport implements Transport interface for testing without Weaviate
type MockTransport struct {
	mu      sync.RWMutex
	classes map[string]map[string]any            // class name -> class definition
	objects map[string]map[string]map[string]any // class -> id -> properties
}

// NewMockTransport creates a new mock transport for testing
func NewMockTransport() *MockTransport {
	return &MockTransport{
		classes: make(map[string]map[string]any),
		objects: make(map[string]map[string]map[string]any),
	}
}

// Ready always returns nil for mock
func (m *MockTransport) Ready(ctx context.Context) error {
	return nil
}

// EnsureClasses stores class definitions in memory
func (m *MockTransport) EnsureClasses(ctx context.Context, classDefs []map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, classDef := range classDefs {
		className, ok := classDef["class"].(string)
		if !ok {
			return fmt.Errorf("class definition missing 'class' field")
		}
		m.classes[className] = classDef
		if m.objects[className] == nil {
			m.objects[className] = make(map[string]map[string]any)
		}
	}
	return nil
}

// PutObject stores an object in memory
func (m *MockTransport) PutObject(ctx context.Context, class, id string, props map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.objects[class] == nil {
		m.objects[class] = make(map[string]map[string]any)
	}
	m.objects[class][id] = props
	return nil
}

// GraphQL implements basic GraphQL query execution for testing
func (m *MockTransport) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple query parsing for common patterns used in the codebase
	query = strings.TrimSpace(query)

	// Handle Get queries
	if strings.HasPrefix(query, "{") && strings.Contains(query, "Get {") {
		return m.handleGetQuery(query, variables, out)
	}

	// For now, return empty result for unsupported queries
	result := map[string]any{"data": map[string]any{"Get": map[string]any{}}}
	return json.Unmarshal([]byte(fmt.Sprintf(`%v`, result)), out)
}

// handleGetQuery handles basic Get { Class(...) { fields } } queries
func (m *MockTransport) handleGetQuery(query string, _variables map[string]any, out any) error {
	// Very basic parsing - extract class name and fields
	// This is a simplified implementation for testing

	// Find the class name after "Get {"
	start := strings.Index(query, "Get {")
	if start == -1 {
		return fmt.Errorf("invalid Get query")
	}
	start += 5 // "Get {"

	// Find the class name
	classStart := strings.Index(query[start:], " ")
	if classStart == -1 {
		return fmt.Errorf("no class found in query")
	}
	classStart += start
	classEnd := strings.Index(query[classStart:], "(")
	if classEnd == -1 {
		return fmt.Errorf("no class query found")
	}
	classEnd += classStart
	className := strings.TrimSpace(query[classStart:classEnd])

	// Get objects for this class
	objects := m.objects[className]
	if objects == nil {
		objects = make(map[string]map[string]any)
	}

	// Extract fields from query (simplified)
	fieldStart := strings.Index(query, "{")
	fieldEnd := strings.LastIndex(query, "}")
	if fieldStart == -1 || fieldEnd == -1 || fieldEnd <= fieldStart {
		return fmt.Errorf("invalid field selection")
	}
	fieldStr := query[fieldStart+1 : fieldEnd]
	fields := strings.Fields(fieldStr)
	// Remove common GraphQL fields
	filteredFields := []string{}
	for _, field := range fields {
		field = strings.Trim(field, " \t\n,")
		if field != "" && field != "_additional" && field != "id" {
			filteredFields = append(filteredFields, field)
		}
	}

	// Apply where clause if present (very basic)
	results := []map[string]any{}
	for id, props := range objects {
		obj := map[string]any{"_additional": map[string]any{"id": id}}
		for _, field := range filteredFields {
			if val, exists := props[field]; exists {
				obj[field] = val
			}
		}
		results = append(results, obj)
	}

	// Sort by id for consistent results
	sort.Slice(results, func(i, j int) bool {
		id1 := results[i]["_additional"].(map[string]any)["id"].(string)
		id2 := results[j]["_additional"].(map[string]any)["id"].(string)
		return id1 < id2
	})

	// Create response
	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				className: results,
			},
		},
	}

	// Convert to JSON and back to match interface
	jsonData, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, out)
}

// GetSchema returns the stored class definitions
func (m *MockTransport) GetSchema(ctx context.Context, out any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	classes := []map[string]any{}
	for _, classDef := range m.classes {
		classes = append(classes, classDef)
	}

	schema := map[string]any{
		"classes": classes,
	}

	jsonData, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, out)
}

// DeleteObject removes an object from memory
func (m *MockTransport) DeleteObject(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and delete the object by id across all classes
	for _, classObjects := range m.objects {
		if _, exists := classObjects[id]; exists {
			delete(classObjects, id)
			return nil
		}
	}
	return fmt.Errorf("object with id %s not found", id)
}

// Clear resets the mock transport state
func (m *MockTransport) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.classes = make(map[string]map[string]any)
	m.objects = make(map[string]map[string]map[string]any)
}

// GetObjects returns all objects for a class (helper for testing)
func (m *MockTransport) GetObjects(class string) map[string]map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if objects, exists := m.objects[class]; exists {
		// Return a copy to avoid external modifications
		copiedObjects := make(map[string]map[string]any)
		for id, props := range objects {
			copiedObjects[id] = props
		}
		return copiedObjects
	}
	return make(map[string]map[string]any)
}
