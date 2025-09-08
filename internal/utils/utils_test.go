package utils

import "testing"

func TestContainsAndUint32(t *testing.T) {
    if !Contains([]string{"a","b"}, "a") { t.Fatalf("contains failed") }
    if IsUint32String("-1") || IsUint32String("abc") { t.Fatalf("invalid uint32 accepted") }
}

func TestQueryValidator_Dangerous(t *testing.T) {
    v := NewQueryValidator()
    if err := v.ValidateLogsQL("drop database foo"); err == nil { t.Fatalf("expected SQL detection") }
}
