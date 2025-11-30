package handlers

import (
	"os"
	"testing"
)

// Sanity check for path resolution used by GetOpenAPISpec. This helps reproduce
// path lookup in CI where working directories may differ.
func TestResolveOpenAPIPath(t *testing.T) {
	p := resolveOpenAPIPath()
	t.Logf("resolveOpenAPIPath -> %s", p)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("resolved path does not exist: %s error=%v", p, err)
	}
}
