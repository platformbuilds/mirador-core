package repo

import "testing"

func TestNewSchemaRepo_Construct(t *testing.T) {
    r := NewSchemaRepo(nil)
    if r == nil { t.Fatalf("nil repo") }
}

